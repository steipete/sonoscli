package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/applemusic"
)

func newSearchAppleMusicCmd(flags *rootFlags) *cobra.Command {
	var (
		category string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "applemusic <query>",
		Short: "Search Apple Music catalog",
		Long: `Searches Apple Music using your authenticated account.

Requires authentication: run 'sonos auth applemusic login' first.

Examples:
  sonos search applemusic "flying lotus"
  sonos search applemusic "chill vibes" --category playlists
  sonos search applemusic "miles davis" --category albums`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return errors.New("query is required")
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

			// Map category to search types
			searchTypes := categoryToSearchTypes(category)

			result, err := client.Search(cmd.Context(), query, applemusic.SearchOptions{
				Types: searchTypes,
				Limit: limit,
			})
			if err != nil {
				return fmt.Errorf("apple music search failed: %w", err)
			}

			// Extract results based on category
			items, _ := extractSearchItems(result, category)
			if len(items) == 0 {
				return errors.New("no results found")
			}

			// Output results
			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"query":    query,
					"category": category,
					"count":    len(items),
					"results":  items,
				})
			}

			if isTSV(flags) {
				for i, item := range items {
					artist := item.Artist
					if artist == "" {
						artist = "-"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\t%s\n",
						i, item.Type, item.Title, artist, item.ID)
				}
				return nil
			}

			// Plain text table output
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "INDEX\tTYPE\tTITLE\tARTIST\tID")
			for i, item := range items {
				artist := item.Artist
				if artist == "" {
					artist = "-"
				}
				// Truncate long titles for display
				title := item.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
					i, item.Type, title, artist, item.ID)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&category, "category", "songs", "Search category: songs, albums, playlists, artists")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results (1-25)")

	return cmd
}
