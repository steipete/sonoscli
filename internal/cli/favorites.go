package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type favoritesClient interface {
	ListFavorites(ctx context.Context, start, count int) (sonos.FavoritesPage, error)
	PlayFavorite(ctx context.Context, favorite sonos.DIDLItem) error
}

var newFavoritesClient = func(ctx context.Context, flags *rootFlags) (favoritesClient, error) {
	return coordinatorClient(ctx, flags)
}

func newFavoritesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "Browse and play Sonos Favorites",
		Long:  "Lists and plays Sonos Favorites (ContentDirectory FV:2).",
	}
	cmd.AddCommand(newFavoritesListCmd(flags))
	cmd.AddCommand(newFavoritesOpenCmd(flags))
	return cmd
}

func newFavoritesListCmd(flags *rootFlags) *cobra.Command {
	var start int
	var limit int

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List Sonos Favorites",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			c, err := newFavoritesClient(cmd.Context(), flags)
			if err != nil {
				return err
			}
			page, err := c.ListFavorites(cmd.Context(), start, limit)
			if err != nil {
				return err
			}
			if isJSON(flags) {
				return writeJSON(cmd, page)
			}
			if isTSV(flags) {
				for _, it := range page.Items {
					title := it.Item.Title
					if title == "" {
						title = it.Item.ID
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", it.Position, title, it.Item.URI)
				}
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "POS\tTITLE\tURI\n")
			for _, it := range page.Items {
				title := it.Item.Title
				if title == "" {
					title = it.Item.ID
				}
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", it.Position, title, it.Item.URI)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&start, "start", 0, "Starting index (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results to return")
	return cmd
}

func newFavoritesOpenCmd(flags *rootFlags) *cobra.Command {
	var index int

	cmd := &cobra.Command{
		Use:          "open [title]",
		Short:        "Play a Sonos Favorite by title or index",
		Long:         "Plays a Sonos Favorite by exact title match (case-insensitive) or by 1-based index from `sonos favorites list`.",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			var title string
			if len(args) == 1 {
				title = strings.TrimSpace(args[0])
			}
			if index <= 0 && title == "" {
				return errors.New("provide --index or a title")
			}

			c, err := newFavoritesClient(cmd.Context(), flags)
			if err != nil {
				return err
			}

			if index > 0 {
				page, err := c.ListFavorites(cmd.Context(), index-1, 1)
				if err != nil {
					return err
				}
				if len(page.Items) == 0 {
					return errors.New("favorite index out of range: " + strconv.Itoa(index))
				}
				if err := c.PlayFavorite(cmd.Context(), page.Items[0].Item); err != nil {
					return err
				}
				return writeOK(cmd, flags, "favorites.open", map[string]any{"favorite": page.Items[0]})
			}

			// Search pages until we find a matching title.
			const pageSize = 100
			start := 0
			for {
				page, err := c.ListFavorites(cmd.Context(), start, pageSize)
				if err != nil {
					return err
				}
				for _, it := range page.Items {
					if strings.EqualFold(it.Item.Title, title) {
						if err := c.PlayFavorite(cmd.Context(), it.Item); err != nil {
							return err
						}
						return writeOK(cmd, flags, "favorites.open", map[string]any{"favorite": it})
					}
				}
				start += page.NumberReturned
				if page.NumberReturned == 0 || start >= page.TotalMatches {
					break
				}
			}

			return errors.New("favorite not found: " + title)
		},
	}

	cmd.Flags().IntVar(&index, "index", 0, "1-based favorite index from `sonos favorites list`")
	return cmd
}
