package sonos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClientGetString_ValidatesInput(t *testing.T) {
	t.Parallel()

	c := &Client{IP: "192.0.2.1", HTTP: &http.Client{Timeout: time.Second}}
	if _, err := c.GetString(context.Background(), "   "); err == nil {
		t.Fatalf("expected error for empty variableName")
	}
}

func TestClientGetString_Success(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/SystemProperties/Control") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if got := r.Header.Get("SOAPACTION"); !strings.Contains(got, "SystemProperties:1#GetString") {
			t.Fatalf("SOAPACTION: %q", got)
		}
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if !strings.Contains(string(b), "<VariableName>R_TrialZPSerial</VariableName>") {
			t.Fatalf("expected VariableName in SOAP body, got: %s", string(b))
		}

		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetStringResponse xmlns:u="urn:schemas-upnp-org:service:SystemProperties:1">
      <StringValue>  hello  </StringValue>
    </u:GetStringResponse>
  </s:Body>
</s:Envelope>`), nil
	})

	c := &Client{
		IP: "192.0.2.1",
		HTTP: &http.Client{
			Timeout:   time.Second,
			Transport: rt,
		},
	}

	v, err := c.GetString(context.Background(), "  R_TrialZPSerial  ")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if v != "hello" {
		t.Fatalf("value: %q", v)
	}
}
