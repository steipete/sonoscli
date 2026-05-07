package sonos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFavoritesListAndPlayFavorite(t *testing.T) {
	t.Parallel()

	outerDIDL := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">` +
		`<item id="FV:2/1"><dc:title>Fav 1</dc:title><r:resMD>` +
		`&lt;DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"&gt;` +
		`&lt;item id="x"&gt;&lt;res&gt;spotify:track:abc&lt;/res&gt;&lt;/item&gt;` +
		`&lt;/DIDL-Lite&gt;` +
		`</r:resMD></item>` +
		`</DIDL-Lite>`
	escapedOuter := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(outerDIDL)

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("SOAPACTION")
		switch {
		case strings.Contains(action, "ContentDirectory:1#Browse"):
			return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <Result>`+escapedOuter+`</Result>
      <NumberReturned>1</NumberReturned>
      <TotalMatches>1</TotalMatches>
      <UpdateID>1</UpdateID>
    </u:BrowseResponse>
  </s:Body>
</s:Envelope>`), nil
		case strings.Contains(action, "AVTransport:1#SetAVTransportURI"):
			// URI should come from resMD (since outer item has no <res>).
			// Meta should be the full resMD string.
			// These will be entity-escaped inside the SOAP body.
			// Just assert key fragments exist.
			b, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			body := string(b)
			if !strings.Contains(body, "spotify:track:abc") {
				t.Fatalf("expected favorite URI in SOAP, body: %s", body)
			}
			if !strings.Contains(body, "DIDL-Lite") {
				t.Fatalf("expected metadata in SOAP, body: %s", body)
			}
			return httpResponse(200, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetAVTransportURIResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:SetAVTransportURIResponse></s:Body></s:Envelope>`), nil
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

	page, err := c.ListFavorites(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("ListFavorites: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Item.Title != "Fav 1" {
		t.Fatalf("unexpected favorites: %+v", page)
	}

	if err := c.PlayFavorite(context.Background(), page.Items[0].Item); err != nil {
		t.Fatalf("PlayFavorite: %v", err)
	}
}

func TestPlayFavoriteContainerClearsQueueAndPlaysFromFirstNewTrack(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		firstTrack string // FirstTrackNumberEnqueued returned by AddURIToQueue
		wantSeek   string // expected Target in Seek
	}{
		{
			name:       "cpcontainer-album",
			uri:        "x-rincon-cpcontainer:1004004cALkSOiEcdiS48fO281HFIkLxnGnQMSdsLxKYBp6iR_eSUEuO?sid=284&flags=76&sn=2",
			firstTrack: "5",
			wantSeek:   "5",
		},
		{
			name:       "cpcontainer-playlist",
			uri:        "x-rincon-cpcontainer:1006206cSomePlaylist?sid=9&sn=1",
			firstTrack: "1",
			wantSeek:   "1",
		},
		{
			name:       "missing-first-track-falls-back-to-1",
			uri:        "x-rincon-cpcontainer:1006206cAnotherPlaylist?sid=9&sn=1",
			firstTrack: "", // omitted by firmware
			wantSeek:   "1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deviceDescriptionXML := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <roomName>Living Room</roomName>
    <manufacturer>Sonos</manufacturer>
    <UDN>uuid:RINCON_LIVING1400</UDN>
  </device>
</root>`
			soapResp := func(action, inner string) string {
				return `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:` + action + `Response xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">` + inner + `</u:` + action + `Response></s:Body></s:Envelope>`
			}

			var actions []string
			rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet && r.URL.Path == "/xml/device_description.xml" {
					return httpResponse(200, deviceDescriptionXML), nil
				}
				action := r.Header.Get("SOAPACTION")
				switch {
				case strings.Contains(action, "AVTransport:1#RemoveAllTracksFromQueue"):
					actions = append(actions, "RemoveAllTracksFromQueue")
					return httpResponse(200, soapResp("RemoveAllTracksFromQueue", "")), nil
				case strings.Contains(action, "AVTransport:1#AddURIToQueue"):
					actions = append(actions, "AddURIToQueue")
					inner := ""
					if tc.firstTrack != "" {
						inner = "<FirstTrackNumberEnqueued>" + tc.firstTrack + "</FirstTrackNumberEnqueued>"
					}
					return httpResponse(200, soapResp("AddURIToQueue", inner)), nil
				case strings.Contains(action, "AVTransport:1#SetAVTransportURI"):
					b, _ := io.ReadAll(r.Body)
					_ = r.Body.Close()
					if !strings.Contains(string(b), "x-rincon-queue:RINCON_LIVING1400#0") {
						t.Fatalf("expected queue URI in SetAVTransportURI body: %s", string(b))
					}
					actions = append(actions, "SetAVTransportURI")
					return httpResponse(200, soapResp("SetAVTransportURI", "")), nil
				case strings.Contains(action, "AVTransport:1#Seek"):
					b, _ := io.ReadAll(r.Body)
					_ = r.Body.Close()
					body := string(b)
					if !strings.Contains(body, "<Unit>TRACK_NR</Unit>") || !strings.Contains(body, "<Target>"+tc.wantSeek+"</Target>") {
						t.Fatalf("expected seek to track %s, body: %s", tc.wantSeek, body)
					}
					actions = append(actions, "Seek")
					return httpResponse(200, soapResp("Seek", "")), nil
				case strings.Contains(action, "AVTransport:1#Play"):
					actions = append(actions, "Play")
					return httpResponse(200, soapResp("Play", "")), nil
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

			fav := DIDLItem{Title: tc.name, URI: tc.uri}
			if err := c.PlayFavorite(context.Background(), fav); err != nil {
				t.Fatalf("PlayFavorite: %v", err)
			}

			want := []string{"RemoveAllTracksFromQueue", "AddURIToQueue", "SetAVTransportURI", "Seek", "Play"}
			if strings.Join(actions, ",") != strings.Join(want, ",") {
				t.Fatalf("actions = %v, want %v", actions, want)
			}
		})
	}
}

func TestFavoriteURIFromResMD(t *testing.T) {
	f := DIDLItem{
		ResMD: `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="x"><res>http://example.com/stream</res></item></DIDL-Lite>`,
	}
	if got := favoriteURI(f); got != "http://example.com/stream" {
		t.Fatalf("favoriteURI: %q", got)
	}
}
