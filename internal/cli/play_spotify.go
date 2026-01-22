package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type spotifyEnqueuer interface {
	EnqueueSpotify(ctx context.Context, input string, opts sonos.EnqueueOptions) (int, error)
	CoordinatorIP() string
}

type realSpotifyEnqueuer struct{ c *sonos.Client }

func (r realSpotifyEnqueuer) EnqueueSpotify(ctx context.Context, input string, opts sonos.EnqueueOptions) (int, error) {
	return r.c.EnqueueSpotify(ctx, input, opts)
}

func (r realSpotifyEnqueuer) CoordinatorIP() string { return r.c.IP }

type smapiSearcher interface {
	Search(ctx context.Context, category, term string, index, count int) (sonos.SMAPISearchResult, error)
}

var newSpotifyEnqueuer = func(ctx context.Context, flags *rootFlags) (spotifyEnqueuer, error) {
	c, err := coordinatorClient(ctx, flags)
	if err != nil {
		return nil, err
	}
	return realSpotifyEnqueuer{c: c}, nil
}

var newSMAPISearcher = func(ctx context.Context, flags *rootFlags, serviceName string) (smapiSearcher, sonos.MusicServiceDescriptor, *sonos.Client, error) {
	speaker, err := anySpeakerClient(ctx, flags)
	if err != nil {
		return nil, sonos.MusicServiceDescriptor{}, nil, err
	}
	services, err := speaker.ListAvailableServices(ctx)
	if err != nil {
		return nil, sonos.MusicServiceDescriptor{}, nil, err
	}
	svc, err := findServiceByName(services, serviceName)
	if err != nil {
		return nil, sonos.MusicServiceDescriptor{}, nil, err
	}
	store, err := newSMAPITokenStore()
	if err != nil {
		return nil, sonos.MusicServiceDescriptor{}, nil, err
	}
	sm, err := sonos.NewSMAPIClient(ctx, speaker, svc, store)
	if err != nil {
		return nil, sonos.MusicServiceDescriptor{}, nil, err
	}
	return sm, svc, speaker, nil
}

func newPlaySpotifyCmd(flags *rootFlags) *cobra.Command {
	var serviceName string
	var category string
	var index int
	var enqueueOnly bool
	var titleOverride string

	cmd := &cobra.Command{
		Use:          `spotify <query>`,
		Short:        "Search Spotify via Sonos and play the top result",
		Long:         "Uses Sonos SMAPI search (no Spotify Web API credentials) to find Spotify content, then enqueues and plays it on the target room's coordinator.",
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
			if strings.TrimSpace(category) == "" {
				category = "tracks"
			}
			if index < 0 {
				return errors.New("--index must be >= 0")
			}

			enq, err := newSpotifyEnqueuer(ctx, flags)
			if err != nil {
				return err
			}

			searcher, svc, speaker, err := newSMAPISearcher(ctx, flags, serviceName)
			if err != nil {
				return err
			}

			res, err := searcher.Search(ctx, category, query, 0, 25)
			if err != nil {
				return err
			}

			items := append([]sonos.SMAPIItem{}, res.MediaMetadata...)
			items = append(items, res.MediaCollection...)
			if len(items) == 0 {
				return errors.New("no results")
			}
			if index >= len(items) {
				return fmt.Errorf("--index %d out of range (results=%d)", index, len(items))
			}

			item := items[index]
			ref := strings.TrimSpace(item.ID)
			if _, ok := sonos.ParseSpotifyRef(ref); !ok {
				return fmt.Errorf("result is not a playable Spotify ref: %q", ref)
			}

			title := strings.TrimSpace(titleOverride)
			if title == "" {
				title = strings.TrimSpace(item.Title)
			}
			pos, err := enq.EnqueueSpotify(ctx, ref, sonos.EnqueueOptions{
				Title:   title,
				PlayNow: !enqueueOnly,
			})
			if err != nil {
				return err
			}

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"service": map[string]any{
						"name": svc.Name,
						"id":   svc.ID,
						"auth": svc.Auth,
					},
					"speakerIP":      speaker.IP,
					"coordinatorIP":  enq.CoordinatorIP(),
					"category":       category,
					"query":          query,
					"index":          index,
					"selected":       item,
					"enqueuedPos":    pos,
					"enqueueOnly":    enqueueOnly,
					"titleEffective": title,
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "Spotify", "Music service name (as shown in `sonos smapi services`)")
	cmd.Flags().StringVar(&category, "category", "tracks", "SMAPI search category (try: tracks, albums, playlists)")
	cmd.Flags().IntVar(&index, "index", 0, "Result index to play (0-based)")
	cmd.Flags().BoolVar(&enqueueOnly, "enqueue", false, "Only enqueue (do not start playback)")
	cmd.Flags().StringVar(&titleOverride, "title", "", "Optional title override for the queued item")

	// Keep the help output stable.
	cmd.Flags().SortFlags = true

	return cmd
}
