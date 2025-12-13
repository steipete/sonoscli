package sonos

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultHTTPClientDisablesKeepAlives(t *testing.T) {
	c := defaultHTTPClient(2 * time.Second)
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.Transport)
	}
	if !tr.DisableKeepAlives {
		t.Fatalf("expected DisableKeepAlives=true")
	}
}

func TestDefaultHTTPClientBypassesProxyForPrivateIPs(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.example:8080")
	t.Setenv("http_proxy", "http://proxy.example:8080")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	c := defaultHTTPClient(2 * time.Second)
	tr := c.Transport.(*http.Transport)
	if tr.Proxy == nil {
		t.Fatalf("expected Proxy func")
	}

	reqPrivate, err := http.NewRequest(http.MethodGet, "http://192.168.1.10:1400/status", nil)
	if err != nil {
		t.Fatalf("NewRequest(private): %v", err)
	}
	proxy, err := tr.Proxy(reqPrivate)
	if err != nil {
		t.Fatalf("Proxy(private): %v", err)
	}
	if proxy != nil {
		t.Fatalf("expected nil proxy for private IP, got %v", proxy)
	}

	reqPublic, err := http.NewRequest(http.MethodGet, "http://example.org/", nil)
	if err != nil {
		t.Fatalf("NewRequest(public): %v", err)
	}
	proxy, err = tr.Proxy(reqPublic)
	if err != nil {
		t.Fatalf("Proxy(public): %v", err)
	}
	if proxy == nil {
		t.Fatalf("expected proxy for public host when HTTP_PROXY is set")
	}
}
