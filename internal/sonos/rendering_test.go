package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRenderingVolumeAndMute(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "#GetVolume"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetVolumeResponse xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"><CurrentVolume>25</CurrentVolume></u:GetVolumeResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#SetVolume"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetVolumeResponse xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"></u:SetVolumeResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#GetMute"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetMuteResponse xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"><CurrentMute>1</CurrentMute></u:GetMuteResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#SetMute"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetMuteResponse xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"></u:SetMuteResponse></s:Body></s:Envelope>`), nil
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

	v, err := c.GetVolume(context.Background())
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if v != 25 {
		t.Fatalf("volume: %d", v)
	}
	if err := c.SetVolume(context.Background(), 101); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}

	m, err := c.GetMute(context.Background())
	if err != nil {
		t.Fatalf("GetMute: %v", err)
	}
	if !m {
		t.Fatalf("expected muted")
	}
	if err := c.SetMute(context.Background(), true); err != nil {
		t.Fatalf("SetMute: %v", err)
	}
}
