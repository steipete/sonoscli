package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
	"github.com/steipete/sonoscli/internal/spotify"
)

type spotifySearcher interface {
	Search(ctx context.Context, query string, typ spotify.SearchType, limit int, market string) ([]spotify.Result, error)
}

type sonosEnqueuer interface {
	EnqueueSpotify(ctx context.Context, input string, opts sonos.EnqueueOptions) (int, error)
}

var newSpotifySearcher = func(flags *rootFlags, clientID, clientSecret string) (spotifySearcher, error) {
	if strings.TrimSpace(clientID) != "" && strings.TrimSpace(clientSecret) != "" {
		return spotify.New(strings.TrimSpace(clientID), strings.TrimSpace(clientSecret), nil), nil
	}
	return spotify.NewFromEnv(nil)
}

var newSonosEnqueuer = func(ctx context.Context, flags *rootFlags) (sonosEnqueuer, error) {
	return coordinatorClient(ctx, flags)
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for music (Spotify, Apple Music)",
	}
	cmd.AddCommand(newSearchSpotifyCmd(flags))
	cmd.AddCommand(newSearchAppleMusicCmd(flags))
	return cmd
}

func newSearchSpotifyCmd(flags *rootFlags) *cobra.Command {
	var (
		typ          string
		limit        int
		market       string
		clientID     string
		clientSecret string
		doOpen       bool
		doEnqueue    bool
		index        int
	)

	cmd := &cobra.Command{
		Use:   "spotify <query>",
		Short: "Search Spotify and print playable URIs",
		Long: "Searches Spotify via the Spotify Web API (client credentials). " +
			"Requires SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET (or --client-id/--client-secret). " +
			"Prints Spotify URIs you can pass to `sonos open` / `sonos enqueue`.",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if doOpen && doEnqueue {
				return errors.New("use only one of --open or --enqueue")
			}
			if (doOpen || doEnqueue) && flags.IP == "" && flags.Name == "" {
				return errors.New("--open/--enqueue require --ip or --name")
			}
			if index <= 0 {
				index = 1
			}

			query := strings.TrimSpace(strings.Join(args, " "))
			st, err := spotify.ParseSearchType(typ)
			if err != nil {
				return err
			}

			sp, err := newSpotifySearcher(flags, clientID, clientSecret)
			if err != nil {
				return err
			}

			results, err := sp.Search(cmd.Context(), query, st, limit, market)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				return errors.New("no results")
			}

			if doOpen || doEnqueue {
				if index > len(results) {
					return fmt.Errorf("--index %d out of range (got %d results)", index, len(results))
				}
				selected := results[index-1]
				ref := selected.URI

				c, err := newSonosEnqueuer(cmd.Context(), flags)
				if err != nil {
					return err
				}
				_, ok := sonos.ParseSpotifyRef(ref)
				if !ok {
					return errors.New("selected result is not a supported Spotify ref: " + ref)
				}

				_, err = c.EnqueueSpotify(cmd.Context(), ref, sonos.EnqueueOptions{
					PlayNow: doOpen,
				})
				if err != nil {
					return err
				}
			}

			if isJSON(flags) {
				if doOpen || doEnqueue {
					selected := results[index-1]
					return writeJSON(cmd, map[string]any{
						"query":    query,
						"type":     st,
						"results":  results,
						"selected": selected,
						"action": map[string]any{
							"enqueue": true,
							"playNow": doOpen,
						},
					})
				}
				return writeJSON(cmd, results)
			}

			if isTSV(flags) {
				for i, r := range results {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\t%s\n", i+1, r.Type, r.Title, r.Subtitle, r.URI)
				}
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "INDEX\tTYPE\tTITLE\tDETAILS\tURI")
			for i, r := range results {
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, r.Type, r.Title, r.Subtitle, r.URI)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&typ, "type", "track", "Result type: track|album|playlist|show|episode")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results (1-50)")
	cmd.Flags().StringVar(&market, "market", "", "Optional market (e.g. US); leave empty for global catalog")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Spotify Web API client id (or set SPOTIFY_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Spotify Web API client secret (or set SPOTIFY_CLIENT_SECRET)")
	cmd.Flags().BoolVar(&doOpen, "open", false, "Open the selected result on Sonos (requires --name/--ip)")
	cmd.Flags().BoolVar(&doEnqueue, "enqueue", false, "Enqueue the selected result on Sonos (requires --name/--ip)")
	cmd.Flags().IntVar(&index, "index", 1, "Which search result to use with --open/--enqueue (1-based)")

	return cmd
}
