package sonos

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// AppleMusicKind represents the type of Apple Music content.
type AppleMusicKind string

const (
	AppleMusicAlbum    AppleMusicKind = "album"
	AppleMusicSong     AppleMusicKind = "song"
	AppleMusicPlaylist AppleMusicKind = "playlist"
	AppleMusicStation  AppleMusicKind = "station"
)

// AppleMusicRef holds parsed Apple Music reference information.
type AppleMusicRef struct {
	Kind        AppleMusicKind
	ID          string
	Canonical   string // Original URL or constructed identifier
	ServiceNums []int  // Sonos service numbers for Apple Music
}

// Apple Music URL patterns:
// https://music.apple.com/us/album/album-name/1234567890
// https://music.apple.com/us/album/album-name/1234567890?i=9876543210 (track within album)
// https://music.apple.com/us/song/song-name/1234567890
// https://music.apple.com/us/playlist/playlist-name/pl.u-xyz
// https://music.apple.com/us/station/station-name/ra.xyz
var appleMusicURLRe = regexp.MustCompile(`(?i)music\.apple\.com/[a-z]{2}/(album|song|playlist|station)/[^/]+/([a-zA-Z0-9._-]+)(?:\?i=(\d+))?`)

// ParseAppleMusicRef parses an Apple Music URL or identifier and returns a reference.
func ParseAppleMusicRef(input string) (AppleMusicRef, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return AppleMusicRef{}, false
	}

	m := appleMusicURLRe.FindStringSubmatch(input)
	if len(m) < 3 {
		return AppleMusicRef{}, false
	}

	kind := AppleMusicKind(strings.ToLower(m[1]))
	id := m[2]

	// If there's a track ID parameter (?i=xxx), this is a song within an album
	if len(m) >= 4 && m[3] != "" {
		kind = AppleMusicSong
		id = m[3]
	}

	if !isSupportedAppleMusicKind(kind) || id == "" {
		return AppleMusicRef{}, false
	}

	return AppleMusicRef{
		Kind:      kind,
		ID:        id,
		Canonical: input,
		// Apple Music service type is 52231 (from ListAvailableServices)
		ServiceNums: []int{52231},
	}, true
}

func isSupportedAppleMusicKind(kind AppleMusicKind) bool {
	switch kind {
	case AppleMusicAlbum, AppleMusicSong, AppleMusicPlaylist, AppleMusicStation:
		return true
	default:
		return false
	}
}

// EnqueueAppleMusic enqueues Apple Music content to the speaker's queue.
// This method uses SMAPI search results which return properly formatted IDs.
func (c *Client) EnqueueAppleMusic(ctx context.Context, input string, opts EnqueueOptions) (int, error) {
	ref, ok := ParseAppleMusicRef(input)
	if !ok {
		return 0, fmt.Errorf("not an Apple Music URL: %q", input)
	}

	desiredPos := opts.Position
	if desiredPos < 0 {
		desiredPos = 0
	}

	itemClass, itemIDPrefix := appleMusicSonosMagic(ref.Kind)
	if itemClass == "" {
		return 0, fmt.Errorf("unsupported Apple Music kind: %s", ref.Kind)
	}

	title := opts.Title

	var lastErr error
	for _, serviceNum := range ref.ServiceNums {
		// Build the item ID and enqueued URI for Apple Music
		// The format follows Sonos conventions for music services
		itemID := fmt.Sprintf("%s%s", itemIDPrefix, ref.ID)
		meta := buildShareDIDL(itemID, title, itemClass, serviceNum)

		// Apple Music uses x-sonos-http: URIs via the SMAPI
		// The actual URI format depends on what SMAPI returns
		enqueuedURI := fmt.Sprintf("x-sonos-http:song%%3a%s.mp4?sid=%d&flags=8232&sn=1", ref.ID, serviceNum)

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
			_ = c.Play(ctx)
		}
		return first, nil
	}

	if lastErr == nil {
		lastErr = errors.New("enqueue failed")
	}
	return 0, lastErr
}

// appleMusicSonosMagic returns the DIDL item class and ID prefix for a given Apple Music kind.
func appleMusicSonosMagic(kind AppleMusicKind) (itemClass string, itemIDPrefix string) {
	switch kind {
	case AppleMusicAlbum:
		return "object.container.album.musicAlbum", "0004206c"
	case AppleMusicPlaylist:
		return "object.container.playlistContainer", "1006206c"
	case AppleMusicSong:
		return "object.item.audioItem.musicTrack", "10032020"
	case AppleMusicStation:
		return "object.item.audioItem.audioBroadcast", "000c206c"
	default:
		return "", ""
	}
}

// EnqueueAppleMusicFromSMAPI enqueues an Apple Music item returned from SMAPI search.
// The item parameter should have ID and Title from the SMAPI search result.
func (c *Client) EnqueueAppleMusicFromSMAPI(ctx context.Context, item SMAPIItem, serviceNum int, opts EnqueueOptions) (int, error) {
	desiredPos := opts.Position
	if desiredPos < 0 {
		desiredPos = 0
	}

	title := opts.Title
	if title == "" {
		title = item.Title
	}

	// Determine the item class and ID prefix based on the SMAPI item type
	// The prefix is required for Sonos to properly display metadata
	var itemClass, itemIDPrefix string
	switch item.ItemType {
	case "album", "container":
		itemClass = "object.container.album.musicAlbum"
		itemIDPrefix = "1004206c" // Album container prefix
	case "playlist":
		itemClass = "object.container.playlistContainer"
		itemIDPrefix = "1006206c" // Playlist container prefix
	default: // song/track
		itemClass = "object.item.audioItem.musicTrack"
		itemIDPrefix = "10032020" // Track prefix
	}

	// Build the full item ID with prefix for proper metadata display
	// The item ID needs URL-encoded colons (%3a) to match Sonos format
	encodedID := strings.ReplaceAll(item.ID, ":", "%3a")
	metaItemID := itemIDPrefix + encodedID

	// Apple Music uses service 52231 for metadata descriptors, even though
	// the URI uses sid=204 for routing. This matches observed Sonos favorites.
	const appleMusicMetadataService = 52231
	meta := buildShareDIDL(metaItemID, title, itemClass, appleMusicMetadataService)

	// Construct URI based on item type
	// Format from Sonos favorites: x-sonos-http:song%3a{ID}.mp4?sid=204&flags=8224&sn=10
	var enqueuedURI string
	if strings.HasPrefix(item.ID, "song:") {
		// Extract just the numeric ID
		songID := strings.TrimPrefix(item.ID, "song:")
		enqueuedURI = fmt.Sprintf("x-sonos-http:song%%3a%s.mp4?sid=%d&flags=8224&sn=10", songID, serviceNum)
	} else if strings.HasPrefix(item.ID, "album:") {
		// Album container format: x-rincon-cpcontainer:1004206calbum%3a{ID}?sid=204&flags=8300&sn=10
		albumID := strings.TrimPrefix(item.ID, "album:")
		enqueuedURI = fmt.Sprintf("x-rincon-cpcontainer:1004206calbum%%3a%s?sid=%d&flags=8300&sn=10", albumID, serviceNum)
	} else if strings.HasPrefix(item.ID, "playlist:") {
		// Playlist container format
		playlistID := strings.TrimPrefix(item.ID, "playlist:")
		enqueuedURI = fmt.Sprintf("x-rincon-cpcontainer:1006206cplaylist%%3a%s?sid=%d&flags=8300&sn=10", playlistID, serviceNum)
	} else {
		// Fallback to original ID
		enqueuedURI = item.ID
	}

	first, err := c.AddURIToQueue(ctx, enqueuedURI, meta, desiredPos, opts.AsNext)
	if err != nil {
		return 0, err
	}

	if opts.PlayNow && first > 0 {
		if err := c.playFromQueueTrack(ctx, first); err != nil {
			return first, err
		}
	} else if opts.PlayNow {
		_ = c.Play(ctx)
	}

	return first, nil
}
