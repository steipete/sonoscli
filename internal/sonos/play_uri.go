package sonos

import (
	"context"
	"strings"
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

func (c *Client) PlayURI(ctx context.Context, uri, meta string) error {
	if err := c.SetAVTransportURI(ctx, uri, meta); err != nil {
		return err
	}
	return c.Play(ctx)
}
