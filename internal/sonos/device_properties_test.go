package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGetHouseholdID(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.Header.Get("SOAPACTION"), "DeviceProperties:1#GetHouseholdID") {
			t.Fatalf("SOAPACTION: %q", r.Header.Get("SOAPACTION"))
		}
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetHouseholdIDResponse xmlns:u="urn:schemas-upnp-org:service:DeviceProperties:1">
      <CurrentHouseholdID> Sonos_ABC </CurrentHouseholdID>
    </u:GetHouseholdIDResponse>
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

	hh, err := c.GetHouseholdID(context.Background())
	if err != nil {
		t.Fatalf("GetHouseholdID: %v", err)
	}
	if hh != "Sonos_ABC" {
		t.Fatalf("household id: %q", hh)
	}
}

func TestGetHouseholdID_MissingErrors(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetHouseholdIDResponse xmlns:u="urn:schemas-upnp-org:service:DeviceProperties:1">
      <CurrentHouseholdID>   </CurrentHouseholdID>
    </u:GetHouseholdIDResponse>
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
	if _, err := c.GetHouseholdID(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}
