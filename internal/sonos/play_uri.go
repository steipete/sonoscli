package sonos

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ForceRadioURI converts an http/https/aac URI to the Sonos "mp3radio" scheme to
// force radio-style playback controls in the Sonos UI.
func ForceRadioURI(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	colon := strings.IndexByte(uri, ':')
	if colon <= 0 {
		return uri
	}
	return "x-rincon-mp3radio" + uri[colon:]
}

// BuildRadioMeta builds minimal DIDL metadata suitable for playing radio streams.
func BuildRadioMeta(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	// TuneIn service descriptor.
	const tuneInService = "SA_RINCON65031_"
	return `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">` +
		`<item id="R:0/0/0" parentID="R:0/0" restricted="true">` +
		`<dc:title>` + xmlEscapeText(title) + `</dc:title>` +
		`<upnp:class>object.item.audioItem.audioBroadcast</upnp:class>` +
		`<desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">` + tuneInService + `</desc>` +
		`</item></DIDL-Lite>`
}

// BuildStreamProxyMeta builds DIDL metadata for sonoscli-managed proxy streams.
// Sonos tends to treat x-rincon-mp3radio streams as station-like sources; the
// stream's ICY metadata carries the currently playing title.
func BuildStreamProxyMeta(title, provider string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Sonos CLI"
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "Sonos CLI"
	}
	return `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">` +
		`<item id="sonoscli:stream" parentID="sonoscli" restricted="true">` +
		`<dc:title>` + xmlEscapeText(title) + `</dc:title>` +
		`<dc:creator>` + xmlEscapeText(provider) + `</dc:creator>` +
		`<upnp:class>object.item.audioItem.audioBroadcast</upnp:class>` +
		`<desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">Sonos CLI</desc>` +
		`</item></DIDL-Lite>`
}

// BuildStreamProxyTrackMeta builds DIDL metadata for a single sonoscli-managed
// proxy track in a queue context. Unlike BuildStreamProxyMeta this uses
// audioItem.musicTrack so Sonos treats it as a finite track and advances to
// the next queue entry on completion. The duration (HH:MM:SS.mmm) is written
// into the <res> element when non-zero so Sonos schedules the advance even
// when the HTTP body has no Content-Length.
func BuildStreamProxyTrackMeta(title, provider, uri string, duration time.Duration) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Sonos CLI"
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "Sonos CLI"
	}
	res := `<res protocolInfo="http-get:*:audio/mpeg:*"`
	if duration > 0 {
		res += ` duration="` + xmlEscapeText(formatDIDLDuration(duration)) + `"`
	}
	res += `>` + xmlEscapeText(uri) + `</res>`

	return `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">` +
		`<item id="sonoscli:track" parentID="sonoscli" restricted="true">` +
		`<dc:title>` + xmlEscapeText(title) + `</dc:title>` +
		`<dc:creator>` + xmlEscapeText(provider) + `</dc:creator>` +
		`<upnp:class>object.item.audioItem.musicTrack</upnp:class>` +
		res +
		`<desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">Sonos CLI</desc>` +
		`</item></DIDL-Lite>`
}

func formatDIDLDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Millisecond)
	ms := total % 1000
	totalSec := total / 1000
	s := totalSec % 60
	totalMin := totalSec / 60
	m := totalMin % 60
	h := totalMin / 60
	return fmt.Sprintf("%d:%02d:%02d.%03d", h, m, s, ms)
}

func (c *Client) PlayURI(ctx context.Context, uri, meta string) error {
	if err := c.SetAVTransportURI(ctx, uri, meta); err != nil {
		return err
	}
	return c.Play(ctx)
}
