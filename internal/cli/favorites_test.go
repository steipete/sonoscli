package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

type fakeFavoritesClient struct {
	page sonos.FavoritesPage

	playCalls int
	lastItem  sonos.DIDLItem
}

func (f *fakeFavoritesClient) ListFavorites(ctx context.Context, start, count int) (sonos.FavoritesPage, error) {
	if count == 1 {
		if start >= 0 && start < len(f.page.Items) {
			return sonos.FavoritesPage{
				Items:          []sonos.FavoriteItem{f.page.Items[start]},
				NumberReturned: 1,
				TotalMatches:   len(f.page.Items),
			}, nil
		}
		return sonos.FavoritesPage{Items: nil, NumberReturned: 0, TotalMatches: len(f.page.Items)}, nil
	}
	return f.page, nil
}

func (f *fakeFavoritesClient) PlayFavorite(ctx context.Context, favorite sonos.DIDLItem) error {
	f.playCalls++
	f.lastItem = favorite
	return nil
}

func TestFavoritesListJSON(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second, Format: formatJSON}
	cmd := newFavoritesListCmd(flags)

	fake := &fakeFavoritesClient{
		page: sonos.FavoritesPage{
			Items: []sonos.FavoriteItem{
				{Position: 1, Item: sonos.DIDLItem{Title: "Fav 1", URI: "x://1"}},
			},
			NumberReturned: 1,
			TotalMatches:   1,
		},
	}

	orig := newFavoritesClient
	t.Cleanup(func() { newFavoritesClient = orig })
	newFavoritesClient = func(ctx context.Context, flags *rootFlags) (favoritesClient, error) {
		return fake, nil
	}

	var out captureWriter
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "\"title\": \"Fav 1\"") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestFavoritesOpenByIndex(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newFavoritesOpenCmd(flags)

	fake := &fakeFavoritesClient{
		page: sonos.FavoritesPage{
			Items: []sonos.FavoriteItem{
				{Position: 1, Item: sonos.DIDLItem{Title: "Fav 1", URI: "x://1"}},
				{Position: 2, Item: sonos.DIDLItem{Title: "Fav 2", URI: "x://2"}},
			},
			NumberReturned: 2,
			TotalMatches:   2,
		},
	}

	orig := newFavoritesClient
	t.Cleanup(func() { newFavoritesClient = orig })
	newFavoritesClient = func(ctx context.Context, flags *rootFlags) (favoritesClient, error) {
		return fake, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--index", "2"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.playCalls != 1 {
		t.Fatalf("expected playCalls=1, got %d", fake.playCalls)
	}
	if fake.lastItem.Title != "Fav 2" {
		t.Fatalf("expected Fav 2, got %q", fake.lastItem.Title)
	}
}
