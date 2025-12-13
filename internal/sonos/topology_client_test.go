package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClientGetTopology_Success(t *testing.T) {
	t.Parallel()

	zgs := `<ZoneGroupState><ZoneGroups><ZoneGroup Coordinator="RINCON_ABC1400" ID="RINCON_ABC1400:1"><ZoneGroupMember ZoneName="Office" UUID="RINCON_ABC1400" Location="http://192.168.1.10:1400/xml/device_description.xml" Invisible="0" /></ZoneGroup></ZoneGroups></ZoneGroupState>`
	escaped := strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(zgs)

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/ZoneGroupTopology/Control") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if got := r.Header.Get("SOAPACTION"); !strings.Contains(got, "ZoneGroupTopology:1#GetZoneGroupState") {
			t.Fatalf("SOAPACTION: %q", got)
		}
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1">
      <ZoneGroupState>`+escaped+`</ZoneGroupState>
    </u:GetZoneGroupStateResponse>
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

	top, err := c.GetTopology(context.Background())
	if err != nil {
		t.Fatalf("GetTopology: %v", err)
	}
	if mem, ok := top.FindByName("Office"); !ok || mem.IP != "192.168.1.10" {
		t.Fatalf("expected Office member, got ok=%v mem=%+v", ok, mem)
	}
}

func TestClientGetTopology_MissingZoneGroupStateErrors(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1">
      <Nope></Nope>
    </u:GetZoneGroupStateResponse>
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
	_, err := c.GetTopology(context.Background())
	if err == nil || !strings.Contains(err.Error(), "zone group state missing") {
		t.Fatalf("expected missing ZoneGroupState error, got: %v", err)
	}
}
