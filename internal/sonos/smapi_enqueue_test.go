package sonos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestEnqueueSMAPIItem(t *testing.T) {
	t.Parallel()

	var actions []string
	c := &Client{
		IP: "192.0.2.1",
		HTTP: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			action := r.Header.Get("SOAPACTION")
			var body []byte
			if r.Body != nil {
				body, _ = io.ReadAll(r.Body)
			}
			actions = append(actions, action)
			if r.Method == http.MethodGet {
				return httpResponse(200, `<?xml version="1.0"?>
<root>
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <manufacturer>Sonos, Inc.</manufacturer>
    <roomName>Office</roomName>
    <UDN>uuid:RINCON_OFFICE1400</UDN>
  </device>
</root>`), nil
			}
			if strings.Contains(action, "AddURIToQueue") || strings.Contains(string(body), "AddURIToQueue") {
				return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:AddURIToQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <FirstTrackNumberEnqueued>4</FirstTrackNumberEnqueued>
    </u:AddURIToQueueResponse>
  </s:Body>
</s:Envelope>`), nil
			}
			if strings.Contains(action, "SetAVTransportURI") || strings.Contains(action, "Seek") || strings.Contains(action, "Play") ||
				strings.Contains(string(body), "SetAVTransportURI") || strings.Contains(string(body), "Seek") || strings.Contains(string(body), "Play") {
				return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:OK xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:OK></s:Body></s:Envelope>`), nil
			}
			t.Fatalf("unexpected action: %s body: %s", action, body)
			return nil, nil
		})},
	}

	first, err := c.EnqueueSMAPIItem(context.Background(),
		MusicServiceDescriptor{ID: "2311", Name: "Spotify", Auth: MusicServiceAuthDeviceLink},
		SMAPIItem{ID: "album 123", Title: "Album", ItemType: "album"},
		EnqueueOptions{Position: -1, AsNext: true, PlayNow: true},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != 4 {
		t.Fatalf("first = %d, want 4", first)
	}
	if len(actions) < 4 {
		t.Fatalf("actions = %v, want add+transport setup", actions)
	}
}

func TestEnqueueSMAPIItemValidation(t *testing.T) {
	t.Parallel()

	c := &Client{IP: "192.0.2.1", HTTP: &http.Client{Timeout: time.Second}}
	if _, err := c.EnqueueSMAPIItem(context.Background(), MusicServiceDescriptor{ID: "1"}, SMAPIItem{}, EnqueueOptions{}); err == nil {
		t.Fatalf("expected missing item error")
	}
	if _, err := c.EnqueueSMAPIItem(context.Background(), MusicServiceDescriptor{}, SMAPIItem{ID: "track"}, EnqueueOptions{}); err == nil {
		t.Fatalf("expected missing service error")
	}
}

func TestSMAPIEnqueueHelpers(t *testing.T) {
	t.Parallel()

	if got := escapeSMAPIItemID("a b+c"); got != "a%20b%2Bc" {
		t.Fatalf("escaped id = %q", got)
	}
	if got := smapiDIDLItemID("SONG:449205:ST"); got != "0fffffffSONG%3A449205%3AST" {
		t.Fatalf("DIDL item id = %q", got)
	}
	if got := smapiEnqueuedURI("0fffffffSONG%3A449205%3AST", "track", "23"); got != "soco://0fffffffSONG%253A449205%253AST?sid=23&sn=0" {
		t.Fatalf("track enqueued URI = %q", got)
	}
	if got := smapiEnqueuedURI("0fffffffPLAYLIST%3A42", "playlist", "23"); got != "x-rincon-cpcontainer:0fffffffPLAYLIST%3A42" {
		t.Fatalf("playlist enqueued URI = %q", got)
	}
	if got := smapiServiceDesc(MusicServiceDescriptor{ID: "5"}); got != "SA_RINCON1287_" {
		t.Fatalf("service desc = %q", got)
	}
	if got := smapiServiceDesc(MusicServiceDescriptor{ID: "svc", ServiceType: "99", Auth: MusicServiceAuthAppLink}); got != "SA_RINCON99_" {
		t.Fatalf("AppLink service desc = %q", got)
	}
	if got := smapiServiceDesc(MusicServiceDescriptor{ID: "svc", ServiceType: "99", Auth: MusicServiceAuthDeviceLink}); got != "SA_RINCON99_X_#Svc99-0-Token" {
		t.Fatalf("DeviceLink service desc = %q", got)
	}
	didl := buildSMAPIDIDL(`id&`, `soco://id?a=1&b=2`, `desc"`)
	for _, want := range []string{"id&amp;", "soco://id?a=1&amp;b=2", "object.item", "desc&#34;"} {
		if !strings.Contains(didl, want) {
			t.Fatalf("DIDL missing %q: %s", want, didl)
		}
	}
}
