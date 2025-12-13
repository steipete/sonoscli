package sonos

import (
	"context"
	"fmt"
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
	return c.PlayURI(ctx, uri, favorite.ResMD)
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
