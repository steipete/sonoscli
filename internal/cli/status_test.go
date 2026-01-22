package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

type fakeStatusClient struct {
	dev       sonos.Device
	transport sonos.TransportInfo
	position  sonos.PositionInfo
	volume    int
	mute      bool
}

func (f *fakeStatusClient) GetDeviceDescription(ctx context.Context) (sonos.Device, error) {
	return f.dev, nil
}

func (f *fakeStatusClient) GetTransportInfo(ctx context.Context) (sonos.TransportInfo, error) {
	return f.transport, nil
}

func (f *fakeStatusClient) GetPositionInfo(ctx context.Context) (sonos.PositionInfo, error) {
	return f.position, nil
}

func (f *fakeStatusClient) GetVolume(ctx context.Context) (int, error) {
	return f.volume, nil
}

func (f *fakeStatusClient) GetMute(ctx context.Context) (bool, error) {
	return f.mute, nil
}

func TestStatusShowsNowPlayingFields(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}

	didl := `<?xml version="1.0" encoding="utf-8"?>
<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"
  xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
  <item id="R:0/0/0" parentID="R:0/0" restricted="true">
    <dc:title>Gareth Emery</dc:title>
    <upnp:artist>Gareth Emery</upnp:artist>
    <upnp:album>Some Album</upnp:album>
    <upnp:albumArtURI>/getaa?s=1&amp;u=abc</upnp:albumArtURI>
    <res>https://example.com/stream.mp3</res>
  </item>
</DIDL-Lite>`

	fake := &fakeStatusClient{
		dev: sonos.Device{Name: "Office", IP: "192.168.1.50", UDN: "RINCON_OFFICE1400"},
		transport: sonos.TransportInfo{
			State: "PLAYING",
		},
		position: sonos.PositionInfo{
			Track:         "1",
			TrackURI:      "x-sonos-spotify:spotify%3atrack%3a123?sid=2311&sn=0",
			TrackMeta:     didl,
			RelTime:       "0:00:10",
			TrackDuration: "0:03:00",
		},
		volume: 25,
		mute:   false,
	}

	orig := newStatusClient
	t.Cleanup(func() { newStatusClient = orig })
	newStatusClient = func(ctx context.Context, flags *rootFlags) (statusClient, error) {
		return fake, nil
	}

	cmd := newStatusCmd(flags)
	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "Title:\t\tGareth Emery") {
		t.Fatalf("missing title: %s", s)
	}
	if !strings.Contains(s, "Artist:\t\tGareth Emery") {
		t.Fatalf("missing artist: %s", s)
	}
	if !strings.Contains(s, "Album:\t\tSome Album") {
		t.Fatalf("missing album: %s", s)
	}
	if !strings.Contains(s, "AlbumArt:\thttp://192.168.1.50:1400/getaa?s=1&u=abc") {
		t.Fatalf("missing album art url: %s", s)
	}
}

func TestStatusJSONIncludesNowPlaying(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second, Format: formatJSON}

	didl := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
  <item id="R:0/0/0" parentID="R:0/0" restricted="true">
    <dc:title>My Song</dc:title>
    <res>https://example.com/stream.mp3</res>
    <albumArtURI>/getaa?s=1&amp;u=abc</albumArtURI>
  </item>
</DIDL-Lite>`

	fake := &fakeStatusClient{
		dev:       sonos.Device{Name: "Office", IP: "192.168.1.50"},
		transport: sonos.TransportInfo{State: "PLAYING"},
		position:  sonos.PositionInfo{TrackMeta: didl},
		volume:    10,
		mute:      false,
	}

	orig := newStatusClient
	t.Cleanup(func() { newStatusClient = orig })
	newStatusClient = func(ctx context.Context, flags *rootFlags) (statusClient, error) {
		return fake, nil
	}

	cmd := newStatusCmd(flags)
	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "\"nowPlaying\"") {
		t.Fatalf("missing nowPlaying: %s", s)
	}
	if !strings.Contains(s, "\"title\": \"My Song\"") {
		t.Fatalf("missing title: %s", s)
	}
	// encoding/json escapes '&' as "\u0026"
	if !strings.Contains(s, "\"albumArtURL\": \"http://192.168.1.50:1400/getaa?s=1") || !strings.Contains(s, "u=abc") {
		t.Fatalf("missing albumArtURL: %s", s)
	}
}
