package sonos

import (
	"context"
	"fmt"
	"strings"
)

type FavoriteItem struct {
	Position int      `json:"position"` // 1-based
	Item     DIDLItem `json:"item"`
}

type FavoritesPage struct {
	Items          []FavoriteItem `json:"items"`
	NumberReturned int            `json:"numberReturned"`
	TotalMatches   int            `json:"totalMatches"`
	UpdateID       int            `json:"updateID"`
}

func (c *Client) ListFavorites(ctx context.Context, start, count int) (FavoritesPage, error) {
	if start < 0 {
		start = 0
	}
	if count <= 0 {
		count = 100
	}
	br, err := c.Browse(ctx, "FV:2", start, count)
	if err != nil {
		return FavoritesPage{}, err
	}
	didlItems, err := ParseDIDLItems(br.Result)
	if err != nil {
		return FavoritesPage{}, err
	}
	items := make([]FavoriteItem, 0, len(didlItems))
	for i, it := range didlItems {
		items = append(items, FavoriteItem{
			Position: start + i + 1,
			Item:     it,
		})
	}
	return FavoritesPage{
		Items:          items,
		NumberReturned: br.NumberReturned,
		TotalMatches:   br.TotalMatches,
		UpdateID:       br.UpdateID,
	}, nil
}

func (c *Client) PlayFavorite(ctx context.Context, favorite DIDLItem) error {
	uri := favoriteURI(favorite)
	if uri == "" {
		return fmt.Errorf("favorite has no URI")
	}
	// Container favorites (service-side albums, playlists, browsable items)
	// cannot be set as AVTransport URI directly — Sonos rejects them with
	// UPnP 714. Clear the queue, append the container's tracks, then play from
	// the first new track. This matches the Sonos app's behavior when you open
	// an album/playlist favorite. Single-track stream URIs (TuneIn radio, Sonos
	// Radio, etc.) are played directly via SetAVTransportURI; the queue is
	// left untouched, also matching the app's behavior for radio favorites.
	if isContainerFavoriteURI(uri) {
		if err := c.RemoveAllTracksFromQueue(ctx); err != nil {
			return err
		}
		first, err := c.AddURIToQueue(ctx, uri, favorite.ResMD, 0, false)
		if err != nil {
			return err
		}
		if first <= 0 {
			// Some firmware variants may omit FirstTrackNumberEnqueued. Since
			// we just cleared the queue, the first appended track is at 1.
			first = 1
		}
		return c.playFromQueueTrack(ctx, first)
	}
	return c.PlayURI(ctx, uri, favorite.ResMD)
}

// isContainerFavoriteURI reports whether the URI scheme designates a
// browsable container (service-side album, playlist, or browsable item) that
// must be enqueued rather than set directly as the AVTransport URI.
func isContainerFavoriteURI(uri string) bool {
	return strings.HasPrefix(uri, "x-rincon-cpcontainer:")
}

func favoriteURI(favorite DIDLItem) string {
	if favorite.URI != "" {
		return favorite.URI
	}
	if favorite.ResMD == "" {
		return ""
	}
	items, err := ParseDIDLItems(favorite.ResMD)
	if err != nil || len(items) == 0 {
		return ""
	}
	return items[0].URI
}
