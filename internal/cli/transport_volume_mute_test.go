package cli

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

func soapActionResponse(serviceURN, action, innerXML string) string {
	return `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">` +
		`<s:Body>` +
		`<u:` + action + `Response xmlns:u="` + serviceURN + `">` +
		innerXML +
		`</u:` + action + `Response>` +
		`</s:Body>` +
		`</s:Envelope>`
}

func soapUPnPFault(code string) string {
	return `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">` +
		`<s:Body>` +
		`<s:Fault>` +
		`<faultcode>s:Client</faultcode>` +
		`<faultstring>UPnPError</faultstring>` +
		`<detail>` +
		`<UPnPError xmlns="urn:schemas-upnp-org:control-1-0">` +
		`<errorCode>` + code + `</errorCode>` +
		`<errorDescription>Transition not available</errorDescription>` +
		`</UPnPError>` +
		`</detail>` +
		`</s:Fault>` +
		`</s:Body>` +
		`</s:Envelope>`
}

func httpResponseWithStatus(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     strconv.Itoa(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestMuteGetPlain(t *testing.T) {
	flags := &rootFlags{IP: "192.0.2.10", Timeout: 2 * time.Second, Format: formatPlain}
	cmd := newMuteCmd(flags)

	var calls []string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		calls = append(calls, action)
		switch {
		case strings.Contains(action, "ZoneGroupTopology:1#GetZoneGroupState"):
			return httpResponseWithStatus(500, ""), nil
		case strings.Contains(action, "RenderingControl:1#GetMute"):
			return httpResponseWithStatus(
				200,
				soapActionResponse("urn:schemas-upnp-org:service:RenderingControl:1", "GetMute", `<CurrentMute>1</CurrentMute>`),
			), nil
		default:
			t.Fatalf("unexpected action: %q", action)
			return nil, nil
		}
	})

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   ip,
			Port: 1400,
			HTTP: &http.Client{Timeout: timeout, Transport: rt},
		}
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"get"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "true" {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if len(calls) != 2 {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestMuteToggleJSON(t *testing.T) {
	flags := &rootFlags{IP: "192.0.2.11", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newMuteCmd(flags)

	var calls []string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		calls = append(calls, action)
		switch {
		case strings.Contains(action, "ZoneGroupTopology:1#GetZoneGroupState"):
			return httpResponseWithStatus(500, ""), nil
		case strings.Contains(action, "RenderingControl:1#GetMute"):
			return httpResponseWithStatus(
				200,
				soapActionResponse("urn:schemas-upnp-org:service:RenderingControl:1", "GetMute", `<CurrentMute>0</CurrentMute>`),
			), nil
		case strings.Contains(action, "RenderingControl:1#SetMute"):
			return httpResponseWithStatus(
				200,
				soapActionResponse("urn:schemas-upnp-org:service:RenderingControl:1", "SetMute", ``),
			), nil
		default:
			t.Fatalf("unexpected action: %q", action)
			return nil, nil
		}
	})

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   ip,
			Port: 1400,
			HTTP: &http.Client{Timeout: timeout, Transport: rt},
		}
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"toggle"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "mute.toggle"`) || !strings.Contains(out.String(), `"mute": true`) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if got := strings.Join(calls, "\n"); !strings.Contains(got, "RenderingControl:1#GetMute") || !strings.Contains(got, "RenderingControl:1#SetMute") {
		t.Fatalf("missing calls: %#v", calls)
	}
}

func TestVolumeGetTSV(t *testing.T) {
	flags := &rootFlags{IP: "192.0.2.12", Timeout: 2 * time.Second, Format: formatTSV}
	cmd := newVolumeCmd(flags)

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "ZoneGroupTopology:1#GetZoneGroupState"):
			return httpResponseWithStatus(500, ""), nil
		case strings.Contains(action, "RenderingControl:1#GetVolume"):
			return httpResponseWithStatus(
				200,
				soapActionResponse("urn:schemas-upnp-org:service:RenderingControl:1", "GetVolume", `<CurrentVolume>33</CurrentVolume>`),
			), nil
		default:
			t.Fatalf("unexpected action: %q", action)
			return nil, nil
		}
	})

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   ip,
			Port: 1400,
			HTTP: &http.Client{Timeout: timeout, Transport: rt},
		}
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"get"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "volume\t33" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestVolumeSetClampsAndOutputsJSON(t *testing.T) {
	flags := &rootFlags{IP: "192.0.2.13", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newVolumeCmd(flags)

	var sawDesired string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "ZoneGroupTopology:1#GetZoneGroupState"):
			return httpResponseWithStatus(500, ""), nil
		case strings.Contains(action, "RenderingControl:1#SetVolume"):
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), "<DesiredVolume>") {
				start := strings.Index(string(b), "<DesiredVolume>") + len("<DesiredVolume>")
				end := strings.Index(string(b), "</DesiredVolume>")
				if start > 0 && end > start {
					sawDesired = string(b)[start:end]
				}
			}
			return httpResponseWithStatus(
				200,
				soapActionResponse("urn:schemas-upnp-org:service:RenderingControl:1", "SetVolume", ``),
			), nil
		default:
			t.Fatalf("unexpected action: %q", action)
			return nil, nil
		}
	})

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   ip,
			Port: 1400,
			HTTP: &http.Client{Timeout: timeout, Transport: rt},
		}
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"set", "120"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawDesired != "100" {
		t.Fatalf("expected DesiredVolume=100, got %q", sawDesired)
	}
	if !strings.Contains(out.String(), `"action": "volume.set"`) || !strings.Contains(out.String(), `"volume": 120`) {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestTransportPrevFallsBackToSeek(t *testing.T) {
	flags := &rootFlags{IP: "192.0.2.14", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newPrevCmd(flags)

	var calls []string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		calls = append(calls, action)
		switch {
		case strings.Contains(action, "ZoneGroupTopology:1#GetZoneGroupState"):
			return httpResponseWithStatus(500, ""), nil
		case strings.Contains(action, "AVTransport:1#Previous"):
			return httpResponseWithStatus(500, soapUPnPFault("701")), nil
		case strings.Contains(action, "AVTransport:1#Seek"):
			return httpResponseWithStatus(
				200,
				soapActionResponse("urn:schemas-upnp-org:service:AVTransport:1", "Seek", ``),
			), nil
		default:
			t.Fatalf("unexpected action: %q", action)
			return nil, nil
		}
	})

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   ip,
			Port: 1400,
			HTTP: &http.Client{Timeout: timeout, Transport: rt},
		}
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "prev"`) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	got := strings.Join(calls, "\n")
	if !strings.Contains(got, "AVTransport:1#Previous") || !strings.Contains(got, "AVTransport:1#Seek") {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestTransportPlayJSON(t *testing.T) {
	flags := &rootFlags{IP: "192.0.2.15", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newPlayCmd(flags)

	var calls []string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		calls = append(calls, action)
		switch {
		case strings.Contains(action, "ZoneGroupTopology:1#GetZoneGroupState"):
			return httpResponseWithStatus(500, ""), nil
		case strings.Contains(action, "AVTransport:1#Play"):
			return httpResponseWithStatus(
				200,
				soapActionResponse("urn:schemas-upnp-org:service:AVTransport:1", "Play", ``),
			), nil
		default:
			t.Fatalf("unexpected action: %q", action)
			return nil, nil
		}
	})

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   ip,
			Port: 1400,
			HTTP: &http.Client{Timeout: timeout, Transport: rt},
		}
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "play"`) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	got := strings.Join(calls, "\n")
	if !strings.Contains(got, "AVTransport:1#Play") {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestTransportPauseStopNextJSON(t *testing.T) {
	actions := []struct {
		name   string
		cmdFn  func(*rootFlags) *cobra.Command
		expect string
		ok     func(out string) bool
	}{
		{
			name:   "pause",
			cmdFn:  newPauseCmd,
			expect: "AVTransport:1#Pause",
			ok:     func(out string) bool { return strings.Contains(out, `"action": "pause"`) },
		},
		{
			name:   "stop",
			cmdFn:  newStopCmd,
			expect: "AVTransport:1#Stop",
			ok:     func(out string) bool { return strings.Contains(out, `"action": "stop"`) },
		},
		{
			name:   "next",
			cmdFn:  newNextCmd,
			expect: "AVTransport:1#Next",
			ok:     func(out string) bool { return strings.Contains(out, `"action": "next"`) },
		},
	}

	for _, tc := range actions {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{IP: "192.0.2.20", Timeout: 2 * time.Second, Format: formatJSON}
			cmd := tc.cmdFn(flags)

			var calls []string
			rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				action := r.Header.Get("SOAPACTION")
				calls = append(calls, action)
				switch {
				case strings.Contains(action, "ZoneGroupTopology:1#GetZoneGroupState"):
					return httpResponseWithStatus(500, ""), nil
				case strings.Contains(action, tc.expect):
					// Map the quoted SOAPACTION back to the raw method name for a response wrapper.
					method := strings.TrimPrefix(tc.expect, "AVTransport:1#")
					return httpResponseWithStatus(
						200,
						soapActionResponse("urn:schemas-upnp-org:service:AVTransport:1", method, ``),
					), nil
				default:
					t.Fatalf("unexpected action: %q", action)
					return nil, nil
				}
			})

			oldNew := newSonosClient
			t.Cleanup(func() { newSonosClient = oldNew })
			newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
				return &sonos.Client{
					IP:   ip,
					Port: 1400,
					HTTP: &http.Client{Timeout: timeout, Transport: rt},
				}
			}

			var out captureWriter
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{})
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			if err := cmd.ExecuteContext(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok(out.String()) {
				t.Fatalf("unexpected output: %q", out.String())
			}
			got := strings.Join(calls, "\n")
			if !strings.Contains(got, tc.expect) {
				t.Fatalf("missing expected call %q in %#v", tc.expect, calls)
			}
		})
	}
}
