package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/applemusic"
	"github.com/steipete/sonoscli/internal/sonos"
)

// appleMusicEnqueuer interface for dependency injection in tests.
type appleMusicEnqueuer interface {
	EnqueueAppleMusicFromSMAPI(ctx context.Context, item sonos.SMAPIItem, serviceNum int, opts sonos.EnqueueOptions) (int, error)
	CoordinatorIP() string
}

type realAppleMusicEnqueuer struct{ c *sonos.Client }

func (r realAppleMusicEnqueuer) EnqueueAppleMusicFromSMAPI(ctx context.Context, item sonos.SMAPIItem, serviceNum int, opts sonos.EnqueueOptions) (int, error) {
	return r.c.EnqueueAppleMusicFromSMAPI(ctx, item, serviceNum, opts)
}

func (r realAppleMusicEnqueuer) CoordinatorIP() string { return r.c.IP }

var newAppleMusicEnqueuer = func(ctx context.Context, flags *rootFlags) (appleMusicEnqueuer, error) {
	c, err := coordinatorClient(ctx, flags)
	if err != nil {
		return nil, err
	}
	return realAppleMusicEnqueuer{c: c}, nil
}

var newAppleMusicTokenStore = func() (applemusic.TokenStore, error) {
	return applemusic.NewDefaultTokenStore()
}

func newPlayAppleMusicCmd(flags *rootFlags) *cobra.Command {
	var category string
	var index int
	var enqueueOnly bool
	var titleOverride string

	cmd := &cobra.Command{
		Use:   `applemusic <query>`,
		Short: "Search Apple Music and play the top result",
		Long: `Searches Apple Music using your authenticated account, then enqueues and plays on the target Sonos speaker.

Requires authentication: run 'sonos auth applemusic login' first.
Also requires Apple Music to be linked to your Sonos system via the Sonos app.

Examples:
  sonos play applemusic "taylor swift" --name "Living Room"
  sonos play applemusic --category albums "abbey road"
  sonos play applemusic --category playlists "workout"`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := validateTarget(flags); err != nil {
				return err
			}

			query := strings.TrimSpace(args[0])
			if query == "" {
				return errors.New("query is required")
			}
			if index < 0 {
				return errors.New("--index must be >= 0")
			}

			// Load Apple Music token
			store, err := newAppleMusicTokenStore()
			if err != nil {
				return err
			}

			token, ok, err := store.Load()
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("not authenticated with Apple Music. Run 'sonos auth applemusic login' first")
			}
			if !token.IsValid() {
				return errors.New("Apple Music token expired. Run 'sonos auth applemusic login' to re-authenticate")
			}

			// Create Apple Music client and search
			client := applemusic.NewClient(token)

			// Map category to Apple Music search types
			searchTypes := categoryToSearchTypes(category)

			result, err := client.Search(ctx, query, applemusic.SearchOptions{
				Types: searchTypes,
				Limit: 10,
			})
			if err != nil {
				return fmt.Errorf("apple music search failed: %w", err)
			}

			// Extract results based on category
			items, itemType := extractSearchItems(result, category)
			if len(items) == 0 {
				return errors.New("no results found")
			}
			if index >= len(items) {
				return fmt.Errorf("--index %d out of range (results=%d)", index, len(items))
			}

			selected := items[index]

			title := strings.TrimSpace(titleOverride)
			if title == "" {
				title = selected.Title
			}

			// Get Sonos coordinator and enqueue
			enq, err := newAppleMusicEnqueuer(ctx, flags)
			if err != nil {
				return err
			}

			// Build SMAPI item from Apple Music result
			smapiItem := sonos.SMAPIItem{
				ID:       fmt.Sprintf("%s:%s", itemType, selected.ID),
				ItemType: itemType,
				Title:    title,
			}

			// Apple Music service number from Sonos (sid=204 based on favorites)
			const appleMusicServiceNum = 204

			pos, err := enq.EnqueueAppleMusicFromSMAPI(ctx, smapiItem, appleMusicServiceNum, sonos.EnqueueOptions{
				Title:   title,
				PlayNow: !enqueueOnly,
			})
			if err != nil {
				return err
			}

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"source":        "applemusic-api",
					"coordinatorIP": enq.CoordinatorIP(),
					"category":      category,
					"query":         query,
					"index":         index,
					"selected":      selected,
					"enqueuedPos":   pos,
					"enqueueOnly":   enqueueOnly,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Playing: %s\n", title)
			if selected.Artist != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Artist: %s\n", selected.Artist)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&category, "category", "songs", "Search category: songs, albums, playlists, artists")
	cmd.Flags().IntVar(&index, "index", 0, "Result index to play (0-based)")
	cmd.Flags().BoolVar(&enqueueOnly, "enqueue", false, "Only enqueue (do not start playback)")
	cmd.Flags().StringVar(&titleOverride, "title", "", "Optional title override for the queued item")

	cmd.Flags().SortFlags = true

	return cmd
}

// searchItem is a unified representation of a search result.
type searchItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist,omitempty"`
	Album  string `json:"album,omitempty"`
	Type   string `json:"type"`
	URL    string `json:"url,omitempty"`
}

func categoryToSearchTypes(category string) []string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "songs", "tracks", "song", "track":
		return []string{"songs"}
	case "albums", "album":
		return []string{"albums"}
	case "playlists", "playlist":
		return []string{"playlists"}
	case "artists", "artist":
		return []string{"artists"}
	default:
		return []string{"songs"}
	}
}

func extractSearchItems(result *applemusic.SearchResult, category string) ([]searchItem, string) {
	var items []searchItem
	var itemType string

	switch strings.ToLower(strings.TrimSpace(category)) {
	case "songs", "tracks", "song", "track":
		itemType = "song"
		if result.Results.Songs != nil {
			for _, s := range result.Results.Songs.Data {
				items = append(items, searchItem{
					ID:     s.ID,
					Title:  s.Attributes.Name,
					Artist: s.Attributes.ArtistName,
					Album:  s.Attributes.AlbumName,
					Type:   "song",
					URL:    s.Attributes.URL,
				})
			}
		}
	case "albums", "album":
		itemType = "album"
		if result.Results.Albums != nil {
			for _, a := range result.Results.Albums.Data {
				items = append(items, searchItem{
					ID:     a.ID,
					Title:  a.Attributes.Name,
					Artist: a.Attributes.ArtistName,
					Type:   "album",
					URL:    a.Attributes.URL,
				})
			}
		}
	case "playlists", "playlist":
		itemType = "playlist"
		if result.Results.Playlists != nil {
			for _, p := range result.Results.Playlists.Data {
				items = append(items, searchItem{
					ID:     p.ID,
					Title:  p.Attributes.Name,
					Artist: p.Attributes.CuratorName,
					Type:   "playlist",
					URL:    p.Attributes.URL,
				})
			}
		}
	case "artists", "artist":
		itemType = "artist"
		if result.Results.Artists != nil {
			for _, a := range result.Results.Artists.Data {
				items = append(items, searchItem{
					ID:    a.ID,
					Title: a.Attributes.Name,
					Type:  "artist",
					URL:   a.Attributes.URL,
				})
			}
		}
	default:
		// Default to songs
		itemType = "song"
		if result.Results.Songs != nil {
			for _, s := range result.Results.Songs.Data {
				items = append(items, searchItem{
					ID:     s.ID,
					Title:  s.Attributes.Name,
					Artist: s.Attributes.ArtistName,
					Album:  s.Attributes.AlbumName,
					Type:   "song",
					URL:    s.Attributes.URL,
				})
			}
		}
	}

	return items, itemType
}
