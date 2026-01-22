package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type sourceClient interface {
	SetAVTransportURI(ctx context.Context, uri, meta string) error
	Play(ctx context.Context) error
}

var newSourceClient = func(ctx context.Context, flags *rootFlags) (sourceClient, error) {
	return coordinatorClient(ctx, flags)
}

func newPlayURICmd(flags *rootFlags) *cobra.Command {
	var title string
	var radio bool

	cmd := &cobra.Command{
		Use:          "play-uri <uri>",
		Short:        "Play an arbitrary URI",
		Long:         "Sets the current transport URI and starts playback. Use --radio to force Sonos radio-style playback controls for http/https streams.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			uri := strings.TrimSpace(args[0])
			if uri == "" {
				return errors.New("uri is required")
			}

			c, err := newSourceClient(cmd.Context(), flags)
			if err != nil {
				return err
			}

			meta := ""
			if radio {
				if title == "" {
					title = uri
				}
				uri = sonos.ForceRadioURI(uri)
				meta = sonos.BuildRadioMeta(title)
			} else if strings.TrimSpace(title) != "" {
				meta = sonos.BuildRadioMeta(title)
			}

			if err := c.SetAVTransportURI(cmd.Context(), uri, meta); err != nil {
				return err
			}
			if err := c.Play(cmd.Context()); err != nil {
				return err
			}
			return writeOK(cmd, flags, "play-uri", map[string]any{"uri": uri, "radio": radio})
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Optional display title (used as radio metadata)")
	cmd.Flags().BoolVar(&radio, "radio", false, "Force radio-style playback for http/https streams")
	return cmd
}

func newLineInCmd(flags *rootFlags) *cobra.Command {
	var from string

	cmd := &cobra.Command{
		Use:          "linein",
		Short:        "Switch playback to line-in",
		Long:         "Plays line-in from a source speaker on the target speaker/group.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}

			c, err := newSourceClient(cmd.Context(), flags)
			if err != nil {
				return err
			}

			// Resolve the source UUID via topology.
			tg, err := newTopologyGetter(cmd.Context(), flags.Timeout)
			if err != nil {
				return err
			}
			top, err := tg.GetTopology(cmd.Context())
			if err != nil {
				return err
			}

			source := strings.TrimSpace(from)
			if source == "" {
				// Default to the device the user targeted.
				source = flags.Name
				if source == "" {
					source = flags.IP
				}
			}
			mem, err := resolveMember(top, source, "")
			if err != nil {
				return err
			}
			if mem.UUID == "" {
				return errors.New("line-in source has no UUID in topology")
			}

			uri := "x-rincon-stream:" + mem.UUID
			if err := c.SetAVTransportURI(cmd.Context(), uri, ""); err != nil {
				return err
			}
			if err := c.Play(cmd.Context()); err != nil {
				return err
			}
			return writeOK(cmd, flags, "linein", map[string]any{"from": mem, "uri": uri})
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source speaker name or IP that has line-in (defaults to target)")
	return cmd
}

func newTVCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tv",
		Short:        "Switch playback to TV input",
		Long:         "Switches the target speaker/group to TV input (soundbar/home theater).",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}

			c, err := newSourceClient(cmd.Context(), flags)
			if err != nil {
				return err
			}

			// Resolve UUID of the targeted device.
			tg, err := newTopologyGetter(cmd.Context(), flags.Timeout)
			if err != nil {
				return err
			}
			top, err := tg.GetTopology(cmd.Context())
			if err != nil {
				return err
			}

			target := flags.Name
			if target == "" {
				target = flags.IP
			}
			mem, err := resolveMember(top, target, "")
			if err != nil {
				return err
			}
			if mem.UUID == "" {
				return errors.New("target has no UUID in topology")
			}

			uri := "x-sonos-htastream:" + mem.UUID + ":spdif"
			if err := c.SetAVTransportURI(cmd.Context(), uri, ""); err != nil {
				return err
			}
			if err := c.Play(cmd.Context()); err != nil {
				return err
			}
			return writeOK(cmd, flags, "tv", map[string]any{"target": mem, "uri": uri})
		},
	}
	return cmd
}
