package sonos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClientEnqueueSpotify_PlayNowTrack(t *testing.T) {
	t.Parallel()

	var addCalls int

	deviceDescriptionXML := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <roomName>Office</roomName>
    <manufacturer>Sonos</manufacturer>
    <UDN>uuid:RINCON_ABC1400</UDN>
  </device>
</root>`

	soapResp := func(action string, inner string) string {
		return `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:` + action + `Response xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">` + inner + `</u:` + action + `Response>
  </s:Body>
</s:Envelope>`
	}

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path != "/xml/device_description.xml" {
				t.Fatalf("GET path: %s", r.URL.Path)
			}
			return httpResponse(200, deviceDescriptionXML), nil
		case http.MethodPost:
			action := r.Header.Get("SOAPACTION")
			if action == "" {
				t.Fatalf("missing SOAPACTION")
			}
			b, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			body := string(b)

			switch {
			case strings.Contains(action, "#AddURIToQueue"):
				addCalls++
				if !strings.Contains(body, "<EnqueuedURI>x-sonos-spotify:spotify%3atrack%3aabc123?sid=2311&amp;sn=0</EnqueuedURI>") {
					t.Fatalf("unexpected EnqueuedURI body: %s", body)
				}
				if !strings.Contains(body, "SA_RINCON2311_X_#Svc2311-0-Token") {
					t.Fatalf("expected service token descriptor, body: %s", body)
				}
				if !strings.Contains(body, "Gareth Emery") {
					t.Fatalf("expected title in DIDL metadata, body: %s", body)
				}
				return httpResponse(200, soapResp("AddURIToQueue", "<FirstTrackNumberEnqueued>7</FirstTrackNumberEnqueued>")), nil
			case strings.Contains(action, "#SetAVTransportURI"):
				if !strings.Contains(body, "<CurrentURI>x-rincon-queue:RINCON_ABC1400#0</CurrentURI>") {
					t.Fatalf("expected queue URI, body: %s", body)
				}
				return httpResponse(200, soapResp("SetAVTransportURI", "")), nil
			case strings.Contains(action, "#Seek"):
				if !strings.Contains(body, "<Unit>TRACK_NR</Unit>") || !strings.Contains(body, "<Target>7</Target>") {
					t.Fatalf("expected seek track nr=7, body: %s", body)
				}
				return httpResponse(200, soapResp("Seek", "")), nil
			case strings.Contains(action, "#Play"):
				return httpResponse(200, soapResp("Play", "")), nil
			default:
				t.Fatalf("unexpected SOAPACTION %q", action)
				return nil, nil
			}
		default:
			t.Fatalf("method: %s", r.Method)
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

	first, err := c.EnqueueSpotify(context.Background(), "spotify:track:abc123", EnqueueOptions{
		Title:   "Gareth Emery",
		PlayNow: true,
	})
	if err != nil {
		t.Fatalf("EnqueueSpotify: %v", err)
	}
	if first != 7 {
		t.Fatalf("FirstTrackNumberEnqueued: %d", first)
	}
	if addCalls != 1 {
		t.Fatalf("expected exactly 1 AddURIToQueue call, got %d", addCalls)
	}
}

func TestClientEnqueueSpotify_InvalidInput(t *testing.T) {
	t.Parallel()

	c := &Client{IP: "192.0.2.1", HTTP: &http.Client{Timeout: time.Second}}
	_, err := c.EnqueueSpotify(context.Background(), "not spotify", EnqueueOptions{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestClientEnqueueSpotify_TrackFallbacksToLegacyURI(t *testing.T) {
	t.Parallel()

	var attemptedURIs []string

	soapResp := func(action string, inner string) string {
		return `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:` + action + `Response xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">` + inner + `</u:` + action + `Response>
  </s:Body>
</s:Envelope>`
	}

	upnpError800 := `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <s:Fault>
      <faultcode>s:Client</faultcode>
      <faultstring>UPnPError</faultstring>
      <detail>
        <UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
          <errorCode>800</errorCode>
          <errorDescription>Failed to queue item</errorDescription>
        </UPnPError>
      </detail>
    </s:Fault>
  </s:Body>
</s:Envelope>`

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		if !strings.Contains(action, "#AddURIToQueue") {
			t.Fatalf("unexpected SOAPACTION %q", action)
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		body := string(bodyBytes)

		switch {
		case strings.Contains(body, "<EnqueuedURI>x-sonos-spotify:spotify%3atrack%3aabc123?sid=2311&amp;sn=0</EnqueuedURI>"):
			attemptedURIs = append(attemptedURIs, "x-sonos-spotify:spotify%3atrack%3aabc123?sid=2311&sn=0")
			return httpResponse(500, upnpError800), nil
		case strings.Contains(body, "<EnqueuedURI>x-sonos-spotify:spotify%3atrack%3aabc123</EnqueuedURI>"):
			attemptedURIs = append(attemptedURIs, "x-sonos-spotify:spotify%3atrack%3aabc123")
			return httpResponse(200, soapResp("AddURIToQueue", "<FirstTrackNumberEnqueued>3</FirstTrackNumberEnqueued>")), nil
		default:
			t.Fatalf("unexpected EnqueuedURI body: %s", body)
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

	first, err := c.EnqueueSpotify(context.Background(), "spotify:track:abc123", EnqueueOptions{
		Title:   "Fallback",
		PlayNow: false,
	})
	if err != nil {
		t.Fatalf("EnqueueSpotify: %v", err)
	}
	if first != 3 {
		t.Fatalf("FirstTrackNumberEnqueued: %d", first)
	}
	expected := []string{
		"x-sonos-spotify:spotify%3atrack%3aabc123?sid=2311&sn=0",
		"x-sonos-spotify:spotify%3atrack%3aabc123",
	}
	if strings.Join(attemptedURIs, "|") != strings.Join(expected, "|") {
		t.Fatalf("attempted URIs mismatch: got %v want %v", attemptedURIs, expected)
	}
}
