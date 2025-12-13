package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestQueueListAndActions(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && r.URL.Path == "/xml/device_description.xml" {
			return httpResponse(200, `<?xml version="1.0"?>
<root>
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <manufacturer>Sonos, Inc.</manufacturer>
    <roomName>Office</roomName>
    <UDN>uuid:RINCON_ABC1400</UDN>
  </device>
</root>`), nil
		}

		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "ContentDirectory:1#Browse"):
			return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <Result>&lt;DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"&gt;&lt;item id="Q:0/1"&gt;&lt;dc:title&gt;Track 1&lt;/dc:title&gt;&lt;res&gt;x&lt;/res&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;</Result>
      <NumberReturned>1</NumberReturned>
      <TotalMatches>1</TotalMatches>
      <UpdateID>1</UpdateID>
    </u:BrowseResponse>
  </s:Body>
</s:Envelope>`), nil
		case strings.Contains(action, "AVTransport:1#RemoveAllTracksFromQueue"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:RemoveAllTracksFromQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:RemoveAllTracksFromQueueResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "AVTransport:1#RemoveTrackFromQueue"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:RemoveTrackFromQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:RemoveTrackFromQueueResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "AVTransport:1#SetAVTransportURI"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetAVTransportURIResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SetAVTransportURIResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "AVTransport:1#Seek"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SeekResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SeekResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "AVTransport:1#Play"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:PlayResponse></s:Body></s:Envelope>`), nil
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

	page, err := c.ListQueue(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("ListQueue: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Item.Title != "Track 1" || page.Items[0].Position != 1 {
		t.Fatalf("unexpected queue page: %+v", page)
	}

	if err := c.ClearQueue(context.Background()); err != nil {
		t.Fatalf("ClearQueue: %v", err)
	}
	if err := c.RemoveQueuePosition(context.Background(), 0); err == nil {
		t.Fatalf("expected error for invalid position")
	}
	if err := c.RemoveQueuePosition(context.Background(), 1); err != nil {
		t.Fatalf("RemoveQueuePosition: %v", err)
	}
	if err := c.PlayQueuePosition(context.Background(), 0); err == nil {
		t.Fatalf("expected error for invalid position")
	}

	// Not asserting network body here (covered by playFromQueueTrack tests).
	_ = c.PlayQueuePosition(context.Background(), 1)
}
