package sonos

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type SpotifyKind string

const (
	SpotifyAlbum    SpotifyKind = "album"
	SpotifyEpisode  SpotifyKind = "episode"
	SpotifyPlaylist SpotifyKind = "playlist"
	SpotifyShow     SpotifyKind = "show"
	SpotifyTrack    SpotifyKind = "track"
)

type SpotifyRef struct {
	Kind        SpotifyKind
	ID          string
	Canonical   string // spotify:<kind>:<id>
	EncodedID   string // spotify%3a<kind>%3a<id>
	ServiceNums []int  // possible Sonos service numbers
}

var spotifyRefRe = regexp.MustCompile(`(?i)spotify.*[:/](album|episode|playlist|show|track)[:/](\w+)`)

func ParseSpotifyRef(input string) (SpotifyRef, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return SpotifyRef{}, false
	}

	// Accept canonical spotify:<kind>:<id> inputs.
	if strings.HasPrefix(strings.ToLower(input), "spotify:") {
		parts := strings.Split(input, ":")
		if len(parts) >= 3 {
			kind := SpotifyKind(strings.ToLower(parts[1]))
			id := parts[2]
			if isSupportedSpotifyKind(kind) && id != "" {
				c := "spotify:" + string(kind) + ":" + id
				return SpotifyRef{
					Kind:        kind,
					ID:          id,
					Canonical:   c,
					EncodedID:   encodeSpotifyID(c),
					ServiceNums: []int{2311, 3079},
				}, true
			}
		}
	}

	m := spotifyRefRe.FindStringSubmatch(input)
	if len(m) != 3 {
		return SpotifyRef{}, false
	}
	kind := SpotifyKind(strings.ToLower(m[1]))
	id := m[2]
	if !isSupportedSpotifyKind(kind) || id == "" {
		return SpotifyRef{}, false
	}
	c := "spotify:" + string(kind) + ":" + id
	return SpotifyRef{
		Kind:        kind,
		ID:          id,
		Canonical:   c,
		EncodedID:   encodeSpotifyID(c),
		ServiceNums: []int{2311, 3079},
	}, true
}

func isSupportedSpotifyKind(kind SpotifyKind) bool {
	switch kind {
	case SpotifyAlbum, SpotifyEpisode, SpotifyPlaylist, SpotifyShow, SpotifyTrack:
		return true
	default:
		return false
	}
}

func encodeSpotifyID(canonical string) string {
	// Sonos share-link payloads typically expect ':' -> '%3a' (lowercase).
	return strings.ReplaceAll(canonical, ":", "%3a")
}

type EnqueueOptions struct {
	Title    string
	AsNext   bool
	PlayNow  bool
	Position int // 0 = append
}

func (c *Client) EnqueueSpotify(ctx context.Context, input string, opts EnqueueOptions) (int, error) {
	ref, ok := ParseSpotifyRef(input)
	if !ok {
		return 0, fmt.Errorf("not a Spotify URI/link: %q", input)
	}

	desiredPos := opts.Position
	if desiredPos < 0 {
		desiredPos = 0
	}

	itemClass, itemIDKey, uriPrefixes := spotifySonosMagic(ref.Kind)
	if itemClass == "" {
		return 0, fmt.Errorf("unsupported Spotify kind: %s", ref.Kind)
	}

	title := opts.Title
	meta := buildShareDIDL(itemIDKey+ref.EncodedID, title, itemClass, ref.ServiceNums[0])

	var lastErr error
	for _, serviceNum := range ref.ServiceNums {
		meta = buildShareDIDL(itemIDKey+ref.EncodedID, title, itemClass, serviceNum)
		for _, prefix := range uriPrefixes {
			enqueuedURI := prefix + ref.EncodedID
			first, err := c.AddURIToQueue(ctx, enqueuedURI, meta, desiredPos, opts.AsNext)
			if err != nil {
				lastErr = err
				continue
			}
			if opts.PlayNow && first > 0 {
				if err := c.playFromQueueTrack(ctx, first); err != nil {
					return first, err
				}
			} else if opts.PlayNow {
				// If the speaker doesn't report a first track number, just call Play.
				_ = c.Play(ctx)
			}
			return first, nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New("enqueue failed")
	}
	return 0, lastErr
}

func spotifySonosMagic(kind SpotifyKind) (itemClass string, itemIDKey string, uriPrefixes []string) {
	switch kind {
	case SpotifyAlbum:
		return "object.container.album.musicAlbum", "00040000", []string{"x-rincon-cpcontainer:1004206c"}
	case SpotifyPlaylist, SpotifyShow:
		return "object.container.playlistContainer", "1006206c", []string{"x-rincon-cpcontainer:1006206c"}
	case SpotifyTrack, SpotifyEpisode:
		// Try a few URI forms. Sonos varies by firmware/service config.
		return "object.item.audioItem.musicTrack", "00032020", []string{"x-sonos-spotify:", ""}
	default:
		return "", "", nil
	}
}

func buildShareDIDL(itemID, title, itemClass string, serviceNum int) string {
	if title == "" {
		title = ""
	}
	// Mirrors the SoCo ShareLink plugin template.
	// desc: SA_RINCON{sn}_X_#Svc{sn}-0-Token
	desc := fmt.Sprintf("SA_RINCON%d_X_#Svc%d-0-Token", serviceNum, serviceNum)

	return fmt.Sprintf(
		`<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="%s" parentID="-1" restricted="true"><dc:title>%s</dc:title><upnp:class>%s</upnp:class><desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">%s</desc></item></DIDL-Lite>`,
		xmlEscapeText(itemID),
		xmlEscapeText(title),
		xmlEscapeText(itemClass),
		xmlEscapeText(desc),
	)
}

func (c *Client) playFromQueueTrack(ctx context.Context, oneBasedTrackNumber int) error {
	dd, err := c.GetDeviceDescription(ctx)
	if err != nil {
		return err
	}
	if dd.UDN == "" {
		return errors.New("missing device UDN")
	}
	queueURI := "x-rincon-queue:" + dd.UDN + "#0"
	if err := c.SetAVTransportURI(ctx, queueURI, ""); err != nil {
		return err
	}
	if err := c.SeekTrackNumber(ctx, oneBasedTrackNumber); err != nil {
		return err
	}
	return c.Play(ctx)
}

func (c *Client) SetGroupVolume(ctx context.Context, volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	// SnapshotGroupVolume first (Sonos convention).
	_, _ = c.soapCall(ctx, controlGroupRendering, urnGroupRenderingControl, "SnapshotGroupVolume", map[string]string{
		"InstanceID": "0",
	})
	_, err := c.soapCall(ctx, controlGroupRendering, urnGroupRenderingControl, "SetGroupVolume", map[string]string{
		"InstanceID":    "0",
		"DesiredVolume": strconv.Itoa(volume),
	})
	return err
}
