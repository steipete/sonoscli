package sonos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGetTransportSettings(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/MediaRenderer/AVTransport/Control") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		action := r.Header.Get("SOAPACTION")
		if !strings.Contains(action, "#GetTransportSettings") {
			t.Fatalf("unexpected SOAPACTION: %q", action)
		}
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:GetTransportSettingsResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
<PlayMode>SHUFFLE</PlayMode>
<RecQualityMode>NOT_IMPLEMENTED</RecQualityMode>
</u:GetTransportSettingsResponse>
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

	settings, err := c.GetTransportSettings(context.Background())
	if err != nil {
		t.Fatalf("GetTransportSettings: %v", err)
	}
	if settings.PlayMode != PlayModeShuffle {
		t.Errorf("PlayMode = %q, want %q", settings.PlayMode, PlayModeShuffle)
	}
	if settings.RecQualityMode != "NOT_IMPLEMENTED" {
		t.Errorf("RecQualityMode = %q, want NOT_IMPLEMENTED", settings.RecQualityMode)
	}
}

func TestSetPlayMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode PlayMode
	}{
		{"normal", PlayModeNormal},
		{"shuffle", PlayModeShuffle},
		{"shuffle_norepeat", PlayModeShuffleNoRepeat},
		{"repeat_all", PlayModeRepeatAll},
		{"repeat_one", PlayModeRepeatOne},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestedMode string
			rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("method: %s", r.Method)
				}
				action := r.Header.Get("SOAPACTION")
				if !strings.Contains(action, "#SetPlayMode") {
					t.Fatalf("unexpected SOAPACTION: %q", action)
				}
				// Read body to extract the mode
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), string(tt.mode)) {
					requestedMode = string(tt.mode)
				}
				return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:SetPlayModeResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
</u:SetPlayModeResponse>
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

			if err := c.SetPlayMode(context.Background(), tt.mode); err != nil {
				t.Fatalf("SetPlayMode(%s): %v", tt.mode, err)
			}
			if requestedMode != string(tt.mode) {
				t.Errorf("requested mode = %q, want %q", requestedMode, tt.mode)
			}
		})
	}
}
