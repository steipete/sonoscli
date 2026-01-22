package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestRealSpotifyEnqueuer_EnqueueSpotify(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
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
	mux.HandleFunc("/MediaRenderer/AVTransport/Control", func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "AVTransport:1#AddURIToQueue"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:AddURIToQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <FirstTrackNumberEnqueued>1</FirstTrackNumberEnqueued>
    </u:AddURIToQueueResponse>
  </s:Body>
</s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#SetAVTransportURI"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetAVTransportURIResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SetAVTransportURIResponse></s:Body></s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#Seek"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SeekResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SeekResponse></s:Body></s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#Play"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:PlayResponse></s:Body></s:Envelope>`))
		default:
			t.Fatalf("unexpected SOAPACTION: %q", action)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	c := &sonos.Client{
		IP:   u.Hostname(),
		Port: port,
		HTTP: srv.Client(),
	}

	enq := realSpotifyEnqueuer{c: c}
	if got := enq.CoordinatorIP(); got != c.IP {
		t.Fatalf("CoordinatorIP: %q", got)
	}

	pos, err := enq.EnqueueSpotify(context.Background(), "spotify:track:abc", sonos.EnqueueOptions{
		Title:   "X",
		PlayNow: true,
	})
	if err != nil {
		t.Fatalf("EnqueueSpotify: %v", err)
	}
	if pos != 1 {
		t.Fatalf("expected pos=1, got %d", pos)
	}

	// Ensure wrapper passes through context cancellation cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.HTTP.Timeout = 10 * time.Second
	_, _ = enq.EnqueueSpotify(ctx, "spotify:track:abc", sonos.EnqueueOptions{Title: "X"})
}
