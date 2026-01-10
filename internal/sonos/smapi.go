package sonos

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	smapiNamespace  = "http://www.sonos.com/Services/1.1"
	smapiSOAPAction = "http://www.sonos.com/Services/1.1#"
)

type SMAPIClient struct {
	httpClient *http.Client

	Service     MusicServiceDescriptor
	HouseholdID string
	DeviceID    string
	TokenStore  SMAPITokenStore

	searchPrefixMap map[string]string
}

func NewSMAPIClient(ctx context.Context, speaker *Client, svc MusicServiceDescriptor, store SMAPITokenStore) (*SMAPIClient, error) {
	if speaker == nil {
		return nil, errors.New("speaker client is required")
	}
	if strings.TrimSpace(svc.SecureURI) == "" {
		return nil, errors.New("service SecureUri is required")
	}
	if store == nil {
		return nil, errors.New("token store is required")
	}

	hh, err := speaker.GetHouseholdID(ctx)
	if err != nil {
		return nil, err
	}

	deviceID, _ := speaker.GetString(ctx, "R_TrialZPSerial")
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		dd, derr := speaker.GetDeviceDescription(ctx)
		if derr != nil {
			return nil, derr
		}
		deviceID = strings.TrimSpace(dd.UDN)
	}
	if deviceID == "" {
		return nil, errors.New("could not determine device id")
	}

	return &SMAPIClient{
		httpClient:      speaker.HTTP,
		Service:         svc,
		HouseholdID:     hh,
		DeviceID:        deviceID,
		TokenStore:      store,
		searchPrefixMap: nil,
	}, nil
}

type SMAPIBeginAuthResult struct {
	RegURL       string `json:"regUrl"`
	LinkCode     string `json:"linkCode"`
	LinkDeviceID string `json:"linkDeviceId,omitempty"`
}

func (c *SMAPIClient) BeginAuthentication(ctx context.Context) (SMAPIBeginAuthResult, error) {
	switch c.Service.Auth {
	case MusicServiceAuthDeviceLink:
		var out struct {
			Result struct {
				RegURL       string `xml:"regUrl"`
				LinkCode     string `xml:"linkCode"`
				LinkDeviceID string `xml:"linkDeviceId"`
			} `xml:"getDeviceLinkCodeResult"`
		}
		if err := c.smapiCallInto(ctx, "getDeviceLinkCode", map[string]string{
			"householdId": c.HouseholdID,
		}, &out, smapiCallOptions{AllowUnauthed: true}); err != nil {
			return SMAPIBeginAuthResult{}, err
		}
		return SMAPIBeginAuthResult{
			RegURL:       strings.TrimSpace(out.Result.RegURL),
			LinkCode:     strings.TrimSpace(out.Result.LinkCode),
			LinkDeviceID: strings.TrimSpace(out.Result.LinkDeviceID),
		}, nil
	case MusicServiceAuthAppLink:
		// AppLink returns authorizeAccount/deviceLink inside getAppLinkResult.
		// Apple Music and other AppLink services require additional parameters.
		var out struct {
			Result struct {
				AuthorizeAccount struct {
					AppURL     string `xml:"appUrl"`
					DeviceLink struct {
						RegURL       string `xml:"regUrl"`
						LinkCode     string `xml:"linkCode"`
						LinkDeviceID string `xml:"linkDeviceId"`
						ShowLinkCode bool   `xml:"showLinkCode"`
					} `xml:"deviceLink"`
				} `xml:"authorizeAccount"`
			} `xml:"getAppLinkResult"`
		}
		if err := c.smapiCallInto(ctx, "getAppLink", map[string]string{
			"householdId":  c.HouseholdID,
			"hardware":     "CLI",
			"osVersion":    "1.0",
			"sonosAppName": "sonoscli",
			"callbackPath": "", // CLI doesn't use callback URLs
		}, &out, smapiCallOptions{AllowUnauthed: true}); err != nil {
			return SMAPIBeginAuthResult{}, err
		}
		return SMAPIBeginAuthResult{
			RegURL:       strings.TrimSpace(out.Result.AuthorizeAccount.DeviceLink.RegURL),
			LinkCode:     strings.TrimSpace(out.Result.AuthorizeAccount.DeviceLink.LinkCode),
			LinkDeviceID: strings.TrimSpace(out.Result.AuthorizeAccount.DeviceLink.LinkDeviceID),
		}, nil
	default:
		return SMAPIBeginAuthResult{}, fmt.Errorf("service auth type %q does not support begin auth", c.Service.Auth)
	}
}

func (c *SMAPIClient) CompleteAuthentication(ctx context.Context, linkCode, linkDeviceID string) (SMAPITokenPair, error) {
	linkCode = strings.TrimSpace(linkCode)
	linkDeviceID = strings.TrimSpace(linkDeviceID)
	if linkCode == "" {
		return SMAPITokenPair{}, errors.New("linkCode is required")
	}
	if linkDeviceID == "" {
		linkDeviceID = c.DeviceID
	}

	var out struct {
		Result struct {
			AuthToken  string `xml:"authToken"`
			PrivateKey string `xml:"privateKey"`
		} `xml:"getDeviceAuthTokenResult"`
	}
	if err := c.smapiCallInto(ctx, "getDeviceAuthToken", map[string]string{
		"householdId":  c.HouseholdID,
		"linkCode":     linkCode,
		"linkDeviceId": linkDeviceID,
	}, &out, smapiCallOptions{AllowUnauthed: true}); err != nil {
		return SMAPITokenPair{}, err
	}

	pair := SMAPITokenPair{
		AuthToken:   strings.TrimSpace(out.Result.AuthToken),
		PrivateKey:  strings.TrimSpace(out.Result.PrivateKey),
		UpdatedAt:   time.Now().UTC(),
		LinkCode:    linkCode,
		DeviceID:    c.DeviceID,
		HouseholdID: c.HouseholdID,
	}
	if pair.AuthToken == "" || pair.PrivateKey == "" {
		return SMAPITokenPair{}, errors.New("empty token pair in response")
	}
	if err := c.TokenStore.Save(c.Service.ID, c.HouseholdID, pair); err != nil {
		return SMAPITokenPair{}, err
	}
	return pair, nil
}

type SMAPIItem struct {
	ID       string `json:"id"`
	ItemType string `json:"itemType"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type SMAPISearchResult struct {
	Category        string      `json:"category"`
	Term            string      `json:"term"`
	Index           int         `json:"index"`
	Count           int         `json:"count"`
	Total           int         `json:"total"`
	MediaMetadata   []SMAPIItem `json:"mediaMetadata,omitempty"`
	MediaCollection []SMAPIItem `json:"mediaCollection,omitempty"`
}

func (c *SMAPIClient) Search(ctx context.Context, category, term string, index, count int) (SMAPISearchResult, error) {
	category = strings.TrimSpace(category)
	term = strings.TrimSpace(term)
	if category == "" {
		return SMAPISearchResult{}, errors.New("category is required")
	}
	if count <= 0 {
		count = 10
	}
	if index < 0 {
		index = 0
	}

	pmap, err := c.searchCategories(ctx)
	if err != nil {
		return SMAPISearchResult{}, err
	}
	searchID, ok := pmap[category]
	if !ok || strings.TrimSpace(searchID) == "" {
		return SMAPISearchResult{}, fmt.Errorf("service %q does not support search category %q", c.Service.Name, category)
	}

	type mediaMetadata struct {
		ID       string `xml:"id"`
		ItemType string `xml:"itemType"`
		Title    string `xml:"title"`
		MimeType string `xml:"mimeType"`
		Summary  string `xml:"summary"`
	}
	type mediaCollection struct {
		ID       string `xml:"id"`
		ItemType string `xml:"itemType"`
		Title    string `xml:"title"`
		Summary  string `xml:"summary"`
	}
	var out struct {
		Result struct {
			Index int               `xml:"index"`
			Count int               `xml:"count"`
			Total int               `xml:"total"`
			MD    []mediaMetadata   `xml:"mediaMetadata"`
			MC    []mediaCollection `xml:"mediaCollection"`
		} `xml:"searchResult"`
	}

	if err := c.smapiCallInto(ctx, "search", map[string]string{
		"id":    searchID,
		"term":  term,
		"index": fmt.Sprintf("%d", index),
		"count": fmt.Sprintf("%d", count),
	}, &out, smapiCallOptions{}); err != nil {
		return SMAPISearchResult{}, err
	}

	res := SMAPISearchResult{
		Category: category,
		Term:     term,
		Index:    out.Result.Index,
		Count:    out.Result.Count,
		Total:    out.Result.Total,
	}
	for _, md := range out.Result.MD {
		res.MediaMetadata = append(res.MediaMetadata, SMAPIItem{
			ID:       strings.TrimSpace(md.ID),
			ItemType: strings.TrimSpace(md.ItemType),
			Title:    strings.TrimSpace(md.Title),
			Summary:  strings.TrimSpace(md.Summary),
			MimeType: strings.TrimSpace(md.MimeType),
		})
	}
	for _, mc := range out.Result.MC {
		res.MediaCollection = append(res.MediaCollection, SMAPIItem{
			ID:       strings.TrimSpace(mc.ID),
			ItemType: strings.TrimSpace(mc.ItemType),
			Title:    strings.TrimSpace(mc.Title),
			Summary:  strings.TrimSpace(mc.Summary),
		})
	}
	return res, nil
}

func (c *SMAPIClient) SearchCategories(ctx context.Context) ([]string, error) {
	m, err := c.searchCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

type SMAPIBrowseResult struct {
	ID              string      `json:"id"`
	Index           int         `json:"index"`
	Count           int         `json:"count"`
	Total           int         `json:"total"`
	MediaMetadata   []SMAPIItem `json:"mediaMetadata,omitempty"`
	MediaCollection []SMAPIItem `json:"mediaCollection,omitempty"`
}

func (c *SMAPIClient) GetMetadata(ctx context.Context, id string, index, count int, recursive bool) (SMAPIBrowseResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = "root"
	}
	if count <= 0 {
		count = 50
	}
	if index < 0 {
		index = 0
	}

	type mediaMetadata struct {
		ID       string `xml:"id"`
		ItemType string `xml:"itemType"`
		Title    string `xml:"title"`
		MimeType string `xml:"mimeType"`
		Summary  string `xml:"summary"`
	}
	type mediaCollection struct {
		ID       string `xml:"id"`
		ItemType string `xml:"itemType"`
		Title    string `xml:"title"`
		Summary  string `xml:"summary"`
	}
	var out struct {
		Result struct {
			Index int               `xml:"index"`
			Count int               `xml:"count"`
			Total int               `xml:"total"`
			MD    []mediaMetadata   `xml:"mediaMetadata"`
			MC    []mediaCollection `xml:"mediaCollection"`
		} `xml:"getMetadataResult"`
	}

	if err := c.smapiCallInto(ctx, "getMetadata", map[string]string{
		"id":        id,
		"index":     fmt.Sprintf("%d", index),
		"count":     fmt.Sprintf("%d", count),
		"recursive": fmt.Sprintf("%d", boolToInt(recursive)),
	}, &out, smapiCallOptions{}); err != nil {
		return SMAPIBrowseResult{}, err
	}

	res := SMAPIBrowseResult{
		ID:    id,
		Index: out.Result.Index,
		Count: out.Result.Count,
		Total: out.Result.Total,
	}
	for _, md := range out.Result.MD {
		res.MediaMetadata = append(res.MediaMetadata, SMAPIItem{
			ID:       strings.TrimSpace(md.ID),
			ItemType: strings.TrimSpace(md.ItemType),
			Title:    strings.TrimSpace(md.Title),
			Summary:  strings.TrimSpace(md.Summary),
			MimeType: strings.TrimSpace(md.MimeType),
		})
	}
	for _, mc := range out.Result.MC {
		res.MediaCollection = append(res.MediaCollection, SMAPIItem{
			ID:       strings.TrimSpace(mc.ID),
			ItemType: strings.TrimSpace(mc.ItemType),
			Title:    strings.TrimSpace(mc.Title),
			Summary:  strings.TrimSpace(mc.Summary),
		})
	}
	return res, nil
}

func (c *SMAPIClient) searchCategories(ctx context.Context) (map[string]string, error) {
	if c.searchPrefixMap != nil {
		return c.searchPrefixMap, nil
	}
	// TuneIn is special-cased: no presentation map, but supports searches.
	if strings.EqualFold(c.Service.Name, "TuneIn") {
		c.searchPrefixMap = map[string]string{
			"stations": "search:station",
			"shows":    "search:show",
			"hosts":    "search:host",
		}
		return c.searchPrefixMap, nil
	}

	pmapURI := strings.TrimSpace(c.Service.PresentationMapURI)
	if pmapURI == "" && strings.TrimSpace(c.Service.ManifestURI) != "" {
		u, err := fetchPresentationMapURIFromManifest(ctx, c.httpClient, c.Service.ManifestURI)
		if err == nil {
			pmapURI = u
		}
	}
	if pmapURI == "" {
		c.searchPrefixMap = map[string]string{}
		return c.searchPrefixMap, nil
	}

	m, err := fetchAndParsePresentationMap(ctx, c.httpClient, pmapURI)
	if err != nil {
		return nil, err
	}
	c.searchPrefixMap = m
	return c.searchPrefixMap, nil
}

func fetchPresentationMapURIFromManifest(ctx context.Context, httpClient *http.Client, manifestURI string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURI, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("manifest: http %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	var payload struct {
		PresentationMap *struct {
			URI string `json:"uri"`
		} `json:"presentationMap"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	if payload.PresentationMap == nil {
		return "", errors.New("manifest missing presentationMap")
	}
	return strings.TrimSpace(payload.PresentationMap.URI), nil
}

type smapiCallOptions struct {
	AllowUnauthed bool
}

func (c *SMAPIClient) smapiCallInto(ctx context.Context, method string, args map[string]string, out any, opts smapiCallOptions) error {
	raw, err := c.smapiCall(ctx, method, args, opts)
	if err != nil {
		return err
	}
	return xml.Unmarshal(raw, out)
}

func (c *SMAPIClient) smapiCall(ctx context.Context, method string, args map[string]string, opts smapiCallOptions) ([]byte, error) {
	endpointURL := strings.TrimSpace(c.Service.SecureURI)
	if endpointURL == "" {
		return nil, errors.New("missing service SecureUri")
	}

	headerXML, err := c.buildCredentialsHeader(opts.AllowUnauthed)
	if err != nil {
		return nil, err
	}
	body := buildSMAPISOAPEnvelope(method, args, headerXML)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPACTION", fmt.Sprintf("%q", smapiSOAPAction+method))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 200 {
		slog.Debug("smapi: response", "method", method, "status", resp.StatusCode, "body", string(raw))
		return extractSOAPBodyFirstChild(raw)
	}
	if resp.StatusCode == 500 {
		fault, ok := parseSOAPFault(raw)
		if !ok {
			return nil, fmt.Errorf("smapi soap http %s", resp.Status)
		}

		// Token refresh flow mirrors SoCo: faultcode contains 'Client.TokenRefreshRequired'
		if strings.Contains(fault.FaultCode, "Client.TokenRefreshRequired") {
			token, key := extractAuthTokenPair(raw)
			if token == "" || key == "" {
				return nil, fmt.Errorf("token refresh required but no new token found: %s", fault.FaultCode)
			}
			_ = c.TokenStore.Save(c.Service.ID, c.HouseholdID, SMAPITokenPair{
				AuthToken:   token,
				PrivateKey:  key,
				UpdatedAt:   time.Now().UTC(),
				DeviceID:    c.DeviceID,
				HouseholdID: c.HouseholdID,
			})
			// Retry once with new tokens.
			headerXML, _ = c.buildCredentialsHeader(false)
			body = buildSMAPISOAPEnvelope(method, args, headerXML)
			req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
			req2.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
			req2.Header.Set("SOAPACTION", fmt.Sprintf("%q", smapiSOAPAction+method))
			resp2, err := c.httpClient.Do(req2)
			if err != nil {
				return nil, err
			}
			defer resp2.Body.Close()
			raw2, err := io.ReadAll(io.LimitReader(resp2.Body, 4<<20))
			if err != nil {
				return nil, err
			}
			if resp2.StatusCode == 200 {
				return extractSOAPBodyFirstChild(raw2)
			}
			if resp2.StatusCode == 500 {
				if fault2, ok := parseSOAPFault(raw2); ok {
					return nil, fmt.Errorf("smapi fault after refresh: %s: %s", fault2.FaultCode, fault2.FaultString)
				}
			}
			return nil, fmt.Errorf("smapi soap http %s", resp2.Status)
		}

		return nil, fmt.Errorf("smapi fault: %s: %s", fault.FaultCode, fault.FaultString)
	}
	return nil, fmt.Errorf("smapi soap http %s", resp.Status)
}

func (c *SMAPIClient) buildCredentialsHeader(allowUnauthed bool) (string, error) {
	creds := strings.Builder{}
	creds.WriteString(`<credentials xmlns="`)
	creds.WriteString(xmlEscapeAttr(smapiNamespace))
	creds.WriteString(`">`)
	creds.WriteString(`<deviceId>`)
	creds.WriteString(xmlEscapeText(c.DeviceID))
	creds.WriteString(`</deviceId>`)
	creds.WriteString(`<deviceProvider>Sonos</deviceProvider>`)

	switch c.Service.Auth {
	case MusicServiceAuthDeviceLink, MusicServiceAuthAppLink:
		creds.WriteString(`<context></context>`)
		if !allowUnauthed {
			pair, ok, err := c.TokenStore.Load(c.Service.ID, c.HouseholdID)
			if err != nil {
				return "", err
			}
			if !ok {
				return "", errors.New("service not authenticated: run `sonos auth smapi begin`/`complete`")
			}
			creds.WriteString(`<loginToken>`)
			creds.WriteString(`<token>`)
			creds.WriteString(xmlEscapeText(pair.AuthToken))
			creds.WriteString(`</token>`)
			creds.WriteString(`<key>`)
			creds.WriteString(xmlEscapeText(pair.PrivateKey))
			creds.WriteString(`</key>`)
			creds.WriteString(`<householdId>`)
			creds.WriteString(xmlEscapeText(c.HouseholdID))
			creds.WriteString(`</householdId>`)
			creds.WriteString(`</loginToken>`)
		}
	case MusicServiceAuthAnonymous, MusicServiceAuthUserID:
		// No login token required.
	default:
		// Be conservative.
	}

	creds.WriteString(`</credentials>`)
	return creds.String(), nil
}

func buildSMAPISOAPEnvelope(method string, args map[string]string, soapHeader string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>`)
	b.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`)
	if strings.TrimSpace(soapHeader) != "" {
		b.WriteString(`<s:Header>`)
		b.WriteString(soapHeader)
		b.WriteString(`</s:Header>`)
	}
	b.WriteString(`<s:Body>`)
	b.WriteString(`<`)
	b.WriteString(xmlEscapeTag(method))
	b.WriteString(` xmlns="`)
	b.WriteString(xmlEscapeAttr(smapiNamespace))
	b.WriteString(`">`)
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := args[k]
		// SMAPI parameters are generally simple strings.
		b.WriteString("<")
		b.WriteString(xmlEscapeTag(k))
		b.WriteString(">")
		b.WriteString(xmlEscapeText(v))
		b.WriteString("</")
		b.WriteString(xmlEscapeTag(k))
		b.WriteString(">")
	}
	b.WriteString(`</`)
	b.WriteString(xmlEscapeTag(method))
	b.WriteString(`>`)
	b.WriteString(`</s:Body></s:Envelope>`)
	return []byte(b.String())
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func extractSOAPBodyFirstChild(raw []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	const soapEnvNS = "http://schemas.xmlsoap.org/soap/envelope/"
	inBody := false
	bodyDepth := 0
	var buf bytes.Buffer
	var enc *xml.Encoder
	var captureDepth int

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !inBody {
				if t.Name.Space == soapEnvNS && t.Name.Local == "Body" {
					inBody = true
					bodyDepth = 1
				}
				continue
			}
			if inBody && captureDepth == 0 && bodyDepth == 1 {
				// First child of Body.
				enc = xml.NewEncoder(&buf)
				_ = enc.EncodeToken(t)
				captureDepth = 1
				continue
			}
			if captureDepth > 0 {
				_ = enc.EncodeToken(t)
				captureDepth++
				continue
			}
			bodyDepth++
		case xml.EndElement:
			if captureDepth > 0 {
				_ = enc.EncodeToken(t)
				captureDepth--
				if captureDepth == 0 {
					_ = enc.Flush()
					return buf.Bytes(), nil
				}
				continue
			}
			if inBody {
				bodyDepth--
				if bodyDepth == 0 {
					inBody = false
				}
			}
		case xml.CharData:
			if captureDepth > 0 {
				_ = enc.EncodeToken(t)
			}
		}
	}
	return nil, errors.New("missing soap body content")
}

type soapFault struct {
	FaultCode   string `xml:"faultcode"`
	FaultString string `xml:"faultstring"`
}

func parseSOAPFault(raw []byte) (soapFault, bool) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var inFault bool
	var f soapFault
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "Fault" {
				inFault = true
				continue
			}
			if !inFault {
				continue
			}
			switch t.Name.Local {
			case "faultcode":
				var v string
				_ = dec.DecodeElement(&v, &t)
				f.FaultCode = strings.TrimSpace(v)
			case "faultstring":
				var v string
				_ = dec.DecodeElement(&v, &t)
				f.FaultString = strings.TrimSpace(v)
			}
		case xml.EndElement:
			if inFault && t.Name.Local == "Fault" {
				return f, f.FaultCode != "" || f.FaultString != ""
			}
		}
	}
	return soapFault{}, false
}

func extractAuthTokenPair(raw []byte) (authToken, privateKey string) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "authToken":
			var v string
			_ = dec.DecodeElement(&v, &se)
			authToken = strings.TrimSpace(v)
		case "privateKey":
			var v string
			_ = dec.DecodeElement(&v, &se)
			privateKey = strings.TrimSpace(v)
		}
		if authToken != "" && privateKey != "" {
			return authToken, privateKey
		}
	}
	return authToken, privateKey
}
