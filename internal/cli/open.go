package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

func newOpenCmd(flags *rootFlags) *cobra.Command {
	var title string
	var asNext bool

	cmd := &cobra.Command{
		Use:   "open <spotify-uri-or-link>",
		Short: "Enqueue a Spotify item and start playback",
		Long:  "Adds a Spotify item to the Sonos queue using AVTransport.AddURIToQueue, then starts playback on the coordinator.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			ref := args[0]
			_, ok := sonos.ParseSpotifyRef(ref)
			if !ok {
				return errors.New("currently only Spotify refs are supported by `open`")
			}
			pos, err := c.EnqueueSpotify(ctx, ref, sonos.EnqueueOptions{
				Title:   title,
				AsNext:  asNext,
				PlayNow: true,
			})
			if err != nil {
				return err
			}
			return writeOK(cmd, flags, "open", map[string]any{"coordinatorIP": c.IP, "pos": pos})
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Optional display title for the queued item")
	cmd.Flags().BoolVar(&asNext, "next", false, "Enqueue as next (shuffle mode only)")
	return cmd
}

func newEnqueueCmd(flags *rootFlags) *cobra.Command {
	var title string
	var asNext bool

	cmd := &cobra.Command{
		Use:   "enqueue <spotify-uri-or-link>",
		Short: "Enqueue a Spotify item (does not start playback)",
		Long:  "Adds a Spotify item to the Sonos queue using AVTransport.AddURIToQueue (no Play).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			ref := args[0]
			_, ok := sonos.ParseSpotifyRef(ref)
			if !ok {
				return errors.New("currently only Spotify refs are supported by `enqueue`")
			}
			pos, err := c.EnqueueSpotify(ctx, ref, sonos.EnqueueOptions{
				Title:   title,
				AsNext:  asNext,
				PlayNow: false,
			})
			if err != nil {
				return err
			}
			return writeOK(cmd, flags, "enqueue", map[string]any{"coordinatorIP": c.IP, "pos": pos})
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Optional display title for the queued item")
	cmd.Flags().BoolVar(&asNext, "next", false, "Enqueue as next (shuffle mode only)")
	return cmd
}
