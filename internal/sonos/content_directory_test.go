package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBrowse(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.Header.Get("SOAPACTION"), "ContentDirectory:1#Browse") {
			t.Fatalf("SOAPACTION: %q", r.Header.Get("SOAPACTION"))
		}
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <Result>&lt;DIDL-Lite/&gt;</Result>
      <NumberReturned>1</NumberReturned>
      <TotalMatches>10</TotalMatches>
      <UpdateID>7</UpdateID>
    </u:BrowseResponse>
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

	br, err := c.Browse(context.Background(), "FV:2", 0, 100)
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if br.NumberReturned != 1 || br.TotalMatches != 10 || br.UpdateID != 7 {
		t.Fatalf("unexpected browse response: %+v", br)
	}
	if br.Result == "" {
		t.Fatalf("expected Result")
	}
}
