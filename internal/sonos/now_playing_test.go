package sonos

import "testing"

func TestAlbumArtURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		deviceIP   string
		albumArt   string
		wantResult string
	}{
		{name: "empty", deviceIP: "1.2.3.4", albumArt: "", wantResult: ""},
		{name: "absolute-http", deviceIP: "1.2.3.4", albumArt: "http://example.com/x.jpg", wantResult: "http://example.com/x.jpg"},
		{name: "absolute-https", deviceIP: "1.2.3.4", albumArt: "https://example.com/x.jpg", wantResult: "https://example.com/x.jpg"},
		{name: "relative-with-ip", deviceIP: "1.2.3.4", albumArt: "/getaa?s=1&u=abc", wantResult: "http://1.2.3.4:1400/getaa?s=1&u=abc"},
		{name: "relative-no-ip", deviceIP: "", albumArt: "/getaa?s=1&u=abc", wantResult: "/getaa?s=1&u=abc"},
		{name: "non-http-non-path", deviceIP: "1.2.3.4", albumArt: "x-sonos-http:foo", wantResult: "x-sonos-http:foo"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := AlbumArtURL(tt.deviceIP, tt.albumArt)
			if got != tt.wantResult {
				t.Fatalf("got %q, want %q", got, tt.wantResult)
			}
		})
	}
}

func TestParseNowPlaying(t *testing.T) {
	t.Parallel()

	didl := `<?xml version="1.0" encoding="utf-8"?>
<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"
  xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
  <item id="R:0/0/0" parentID="R:0/0" restricted="true">
    <dc:title>My Song</dc:title>
    <upnp:artist>My Artist</upnp:artist>
    <upnp:album>My Album</upnp:album>
    <upnp:albumArtURI>/getaa?s=1&amp;u=abc</upnp:albumArtURI>
    <res>https://example.com/stream.mp3</res>
  </item>
</DIDL-Lite>`

	it, ok := ParseNowPlaying(didl)
	if !ok {
		t.Fatalf("expected ok")
	}
	if it.Title != "My Song" {
		t.Fatalf("unexpected title: %q", it.Title)
	}
	if it.Artist != "My Artist" {
		t.Fatalf("unexpected artist: %q", it.Artist)
	}
	if it.Album != "My Album" {
		t.Fatalf("unexpected album: %q", it.Album)
	}
	if it.AlbumArtURI != "/getaa?s=1&u=abc" {
		t.Fatalf("unexpected albumArtURI: %q", it.AlbumArtURI)
	}
	if it.URI != "https://example.com/stream.mp3" {
		t.Fatalf("unexpected uri: %q", it.URI)
	}
}

func TestParseNowPlayingEmpty(t *testing.T) {
	t.Parallel()
	if _, ok := ParseNowPlaying(""); ok {
		t.Fatalf("expected not ok")
	}
}

func TestParseNowPlayingInvalidXML(t *testing.T) {
	t.Parallel()
	if _, ok := ParseNowPlaying("<DIDL-Lite><item>"); ok {
		t.Fatalf("expected not ok")
	}
}
