package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestWatchCmd_TSVAndMethodNotAllowed(t *testing.T) {
	callbackCh := make(chan string, 1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/ZoneGroupTopology/Control":
			w.WriteHeader(http.StatusInternalServerError)
			return
		case r.Method == "SUBSCRIBE" && r.URL.Path == "/MediaRenderer/AVTransport/Event":
			cb := strings.Trim(strings.TrimSpace(r.Header.Get("CALLBACK")), "<>")
			w.Header().Set("SID", "uuid:avt")
			w.Header().Set("TIMEOUT", "Second-1800")
			w.WriteHeader(http.StatusOK)
			select {
			case callbackCh <- cb:
			default:
			}
			return
		case r.Method == "SUBSCRIBE" && r.URL.Path == "/MediaRenderer/RenderingControl/Event":
			w.Header().Set("SID", "uuid:rc")
			w.Header().Set("TIMEOUT", "Second-1800")
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == "UNSUBSCRIBE":
			w.WriteHeader(http.StatusOK)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: srv.Client()}
	}

	flags := &rootFlags{IP: u.Hostname(), Timeout: 2 * time.Second, Format: formatTSV}
	cmd := newWatchCmd(flags)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--duration", "200ms"})

	var out syncBuffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.ExecuteContext(context.Background()) }()

	var callbackURL string
	select {
	case callbackURL = <-callbackCh:
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for callback")
	}

	// Ensure non-NOTIFY methods are rejected.
	resp, err := http.Get(callbackURL)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}

	// Emit a NOTIFY event.
	ev := `<e:propertyset xmlns:e="urn:schemas-upnp-org:event-1-0"><e:property>` +
		`<LastChange>` +
		`&lt;Event xmlns=&quot;urn:schemas-upnp-org:metadata-1-0/AVT/&quot;&gt;` +
		`&lt;InstanceID val=&quot;0&quot;&gt;` +
		`&lt;TransportState val=&quot;PLAYING&quot;/&gt;` +
		`&lt;/InstanceID&gt;` +
		`&lt;/Event&gt;` +
		`</LastChange>` +
		`</e:property></e:propertyset>`

	req, _ := http.NewRequest("NOTIFY", callbackURL, strings.NewReader(ev))
	req.Header.Set("SID", "uuid:avt")
	req.Header.Set("SEQ", "1")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	_, _ = io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watch: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("watch did not exit")
	}

	got := out.String()
	if !strings.Contains(got, "\tavtransport\tuuid:avt\ttransport_state\tPLAYING") {
		t.Fatalf("unexpected tsv output: %q", got)
	}
}

func TestWatchCmd_PlainParseError(t *testing.T) {
	callbackCh := make(chan string, 1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/ZoneGroupTopology/Control":
			w.WriteHeader(http.StatusInternalServerError)
			return
		case r.Method == "SUBSCRIBE" && r.URL.Path == "/MediaRenderer/AVTransport/Event":
			cb := strings.Trim(strings.TrimSpace(r.Header.Get("CALLBACK")), "<>")
			w.Header().Set("SID", "uuid:avt")
			w.Header().Set("TIMEOUT", "Second-1800")
			w.WriteHeader(http.StatusOK)
			select {
			case callbackCh <- cb:
			default:
			}
			return
		case r.Method == "SUBSCRIBE" && r.URL.Path == "/MediaRenderer/RenderingControl/Event":
			w.Header().Set("SID", "uuid:rc")
			w.Header().Set("TIMEOUT", "Second-1800")
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == "UNSUBSCRIBE":
			w.WriteHeader(http.StatusOK)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: u.Hostname(), Port: port, HTTP: srv.Client()}
	}

	flags := &rootFlags{IP: u.Hostname(), Timeout: 2 * time.Second, Format: formatPlain}
	cmd := newWatchCmd(flags)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--duration", "200ms"})

	var out syncBuffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.ExecuteContext(context.Background()) }()

	var callbackURL string
	select {
	case callbackURL = <-callbackCh:
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for callback")
	}

	// Send malformed XML body to trigger parse_error.
	req, _ := http.NewRequest("NOTIFY", callbackURL, strings.NewReader("<e:propertyset"))
	req.Header.Set("SID", "uuid:avt")
	req.Header.Set("SEQ", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watch: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("watch did not exit")
	}

	got := out.String()
	if !strings.Contains(got, "[avtransport]") || !strings.Contains(got, "parse_error=") {
		t.Fatalf("unexpected output: %q", got)
	}
}
