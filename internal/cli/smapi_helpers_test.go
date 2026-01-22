package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestAnySpeakerClient_BasicPaths(t *testing.T) {
	oldNew := newSonosClient
	oldDiscover := sonosDiscover
	t.Cleanup(func() {
		newSonosClient = oldNew
		sonosDiscover = oldDiscover
	})

	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: ip, Port: 1400, HTTP: &http.Client{Timeout: timeout}}
	}
	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return nil, nil
	}

	flags := &rootFlags{IP: " 10.0.0.9 ", Timeout: time.Second}
	c, err := anySpeakerClient(context.Background(), flags)
	if err != nil {
		t.Fatalf("anySpeakerClient(ip): %v", err)
	}
	if c.IP != "10.0.0.9" {
		t.Fatalf("unexpected ip: %q", c.IP)
	}

	flags2 := &rootFlags{Timeout: time.Second}
	if _, err := anySpeakerClient(context.Background(), flags2); err == nil {
		t.Fatalf("expected error for no speakers")
	}
}

func TestAnySpeakerClient_ByNameUsesTopology(t *testing.T) {
	oldNew := newSonosClient
	oldDiscover := sonosDiscover
	t.Cleanup(func() {
		newSonosClient = oldNew
		sonosDiscover = oldDiscover
	})

	zgs := `<ZoneGroupState><ZoneGroups><ZoneGroup Coordinator="RINCON_COORD1400" ID="RINCON_COORD1400:1">` +
		`<ZoneGroupMember ZoneName="Living Room" UUID="RINCON_COORD1400" Location="http://10.0.0.1:1400/xml/device_description.xml" Invisible="0" />` +
		`<ZoneGroupMember ZoneName="Office" UUID="RINCON_OFFICE1400" Location="http://10.0.0.2:1400/xml/device_description.xml" Invisible="0" />` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.Header.Get("SOAPACTION"), "ZoneGroupTopology:1#GetZoneGroupState") {
			return httpResponse(500, ""), nil
		}
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1">
      <ZoneGroupState><![CDATA[`+zgs+`]]></ZoneGroupState>
    </u:GetZoneGroupStateResponse>
  </s:Body>
</s:Envelope>`), nil
	})

	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{IP: ip, Port: 1400, HTTP: &http.Client{Timeout: timeout, Transport: rt}}
	}
	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return []sonos.Device{{IP: "10.0.0.1", Name: "Living Room"}}, nil
	}

	flags := &rootFlags{Name: "Office", Timeout: time.Second}
	c, err := anySpeakerClient(context.Background(), flags)
	if err != nil {
		t.Fatalf("anySpeakerClient(name): %v", err)
	}
	if c.IP != "10.0.0.2" {
		t.Fatalf("unexpected ip: %q", c.IP)
	}
}
