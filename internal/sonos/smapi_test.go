package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type memTokenStore struct {
	m map[string]SMAPITokenPair
}

func newMemTokenStore() *memTokenStore {
	return &memTokenStore{m: map[string]SMAPITokenPair{}}
}

func (s *memTokenStore) Has(serviceID, householdID string) bool {
	_, ok := s.m[smapiTokenKey(serviceID, householdID)]
	return ok
}

func (s *memTokenStore) Load(serviceID, householdID string) (SMAPITokenPair, bool, error) {
	p, ok := s.m[smapiTokenKey(serviceID, householdID)]
	return p, ok, nil
}

func (s *memTokenStore) Save(serviceID, householdID string, pair SMAPITokenPair) error {
	s.m[smapiTokenKey(serviceID, householdID)] = pair
	return nil
}

func TestSMAPI_Search_Success(t *testing.T) {
	var seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("SOAPACTION"); got != `"http://www.sonos.com/Services/1.1#search"` {
			t.Fatalf("unexpected SOAPACTION: %q", got)
		}
		b, _ := ioReadAllLimit(r.Body, 1<<20)
		seenBody = string(b)
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <searchResponse xmlns="http://www.sonos.com/Services/1.1">
      <searchResult>
        <index>0</index><count>1</count><total>1</total>
        <mediaMetadata>
          <id>spotify:track:abc</id>
          <itemType>track</itemType>
          <title>Gareth Emery</title>
          <mimeType>audio/x-spotify</mimeType>
        </mediaMetadata>
      </searchResult>
    </searchResponse>
  </s:Body>
</s:Envelope>`))
	}))
	defer srv.Close()

	store := newMemTokenStore()
	if err := store.Save("9", "Sonos_ABC", SMAPITokenPair{AuthToken: "T1", PrivateKey: "K1", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	c := &SMAPIClient{
		httpClient: srv.Client(),
		Service: MusicServiceDescriptor{
			ID:        "9",
			Name:      "Spotify",
			SecureURI: srv.URL,
			Auth:      MusicServiceAuthDeviceLink,
		},
		HouseholdID:     "Sonos_ABC",
		DeviceID:        "DEV",
		TokenStore:      store,
		searchPrefixMap: map[string]string{"tracks": "search:track"},
	}

	res, err := c.Search(context.Background(), "tracks", "gareth", 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 1 || len(res.MediaMetadata) != 1 {
		t.Fatalf("unexpected result: %#v", res)
	}
	if res.MediaMetadata[0].ID != "spotify:track:abc" {
		t.Fatalf("unexpected item id: %q", res.MediaMetadata[0].ID)
	}
	if !strings.Contains(seenBody, "<token>T1</token>") || !strings.Contains(seenBody, "<key>K1</key>") {
		t.Fatalf("request missing credentials header: %s", seenBody)
	}
}

func TestSMAPI_TokenRefresh(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		body, _ := ioReadAllLimit(r.Body, 1<<20)
		text := string(body)
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)

		if n == 1 {
			if !strings.Contains(text, "<token>OLD</token>") {
				t.Fatalf("expected OLD token in first request, got: %s", text)
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <s:Fault>
      <faultcode>Client.TokenRefreshRequired</faultcode>
      <faultstring>TokenRefreshRequired</faultstring>
      <detail xmlns:ms="http://www.sonos.com/Services/1.1">
        <ms:RefreshAuthTokenResult>
          <ms:authToken>NEW</ms:authToken>
          <ms:privateKey>NEWK</ms:privateKey>
        </ms:RefreshAuthTokenResult>
      </detail>
    </s:Fault>
  </s:Body>
</s:Envelope>`))
			return
		}

		if !strings.Contains(text, "<token>NEW</token>") || !strings.Contains(text, "<key>NEWK</key>") {
			t.Fatalf("expected NEW token on retry, got: %s", text)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <searchResponse xmlns="http://www.sonos.com/Services/1.1">
      <searchResult><index>0</index><count>0</count><total>0</total></searchResult>
    </searchResponse>
  </s:Body>
</s:Envelope>`))
	}))
	defer srv.Close()

	store := newMemTokenStore()
	if err := store.Save("9", "Sonos_ABC", SMAPITokenPair{AuthToken: "OLD", PrivateKey: "OLDK", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	c := &SMAPIClient{
		httpClient: srv.Client(),
		Service: MusicServiceDescriptor{
			ID:        "9",
			Name:      "Spotify",
			SecureURI: srv.URL,
			Auth:      MusicServiceAuthDeviceLink,
		},
		HouseholdID:     "Sonos_ABC",
		DeviceID:        "DEV",
		TokenStore:      store,
		searchPrefixMap: map[string]string{"tracks": "search:track"},
	}

	_, err := c.Search(context.Background(), "tracks", "x", 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	p, ok, _ := store.Load("9", "Sonos_ABC")
	if !ok || p.AuthToken != "NEW" || p.PrivateKey != "NEWK" {
		t.Fatalf("expected store to be updated, got: %#v", p)
	}
}

func TestSMAPI_Search_RequiresAuth(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := newMemTokenStore()
	c := &SMAPIClient{
		httpClient: srv.Client(),
		Service: MusicServiceDescriptor{
			ID:        "9",
			Name:      "Spotify",
			SecureURI: srv.URL,
			Auth:      MusicServiceAuthDeviceLink,
		},
		HouseholdID:     "Sonos_ABC",
		DeviceID:        "DEV",
		TokenStore:      store,
		searchPrefixMap: map[string]string{"tracks": "search:track"},
	}

	_, err := c.Search(context.Background(), "tracks", "x", 0, 10)
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("expected auth error, got: %v", err)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("expected no http calls, got %d", calls)
	}
}

func TestSMAPI_GetMetadata_Success(t *testing.T) {
	var seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("SOAPACTION"); got != `"http://www.sonos.com/Services/1.1#getMetadata"` {
			t.Fatalf("unexpected SOAPACTION: %q", got)
		}
		b, _ := ioReadAllLimit(r.Body, 1<<20)
		seenBody = string(b)
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <getMetadataResponse xmlns="http://www.sonos.com/Services/1.1">
      <getMetadataResult>
        <index>0</index><count>2</count><total>2</total>
        <mediaCollection>
          <id>spotify:playlist:pl123</id>
          <itemType>playlist</itemType>
          <title>My Playlist</title>
        </mediaCollection>
        <mediaMetadata>
          <id>spotify:track:abc</id>
          <itemType>track</itemType>
          <title>Track Title</title>
          <mimeType>audio/x-spotify</mimeType>
        </mediaMetadata>
      </getMetadataResult>
    </getMetadataResponse>
  </s:Body>
</s:Envelope>`))
	}))
	defer srv.Close()

	store := newMemTokenStore()
	if err := store.Save("9", "Sonos_ABC", SMAPITokenPair{AuthToken: "T1", PrivateKey: "K1", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	c := &SMAPIClient{
		httpClient: srv.Client(),
		Service: MusicServiceDescriptor{
			ID:        "9",
			Name:      "Spotify",
			SecureURI: srv.URL,
			Auth:      MusicServiceAuthDeviceLink,
		},
		HouseholdID:     "Sonos_ABC",
		DeviceID:        "DEV",
		TokenStore:      store,
		searchPrefixMap: map[string]string{"tracks": "search:track"},
	}

	res, err := c.GetMetadata(context.Background(), "root", 0, 50, false)
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("unexpected total: %d", res.Total)
	}
	if len(res.MediaCollection) != 1 || res.MediaCollection[0].ID != "spotify:playlist:pl123" {
		t.Fatalf("unexpected mediaCollection: %#v", res.MediaCollection)
	}
	if len(res.MediaMetadata) != 1 || res.MediaMetadata[0].ID != "spotify:track:abc" {
		t.Fatalf("unexpected mediaMetadata: %#v", res.MediaMetadata)
	}
	if !strings.Contains(seenBody, "<token>T1</token>") || !strings.Contains(seenBody, "<key>K1</key>") {
		t.Fatalf("request missing credentials header: %s", seenBody)
	}
}

func ioReadAllLimit(r io.ReadCloser, limit int64) ([]byte, error) {
	defer func() { _ = r.Close() }()
	return io.ReadAll(io.LimitReader(r, limit))
}
