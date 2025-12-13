package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAVTransportCommands_PauseNextBecomeCoordinator(t *testing.T) {
	t.Parallel()

	var seenPause, seenNext, seenBecome bool

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/MediaRenderer/AVTransport/Control") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "#Pause"):
			seenPause = true
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PauseResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:PauseResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#Next"):
			seenNext = true
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:NextResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:NextResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#BecomeCoordinatorOfStandaloneGroup"):
			seenBecome = true
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BecomeCoordinatorOfStandaloneGroupResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:BecomeCoordinatorOfStandaloneGroupResponse></s:Body></s:Envelope>`), nil
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

	if err := c.Pause(context.Background()); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if err := c.Next(context.Background()); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if err := c.BecomeCoordinatorOfStandaloneGroup(context.Background()); err != nil {
		t.Fatalf("BecomeCoordinatorOfStandaloneGroup: %v", err)
	}
	if !seenPause || !seenNext || !seenBecome {
		t.Fatalf("expected all actions called, pause=%v next=%v become=%v", seenPause, seenNext, seenBecome)
	}
}
