package sonos

import (
	"net/http"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("192.168.1.10", 123*time.Millisecond)
	if c.IP != "192.168.1.10" {
		t.Fatalf("IP: %q", c.IP)
	}
	if c.HTTP == nil {
		t.Fatalf("expected HTTP client")
	}
	tr, ok := c.HTTP.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.HTTP.Transport)
	}
	if !tr.DisableKeepAlives {
		t.Fatalf("expected DisableKeepAlives=true")
	}
}
