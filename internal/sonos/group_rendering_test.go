package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGroupRenderingVolumeAndMute(t *testing.T) {
	t.Parallel()

	var sawSnapshot bool
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "#SnapshotGroupVolume"):
			sawSnapshot = true
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SnapshotGroupVolumeResponse xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1"></u:SnapshotGroupVolumeResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#GetGroupVolume"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetGroupVolumeResponse xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1"><CurrentVolume>33</CurrentVolume></u:GetGroupVolumeResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#SetGroupVolume"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetGroupVolumeResponse xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1"></u:SetGroupVolumeResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#GetGroupMute"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetGroupMuteResponse xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1"><CurrentMute>0</CurrentMute></u:GetGroupMuteResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#SetGroupMute"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetGroupMuteResponse xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1"></u:SetGroupMuteResponse></s:Body></s:Envelope>`), nil
		default:
			t.Fatalf("unexpected SOAPACTION: %q", action)
			return nil, nil
		}
	})

	c := &Client{
		IP: "192.0.2.1",
		HTTP: &http.Client{
			Timeout:   time.Second,
			Transport: rt,
		},
	}

	v, err := c.GetGroupVolume(context.Background())
	if err != nil {
		t.Fatalf("GetGroupVolume: %v", err)
	}
	if v != 33 {
		t.Fatalf("volume: %d", v)
	}

	if err := c.SetGroupVolume(context.Background(), 101); err != nil {
		t.Fatalf("SetGroupVolume: %v", err)
	}
	if !sawSnapshot {
		t.Fatalf("expected SnapshotGroupVolume to be called")
	}

	m, err := c.GetGroupMute(context.Background())
	if err != nil {
		t.Fatalf("GetGroupMute: %v", err)
	}
	if m {
		t.Fatalf("expected not muted")
	}
	if err := c.SetGroupMute(context.Background(), true); err != nil {
		t.Fatalf("SetGroupMute: %v", err)
	}
}
