package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type memSMAPITokenStore struct {
	tokens map[string]SMAPITokenPair
}

func (m *memSMAPITokenStore) Has(serviceID, householdID string) bool {
	_, ok, _ := m.Load(serviceID, householdID)
	return ok
}

func (m *memSMAPITokenStore) Load(serviceID, householdID string) (SMAPITokenPair, bool, error) {
	if m.tokens == nil {
		return SMAPITokenPair{}, false, nil
	}
	p, ok := m.tokens[smapiTokenKey(serviceID, householdID)]
	return p, ok, nil
}

func (m *memSMAPITokenStore) Save(serviceID, householdID string, pair SMAPITokenPair) error {
	if m.tokens == nil {
		m.tokens = map[string]SMAPITokenPair{}
	}
	m.tokens[smapiTokenKey(serviceID, householdID)] = pair
	return nil
}

func TestNewSMAPIClient(t *testing.T) {
	t.Parallel()

	if _, err := NewSMAPIClient(context.Background(), nil, MusicServiceDescriptor{SecureURI: "x"}, &memSMAPITokenStore{}); err == nil {
		t.Fatalf("expected error for nil speaker")
	}
	if _, err := NewSMAPIClient(context.Background(), &Client{}, MusicServiceDescriptor{}, &memSMAPITokenStore{}); err == nil {
		t.Fatalf("expected error for missing SecureURI")
	}
	if _, err := NewSMAPIClient(context.Background(), &Client{}, MusicServiceDescriptor{SecureURI: "x"}, nil); err == nil {
		t.Fatalf("expected error for nil token store")
	}

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "DeviceProperties:1#GetHouseholdID"):
			return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetHouseholdIDResponse xmlns:u="urn:schemas-upnp-org:service:DeviceProperties:1">
      <CurrentHouseholdID>Sonos_TEST</CurrentHouseholdID>
    </u:GetHouseholdIDResponse>
  </s:Body>
</s:Envelope>`), nil
		case strings.Contains(action, "SystemProperties:1#GetString"):
			return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetStringResponse xmlns:u="urn:schemas-upnp-org:service:SystemProperties:1">
      <StringValue>RINCON_DEVICEID</StringValue>
    </u:GetStringResponse>
  </s:Body>
</s:Envelope>`), nil
		default:
			t.Fatalf("unexpected SOAPACTION: %q", action)
			return nil, nil
		}
	})

	speaker := &Client{
		IP:   "192.0.2.1",
		Port: 1400,
		HTTP: &http.Client{Timeout: time.Second, Transport: rt},
	}
	store := &memSMAPITokenStore{}
	svc := MusicServiceDescriptor{ID: "2311", Name: "Spotify", SecureURI: "https://example.invalid/smapi", Auth: MusicServiceAuthDeviceLink}

	sm, err := NewSMAPIClient(context.Background(), speaker, svc, store)
	if err != nil {
		t.Fatalf("NewSMAPIClient: %v", err)
	}
	if sm.HouseholdID != "Sonos_TEST" || sm.DeviceID != "RINCON_DEVICEID" {
		t.Fatalf("unexpected ids: household=%q device=%q", sm.HouseholdID, sm.DeviceID)
	}
}

func TestSMAPIClient_BeginAndCompleteAuthentication_DeviceLink(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/smapi", func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(r.Header.Get("SOAPACTION"), `"`)
		switch action {
		case smapiSOAPAction + "getDeviceLinkCode":
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getDeviceLinkCodeResponse xmlns="http://www.sonos.com/Services/1.1">
      <getDeviceLinkCodeResult>
        <regUrl>https://example.com/link</regUrl>
        <linkCode>ABCD</linkCode>
        <linkDeviceId>DEVX</linkDeviceId>
      </getDeviceLinkCodeResult>
    </getDeviceLinkCodeResponse>
  </s:Body>
</s:Envelope>`))
		case smapiSOAPAction + "getDeviceAuthToken":
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getDeviceAuthTokenResponse xmlns="http://www.sonos.com/Services/1.1">
      <getDeviceAuthTokenResult>
        <authToken>tok</authToken>
        <privateKey>key</privateKey>
      </getDeviceAuthTokenResult>
    </getDeviceAuthTokenResponse>
  </s:Body>
</s:Envelope>`))
		default:
			t.Fatalf("unexpected SOAPACTION: %q", action)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	store := &memSMAPITokenStore{}
	sm := &SMAPIClient{
		httpClient: srv.Client(),
		Service: MusicServiceDescriptor{
			ID:          "2311",
			Name:        "Spotify",
			SecureURI:   srv.URL + "/smapi",
			Auth:        MusicServiceAuthDeviceLink,
			ServiceType: "59271",
		},
		HouseholdID: "Sonos_TEST",
		DeviceID:    "RINCON_DEVICEID",
		TokenStore:  store,
	}

	begin, err := sm.BeginAuthentication(context.Background())
	if err != nil {
		t.Fatalf("BeginAuthentication: %v", err)
	}
	if begin.RegURL != "https://example.com/link" || begin.LinkCode != "ABCD" || begin.LinkDeviceID != "DEVX" {
		t.Fatalf("unexpected begin: %#v", begin)
	}

	pair, err := sm.CompleteAuthentication(context.Background(), begin.LinkCode, "")
	if err != nil {
		t.Fatalf("CompleteAuthentication: %v", err)
	}
	if pair.AuthToken != "tok" || pair.PrivateKey != "key" || pair.UpdatedAt.IsZero() {
		t.Fatalf("unexpected pair: %#v", pair)
	}
	if _, ok, _ := store.Load(sm.Service.ID, sm.HouseholdID); !ok {
		t.Fatalf("expected token to be stored")
	}
}

func TestSMAPIClient_CompleteAuthentication_AcceptsTokenWithoutPrivateKey(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if action := strings.Trim(r.Header.Get("SOAPACTION"), `"`); action != smapiSOAPAction+"getDeviceAuthToken" {
			t.Fatalf("unexpected SOAPACTION: %q", action)
		}
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getDeviceAuthTokenResponse xmlns="http://www.sonos.com/Services/1.1">
      <getDeviceAuthTokenResult>
        <authToken> token-only </authToken>
      </getDeviceAuthTokenResult>
    </getDeviceAuthTokenResponse>
  </s:Body>
</s:Envelope>`))
	}))
	t.Cleanup(srv.Close)

	store, err := NewFileSMAPITokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	if err != nil {
		t.Fatalf("NewFileSMAPITokenStore: %v", err)
	}
	sm := &SMAPIClient{
		httpClient:  srv.Client(),
		Service:     MusicServiceDescriptor{ID: "233", Name: "Pocket Casts", SecureURI: srv.URL, Auth: MusicServiceAuthDeviceLink},
		HouseholdID: "Sonos_TEST",
		DeviceID:    "RINCON_DEVICEID",
		TokenStore:  store,
	}

	pair, err := sm.CompleteAuthentication(context.Background(), "ABCD", "")
	if err != nil {
		t.Fatalf("CompleteAuthentication: %v", err)
	}
	if pair.AuthToken != "token-only" || pair.PrivateKey != "" {
		t.Fatalf("unexpected pair: %#v", pair)
	}
	stored, ok, err := store.Load(sm.Service.ID, sm.HouseholdID)
	if err != nil || !ok {
		t.Fatalf("Load: ok=%v err=%v", ok, err)
	}
	if stored.AuthToken != "token-only" || stored.PrivateKey != "" {
		t.Fatalf("unexpected stored pair: %#v", stored)
	}
}

func TestSMAPIClient_CompleteAuthentication_RejectsMissingAuthToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getDeviceAuthTokenResponse xmlns="http://www.sonos.com/Services/1.1">
      <getDeviceAuthTokenResult><privateKey>refresh-key</privateKey></getDeviceAuthTokenResult>
    </getDeviceAuthTokenResponse>
  </s:Body>
</s:Envelope>`))
	}))
	t.Cleanup(srv.Close)

	store := &memSMAPITokenStore{}
	sm := &SMAPIClient{
		httpClient:  srv.Client(),
		Service:     MusicServiceDescriptor{ID: "233", SecureURI: srv.URL, Auth: MusicServiceAuthDeviceLink},
		HouseholdID: "Sonos_TEST",
		DeviceID:    "RINCON_DEVICEID",
		TokenStore:  store,
	}

	if _, err := sm.CompleteAuthentication(context.Background(), "ABCD", ""); err == nil {
		t.Fatal("expected missing auth token error")
	}
	if store.Has(sm.Service.ID, sm.HouseholdID) {
		t.Fatal("missing auth token must not be stored")
	}
}

func TestSMAPIClient_BeginAuthentication_AppLinkAndSearchCategories(t *testing.T) {
	t.Parallel()

	// Cached categories
	sm := &SMAPIClient{Service: MusicServiceDescriptor{Name: "X"}, searchPrefixMap: map[string]string{"tracks": "TR", "albums": "AL"}}
	cats, err := sm.SearchCategories(context.Background())
	if err != nil {
		t.Fatalf("SearchCategories: %v", err)
	}
	if len(cats) != 2 || cats[0] != "albums" || cats[1] != "tracks" {
		t.Fatalf("unexpected cats: %#v", cats)
	}

	// TuneIn special case
	sm2 := &SMAPIClient{Service: MusicServiceDescriptor{Name: "TuneIn"}}
	cats2, err := sm2.SearchCategories(context.Background())
	if err != nil {
		t.Fatalf("SearchCategories(TuneIn): %v", err)
	}
	if got := strings.Join(cats2, ","); got != "hosts,shows,stations" {
		t.Fatalf("unexpected tunein cats: %q", got)
	}

	// AppLink begin auth happy path
	var seenAppLinkBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/smapi", func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(r.Header.Get("SOAPACTION"), `"`)
		if action != smapiSOAPAction+"getAppLink" {
			t.Fatalf("SOAPACTION: %q", action)
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		seenAppLinkBody = string(body)
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getAppLinkResponse xmlns="http://www.sonos.com/Services/1.1">
      <getAppLinkResult>
        <authorizeAccount>
          <deviceLink>
            <regUrl>https://example.com/applink</regUrl>
            <linkCode>WXYZ</linkCode>
            <linkDeviceId>DEVY</linkDeviceId>
          </deviceLink>
        </authorizeAccount>
      </getAppLinkResult>
    </getAppLinkResponse>
  </s:Body>
</s:Envelope>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	sm3 := &SMAPIClient{
		httpClient:  srv.Client(),
		Service:     MusicServiceDescriptor{Name: "Svc", SecureURI: srv.URL + "/smapi", Auth: MusicServiceAuthAppLink},
		HouseholdID: "Sonos_TEST",
		DeviceID:    "DEV",
		TokenStore:  &memSMAPITokenStore{},
	}
	begin, err := sm3.BeginAuthentication(context.Background())
	if err != nil {
		t.Fatalf("BeginAuthentication(AppLink): %v", err)
	}
	if begin.LinkCode != "WXYZ" || begin.RegURL != "https://example.com/applink" {
		t.Fatalf("unexpected begin: %#v", begin)
	}
	for _, want := range []string{
		"<callbackPath></callbackPath>",
		"<hardware>CLI</hardware>",
		"<osVersion>1.0</osVersion>",
		"<sonosAppName>sonoscli</sonosAppName>",
	} {
		if !strings.Contains(seenAppLinkBody, want) {
			t.Fatalf("expected AppLink request to contain %s, got: %s", want, seenAppLinkBody)
		}
	}

	// Some AppLink services, including Apple Music, return only native app URLs
	// when the request looks like it came from a Sonos mobile controller.
	var appLinkRequests int
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/smapi", func(w http.ResponseWriter, r *http.Request) {
		appLinkRequests++
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		bodyText := string(body)
		if appLinkRequests == 1 {
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <ns2:getAppLinkResponse xmlns:ns2="http://www.sonos.com/Services/1.1">
      <ns2:getAppLinkResult>
        <ns2:callToAction/>
        <ns2:appUrlEncrypt>true</ns2:appUrlEncrypt>
      </ns2:getAppLinkResult>
    </ns2:getAppLinkResponse>
  </s:Body>
</s:Envelope>`))
			return
		}
		for _, want := range []string{
			"<hardware>iPhone15,2</hardware>",
			"<osVersion>Version 17.5</osVersion>",
			"<sonosAppName>ICRU_iPhone15,2</sonosAppName>",
			"sid%3D204%26OAuthDeviceID%3DSonos_TEST%26callbackPath%3D%2FaddAccount",
		} {
			if !strings.Contains(bodyText, want) {
				t.Fatalf("expected AppLink retry request to contain %s, got: %s", want, bodyText)
			}
		}
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <ns2:getAppLinkResponse xmlns:ns2="http://www.sonos.com/Services/1.1">
      <ns2:getAppLinkResult>
        <ns2:authorizeAccount>
          <ns2:appUrl>music://authorize</ns2:appUrl>
          <ns2:appUrlStringId>LoginAppleMusic</ns2:appUrlStringId>
        </ns2:authorizeAccount>
        <ns2:createAccount>
          <ns2:appUrl>music://trial</ns2:appUrl>
          <ns2:appUrlStringId>StartTrial</ns2:appUrlStringId>
        </ns2:createAccount>
        <ns2:appUrlEncrypt>true</ns2:appUrlEncrypt>
      </ns2:getAppLinkResult>
    </ns2:getAppLinkResponse>
  </s:Body>
</s:Envelope>`))
	})
	srv2 := httptest.NewServer(mux2)
	t.Cleanup(srv2.Close)
	smApple := &SMAPIClient{
		httpClient:  srv2.Client(),
		Service:     MusicServiceDescriptor{ID: "204", Name: "Apple Music", SecureURI: srv2.URL + "/smapi", Auth: MusicServiceAuthAppLink},
		HouseholdID: "Sonos_TEST",
		DeviceID:    "DEV",
		TokenStore:  &memSMAPITokenStore{},
	}
	appleBegin, err := smApple.BeginAuthentication(context.Background())
	if err != nil {
		t.Fatalf("BeginAuthentication(native AppLink): %v", err)
	}
	if appleBegin.AppURL != "music://authorize" || appleBegin.AppURLStringID != "LoginAppleMusic" || !appleBegin.AppURLEncrypt {
		t.Fatalf("unexpected native AppLink result: %#v", appleBegin)
	}
	if appleBegin.CreateAccountURL != "music://trial" || appleBegin.CreateAccountURLStringID != "StartTrial" {
		t.Fatalf("unexpected create-account AppLink result: %#v", appleBegin)
	}
	if appLinkRequests != 2 {
		t.Fatalf("expected AppLink retry, got %d requests", appLinkRequests)
	}

	sm4 := &SMAPIClient{Service: MusicServiceDescriptor{Name: "Svc", Auth: MusicServiceAuthAnonymous}}
	if _, err := sm4.BeginAuthentication(context.Background()); err == nil {
		t.Fatalf("expected error for unsupported auth")
	}
}

func TestSMAPI_PresentationMapFetching(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"presentationMap":{"uri":"` + r.Host + `/pmap"}}`))
	})
	mux.HandleFunc("/manifest_missing", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/pmap", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<PresentationMap>
  <SearchCategories>
    <Category id="tracks" mappedId="search:track" />
    <Category id="albums" mappedID="search:album" />
    <CustomCategory stringId="Blogs" mappedId="SBLG" />
  </SearchCategories>
</PresentationMap>`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	manifestURL := srv.URL + "/manifest"
	pmapURL, err := fetchPresentationMapURIFromManifest(context.Background(), srv.Client(), manifestURL)
	if err != nil {
		t.Fatalf("fetchPresentationMapURIFromManifest: %v", err)
	}
	if !strings.Contains(pmapURL, u.Host) {
		t.Fatalf("unexpected pmap url: %q", pmapURL)
	}

	m, err := fetchAndParsePresentationMap(context.Background(), srv.Client(), srv.URL+"/pmap")
	if err != nil {
		t.Fatalf("fetchAndParsePresentationMap: %v", err)
	}
	if m["tracks"] != "search:track" || m["albums"] != "search:album" || m["Blogs"] != "SBLG" {
		t.Fatalf("unexpected map: %#v", m)
	}

	if _, err := fetchPresentationMapURIFromManifest(context.Background(), srv.Client(), srv.URL+"/manifest_missing"); err == nil {
		t.Fatalf("expected error for missing presentationMap")
	}
}
