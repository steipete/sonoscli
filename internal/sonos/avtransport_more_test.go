package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAVTransportQueueAndInfoCalls(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "#RemoveAllTracksFromQueue"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:RemoveAllTracksFromQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:RemoveAllTracksFromQueueResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#RemoveTrackFromQueue"):
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:RemoveTrackFromQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:RemoveTrackFromQueueResponse></s:Body></s:Envelope>`), nil
		case strings.Contains(action, "#GetPositionInfo"):
			return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetPositionInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <Track>5</Track>
      <TrackURI>x-sonos-spotify:foo</TrackURI>
      <TrackMetaData>&lt;DIDL-Lite/&gt;</TrackMetaData>
      <TrackDuration>0:03:21</TrackDuration>
      <RelTime>0:00:10</RelTime>
    </u:GetPositionInfoResponse>
  </s:Body>
</s:Envelope>`), nil
		case strings.Contains(action, "#GetTransportInfo"):
			return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <CurrentTransportState>PLAYING</CurrentTransportState>
      <CurrentTransportStatus>OK</CurrentTransportStatus>
      <CurrentSpeed>1</CurrentSpeed>
    </u:GetTransportInfoResponse>
  </s:Body>
</s:Envelope>`), nil
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

	if err := c.RemoveAllTracksFromQueue(context.Background()); err != nil {
		t.Fatalf("RemoveAllTracksFromQueue: %v", err)
	}
	if err := c.RemoveTrackFromQueue(context.Background(), 3); err != nil {
		t.Fatalf("RemoveTrackFromQueue: %v", err)
	}

	pos, err := c.GetPositionInfo(context.Background())
	if err != nil {
		t.Fatalf("GetPositionInfo: %v", err)
	}
	if pos.Track != "5" || pos.TrackDuration != "0:03:21" || pos.RelTime != "0:00:10" {
		t.Fatalf("unexpected position info: %+v", pos)
	}

	ti, err := c.GetTransportInfo(context.Background())
	if err != nil {
		t.Fatalf("GetTransportInfo: %v", err)
	}
	if ti.State != "PLAYING" || ti.Status != "OK" || ti.Speed != "1" {
		t.Fatalf("unexpected transport info: %+v", ti)
	}
}
