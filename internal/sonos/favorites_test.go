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

func TestFavoriteURIFromResMD(t *testing.T) {
	f := DIDLItem{
		ResMD: `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="x"><res>http://example.com/stream</res></item></DIDL-Lite>`,
	}
	if got := favoriteURI(f); got != "http://example.com/stream" {
		t.Fatalf("favoriteURI: %q", got)
	}
}
