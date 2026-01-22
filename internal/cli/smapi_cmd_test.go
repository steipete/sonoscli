package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

type memTokenStore struct {
	tokens map[string]sonos.SMAPITokenPair
}

func (m *memTokenStore) Has(serviceID, householdID string) bool {
	_, ok, _ := m.Load(serviceID, householdID)
	return ok
}

func (m *memTokenStore) Load(serviceID, householdID string) (sonos.SMAPITokenPair, bool, error) {
	if m.tokens == nil {
		return sonos.SMAPITokenPair{}, false, nil
	}
	p, ok := m.tokens[strings.TrimSpace(serviceID)+"#"+strings.TrimSpace(householdID)]
	return p, ok, nil
}

func (m *memTokenStore) Save(serviceID, householdID string, pair sonos.SMAPITokenPair) error {
	if m.tokens == nil {
		m.tokens = map[string]sonos.SMAPITokenPair{}
	}
	m.tokens[strings.TrimSpace(serviceID)+"#"+strings.TrimSpace(householdID)] = pair
	return nil
}

type fakeSonosSMAPIServer struct {
	srv *httptest.Server

	playCalls         atomic.Int32
	addURIToQueueCall atomic.Int32
	authTokenCalls    atomic.Int32
}

func newFakeSonosSMAPIServer(t *testing.T) *fakeSonosSMAPIServer {
	t.Helper()

	fs := &fakeSonosSMAPIServer{}

	mux := http.NewServeMux()

	// Sonos device description, used by EnqueueSpotify play-from-queue path.
	mux.HandleFunc("/xml/device_description.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<root>
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <manufacturer>Sonos, Inc.</manufacturer>
    <roomName>Office</roomName>
    <UDN>uuid:RINCON_OFFICE1400</UDN>
  </device>
</root>`))
	})

	// UPnP SOAP endpoints.
	mux.HandleFunc("/MusicServices/Control", func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		if !strings.Contains(action, "MusicServices:1#ListAvailableServices") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		servicesXML := `<Services SchemaVersion="1">
  <Service Id="2311" Name="Spotify" Version="1.1" Uri="http://example" SecureUri="` + fs.srv.URL + `/smapi" ContainerType="MService" Capabilities="513">
    <Policy Auth="DeviceLink" />
    <Presentation>
      <PresentationMap Version="2" Uri="` + fs.srv.URL + `/pmap" />
    </Presentation>
  </Service>
</Services>`
		escaped := strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(servicesXML)
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:ListAvailableServicesResponse xmlns:u="urn:schemas-upnp-org:service:MusicServices:1">
      <AvailableServiceDescriptorList>` + escaped + `</AvailableServiceDescriptorList>
    </u:ListAvailableServicesResponse>
  </s:Body>
</s:Envelope>`))
	})

	mux.HandleFunc("/DeviceProperties/Control", func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		if !strings.Contains(action, "DeviceProperties:1#GetHouseholdID") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetHouseholdIDResponse xmlns:u="urn:schemas-upnp-org:service:DeviceProperties:1">
      <CurrentHouseholdID>Sonos_TEST</CurrentHouseholdID>
    </u:GetHouseholdIDResponse>
  </s:Body>
</s:Envelope>`))
	})

	mux.HandleFunc("/SystemProperties/Control", func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		if !strings.Contains(action, "SystemProperties:1#GetString") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetStringResponse xmlns:u="urn:schemas-upnp-org:service:SystemProperties:1">
      <StringValue>RINCON_DEVICEID</StringValue>
    </u:GetStringResponse>
  </s:Body>
</s:Envelope>`))
	})

	mux.HandleFunc("/MediaRenderer/AVTransport/Control", func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "AVTransport:1#AddURIToQueue"):
			fs.addURIToQueueCall.Add(1)
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:AddURIToQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <FirstTrackNumberEnqueued>1</FirstTrackNumberEnqueued>
    </u:AddURIToQueueResponse>
  </s:Body>
</s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#SetAVTransportURI"):
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetAVTransportURIResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SetAVTransportURIResponse></s:Body></s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#Seek"):
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SeekResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SeekResponse></s:Body></s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#Play"):
			fs.playCalls.Add(1)
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:PlayResponse></s:Body></s:Envelope>`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	// Presentation map for search categories.
	mux.HandleFunc("/pmap", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<PresentationMap>
  <Nested>
    <SearchCategories>
      <Category id="tracks" mappedId="search:track"/>
      <Category id="albums" mappedId="search:album"/>
    </SearchCategories>
  </Nested>
</PresentationMap>`))
	})

	// SMAPI endpoint.
	mux.HandleFunc("/smapi", func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(r.Header.Get("SOAPACTION"), `"`)
		switch action {
		case "http://www.sonos.com/Services/1.1#getDeviceLinkCode":
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getDeviceLinkCodeResponse xmlns="http://www.sonos.com/Services/1.1">
      <getDeviceLinkCodeResult>
        <regUrl>https://example.invalid/link</regUrl>
        <linkCode>ABCD</linkCode>
        <linkDeviceId>DEVX</linkDeviceId>
      </getDeviceLinkCodeResult>
    </getDeviceLinkCodeResponse>
  </s:Body>
</s:Envelope>`))
		case "http://www.sonos.com/Services/1.1#getDeviceAuthToken":
			call := fs.authTokenCalls.Add(1)
			// First call: NOT_LINKED_RETRY fault; second call: success.
			if call == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <s:Fault>
      <faultcode>SOAP-ENV:Server</faultcode>
      <faultstring>NOT_LINKED_RETRY</faultstring>
      <detail/>
    </s:Fault>
  </s:Body>
</s:Envelope>`))
				return
			}
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
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
		case "http://www.sonos.com/Services/1.1#search":
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <searchResponse xmlns="http://www.sonos.com/Services/1.1">
      <searchResult>
        <index>0</index>
        <count>1</count>
        <total>1</total>
        <mediaMetadata>
          <id>spotify:track:abc</id>
          <itemType>track</itemType>
          <title>Gareth Emery</title>
          <mimeType>audio/x-spotify</mimeType>
          <summary></summary>
        </mediaMetadata>
      </searchResult>
    </searchResponse>
  </s:Body>
</s:Envelope>`))
		case "http://www.sonos.com/Services/1.1#getMetadata":
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getMetadataResponse xmlns="http://www.sonos.com/Services/1.1">
      <getMetadataResult>
        <index>0</index>
        <count>2</count>
        <total>2</total>
        <mediaCollection>
          <id>spotify:album:zzz</id>
          <itemType>album</itemType>
          <title>An Album</title>
          <summary></summary>
        </mediaCollection>
        <mediaMetadata>
          <id>spotify:track:abc</id>
          <itemType>track</itemType>
          <title>Gareth Emery</title>
          <mimeType>audio/x-spotify</mimeType>
          <summary></summary>
        </mediaMetadata>
      </getMetadataResult>
    </getMetadataResponse>
  </s:Body>
</s:Envelope>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	fs.srv = httptest.NewServer(mux)
	t.Cleanup(fs.srv.Close)
	return fs
}

func TestSMAPICategoriesCmd_PlainAndJSON(t *testing.T) {
	fs := newFakeSonosSMAPIServer(t)
	u, _ := url.Parse(fs.srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	oldStore := newSMAPITokenStore
	t.Cleanup(func() {
		newSonosClient = oldNew
		newSMAPITokenStore = oldStore
	})

	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: fs.srv.Client()}
	}
	newSMAPITokenStore = func() (sonos.SMAPITokenStore, error) {
		return &memTokenStore{}, nil
	}

	flags := &rootFlags{IP: u.Hostname(), Timeout: time.Second, Format: formatPlain}
	out, err := execute(t, newSMAPICategoriesCmd(flags), "--service", "Spotify")
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	if !strings.Contains(out, "albums") || !strings.Contains(out, "tracks") {
		t.Fatalf("unexpected output: %q", out)
	}

	flagsJSON := &rootFlags{IP: u.Hostname(), Timeout: time.Second, Format: formatJSON}
	out2, err := execute(t, newSMAPICategoriesCmd(flagsJSON), "--service", "Spotify")
	if err != nil {
		t.Fatalf("categories json: %v", err)
	}
	if !strings.Contains(out2, "\"categories\"") || !strings.Contains(out2, "tracks") {
		t.Fatalf("unexpected json: %q", out2)
	}
}

func TestSMAPISearchCmd_OpenPlaysOnSonos(t *testing.T) {
	fs := newFakeSonosSMAPIServer(t)
	u, _ := url.Parse(fs.srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	oldStore := newSMAPITokenStore
	t.Cleanup(func() {
		newSonosClient = oldNew
		newSMAPITokenStore = oldStore
	})

	store := &memTokenStore{}
	_ = store.Save("2311", "Sonos_TEST", sonos.SMAPITokenPair{AuthToken: "t", PrivateKey: "k"})
	newSMAPITokenStore = func() (sonos.SMAPITokenStore, error) { return store, nil }

	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: fs.srv.Client()}
	}

	flags := &rootFlags{IP: u.Hostname(), Timeout: 2 * time.Second, Format: formatJSON}
	out, err := execute(t, newSMAPISearchCmd(flags), "--service", "Spotify", "--category", "tracks", "--open", "--index", "1", "gareth")
	if err != nil {
		t.Fatalf("smapi search --open: %v", err)
	}
	if !strings.Contains(out, "\"result\"") || !strings.Contains(out, "spotify:track:abc") {
		t.Fatalf("unexpected output: %q", out)
	}
	if fs.playCalls.Load() == 0 {
		t.Fatalf("expected Play to be called")
	}
}

func TestSMAPIBrowseCmd_TableOutput(t *testing.T) {
	fs := newFakeSonosSMAPIServer(t)
	u, _ := url.Parse(fs.srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	oldStore := newSMAPITokenStore
	t.Cleanup(func() {
		newSonosClient = oldNew
		newSMAPITokenStore = oldStore
	})

	store := &memTokenStore{}
	_ = store.Save("2311", "Sonos_TEST", sonos.SMAPITokenPair{AuthToken: "t", PrivateKey: "k"})
	newSMAPITokenStore = func() (sonos.SMAPITokenStore, error) { return store, nil }

	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: fs.srv.Client()}
	}

	flags := &rootFlags{IP: u.Hostname(), Timeout: 2 * time.Second, Format: formatPlain}
	out, err := execute(t, newSMAPIBrowseCmd(flags), "--service", "Spotify", "--id", "root", "--limit", "2")
	if err != nil {
		t.Fatalf("smapi browse: %v", err)
	}
	if !strings.Contains(out, "INDEX") || !strings.Contains(out, "spotify:track:abc") || !strings.Contains(out, "spotify:album:zzz") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestAuthSMAPI_BeginAndCompleteWithWait(t *testing.T) {
	fs := newFakeSonosSMAPIServer(t)
	u, _ := url.Parse(fs.srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	oldStore := newSMAPITokenStore
	t.Cleanup(func() {
		newSonosClient = oldNew
		newSMAPITokenStore = oldStore
	})

	store := &memTokenStore{}
	newSMAPITokenStore = func() (sonos.SMAPITokenStore, error) { return store, nil }

	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: fs.srv.Client()}
	}

	flagsBegin := &rootFlags{IP: u.Hostname(), Timeout: time.Second, Format: formatJSON}
	out, err := execute(t, newSMAPIAuthBeginCmd(flagsBegin), "--service", "Spotify")
	if err != nil {
		t.Fatalf("auth begin: %v", err)
	}
	if !strings.Contains(out, "\"linkCode\"") || !strings.Contains(out, "ABCD") {
		t.Fatalf("unexpected output: %q", out)
	}

	flagsComplete := &rootFlags{IP: u.Hostname(), Timeout: time.Second, Format: formatJSON}
	out2, err := execute(t, newSMAPIAuthCompleteCmd(flagsComplete), "--service", "Spotify", "--code", "ABCD", "--wait", "60ms")
	if err != nil {
		t.Fatalf("auth complete: %v", err)
	}
	if !strings.Contains(out2, "\"token\"") || !strings.Contains(out2, "\"authToken\"") {
		t.Fatalf("unexpected output: %q", out2)
	}
	if fs.authTokenCalls.Load() < 2 {
		t.Fatalf("expected polling retries, got %d calls", fs.authTokenCalls.Load())
	}
	if !store.Has("2311", "Sonos_TEST") {
		t.Fatalf("expected token to be stored")
	}
}

func TestSMAPISearchCmd_OpenRequiresTarget(t *testing.T) {
	flags := &rootFlags{Timeout: time.Second, Format: formatPlain}
	_, err := execute(t, newSMAPISearchCmd(flags), "--open", "hello")
	if err == nil || !strings.Contains(err.Error(), "require --ip or --name") {
		t.Fatalf("expected missing target error, got: %v", err)
	}
}

func TestCompleteSMAPIAuth_RespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	_, err := completeSMAPIAuth(ctx, 5*time.Second, func(context.Context) (sonos.SMAPITokenPair, error) {
		attempts++
		if attempts == 1 {
			cancel()
		}
		return sonos.SMAPITokenPair{}, errors.New("NOT_LINKED_RETRY")
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error")
	}
}
