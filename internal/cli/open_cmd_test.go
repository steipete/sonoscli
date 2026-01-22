package cli

import (
	"context"
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

func TestOpenCmd_Plays(t *testing.T) {
	var playCalls atomic.Int32
	var addCalls atomic.Int32

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
			addCalls.Add(1)
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
			playCalls.Add(1)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:PlayResponse></s:Body></s:Envelope>`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: srv.Client()}
	}

	flags := &rootFlags{IP: u.Hostname(), Timeout: time.Second, Format: formatJSON}
	out, err := execute(t, newOpenCmd(flags), "spotify:track:abc")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !strings.Contains(out, "\"action\": \"open\"") {
		t.Fatalf("unexpected output: %q", out)
	}
	if addCalls.Load() == 0 || playCalls.Load() == 0 {
		t.Fatalf("expected AddURIToQueue and Play; add=%d play=%d", addCalls.Load(), playCalls.Load())
	}

	// Ensure context cancellation is surfaced as an error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := newOpenCmd(flags)
	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"spotify:track:abc"})
	_ = cmd.ExecuteContext(ctx)
}

func TestEnqueueCmd_DoesNotPlay(t *testing.T) {
	var playCalls atomic.Int32
	var addCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/MediaRenderer/AVTransport/Control", func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "AVTransport:1#AddURIToQueue"):
			addCalls.Add(1)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:AddURIToQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <FirstTrackNumberEnqueued>1</FirstTrackNumberEnqueued>
    </u:AddURIToQueueResponse>
  </s:Body>
</s:Envelope>`))
		case strings.Contains(action, "AVTransport:1#Play"):
			playCalls.Add(1)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:PlayResponse></s:Body></s:Envelope>`))
		default:
			// Enqueue should not need any other calls.
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: srv.Client()}
	}

	flags := &rootFlags{IP: u.Hostname(), Timeout: time.Second, Format: formatJSON}
	out, err := execute(t, newEnqueueCmd(flags), "spotify:track:abc")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if !strings.Contains(out, "\"action\": \"enqueue\"") {
		t.Fatalf("unexpected output: %q", out)
	}
	if addCalls.Load() == 0 {
		t.Fatalf("expected AddURIToQueue")
	}
	if playCalls.Load() != 0 {
		t.Fatalf("expected no Play, got %d", playCalls.Load())
	}
}
