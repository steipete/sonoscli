package sonos

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
)

type UPnPError struct {
	Code        string
	Description string
}

func (e *UPnPError) Error() string {
	if e.Description == "" {
		return "upnp error " + e.Code
	}
	return fmt.Sprintf("upnp error %s: %s", e.Code, e.Description)
}

func soapCall(ctx context.Context, httpClient *http.Client, endpointURL, serviceURN, action string, args map[string]string) (map[string]string, error) {
	body := buildSOAPEnvelope(serviceURN, action, args)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPACTION", fmt.Sprintf("%q", serviceURN+"#"+action))

	start := time.Now()
	slog.Debug("soap: request", "action", action, "endpoint", endpointURL)
	resp, err := doRequest(ctx, httpClient, req)
	if err != nil {
		slog.Debug("soap: request failed", "action", action, "endpoint", endpointURL, "elapsed", time.Since(start).String(), "err", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	slog.Debug("soap: response", "action", action, "endpoint", endpointURL, "status", resp.StatusCode, "bytes", len(raw), "elapsed", time.Since(start).String())

	if resp.StatusCode == 200 {
		return parseSOAPResponse(raw)
	}
	if resp.StatusCode == 500 {
		if upnpErr, ok := parseUPnPError(raw); ok {
			return nil, upnpErr
		}
	}
	return nil, fmt.Errorf("soap http %s", resp.Status)
}

func buildSOAPEnvelope(serviceURN, action string, args map[string]string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>`)
	b.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`)
	b.WriteString(`<s:Body>`)
	b.WriteString(`<u:`)
	b.WriteString(action)
	b.WriteString(` xmlns:u="`)
	b.WriteString(xmlEscapeAttr(serviceURN))
	b.WriteString(`">`)

	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := args[k]
		b.WriteString("<")
		b.WriteString(xmlEscapeTag(k))
		b.WriteString(">")
		b.WriteString(xmlEscapeText(v))
		b.WriteString("</")
		b.WriteString(xmlEscapeTag(k))
		b.WriteString(">")
	}

	b.WriteString(`</u:`)
	b.WriteString(action)
	b.WriteString(`>`)
	b.WriteString(`</s:Body></s:Envelope>`)
	return []byte(b.String())
}

func parseSOAPResponse(raw []byte) (map[string]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	out := map[string]string{}

	// We want the first element under <Body>, and then its direct children.
	const soapEnvNS = "http://schemas.xmlsoap.org/soap/envelope/"
	inBody := false
	bodyDepth := 0
	inResponse := false
	respDepth := 0
	var currentKey string

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return out, nil
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
			if inBody && !inResponse && bodyDepth == 1 {
				// This is the ActionResponse element.
				inResponse = true
				respDepth = 1
				continue
			}
			if inResponse {
				respDepth++
				if respDepth == 2 {
					currentKey = t.Name.Local
					out[currentKey] = ""
				}
			} else if inBody {
				bodyDepth++
			}
		case xml.EndElement:
			if inResponse {
				if respDepth == 2 {
					currentKey = ""
				}
				respDepth--
				if respDepth == 0 {
					inResponse = false
				}
			} else if inBody {
				bodyDepth--
				if bodyDepth == 0 {
					inBody = false
				}
			}
		case xml.CharData:
			if inResponse && respDepth == 2 && currentKey != "" {
				out[currentKey] += string(t)
			}
		}
	}
}

func parseUPnPError(raw []byte) (*UPnPError, bool) {
	type upnpErrBody struct {
		Code        string `xml:"errorCode"`
		Description string `xml:"errorDescription"`
	}

	// Parse generically: look for <errorCode> and <errorDescription> anywhere.
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var code, desc string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local == "UPnPError" {
			var body upnpErrBody
			if err := dec.DecodeElement(&body, &se); err == nil {
				code = strings.TrimSpace(body.Code)
				desc = strings.TrimSpace(body.Description)
				break
			}
		}
	}
	if code == "" && desc == "" {
		return nil, false
	}
	return &UPnPError{Code: code, Description: desc}, true
}

func xmlEscapeText(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func xmlEscapeAttr(s string) string {
	// Good enough for URNs; EscapeText also escapes quotes when used in attributes.
	return xmlEscapeText(s)
}

func xmlEscapeTag(s string) string {
	// Tag names are controlled by our code; keep it conservative.
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_' || r == '-':
			return r
		default:
			return -1
		}
	}, s)
}
