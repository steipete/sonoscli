package cli

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
)

type rootFlags struct {
	IP      string
	Name    string
	Timeout time.Duration
	JSON    bool
	Debug   bool
}

func Execute() error {
	flags := &rootFlags{}

	rootCmd := &cobra.Command{
		Use:          "sonos",
		Short:        "Control Sonos speakers from the command line",
		Long:         "Control Sonos speakers over your local network (UPnP/SOAP): discover devices, show status, control playback, and enqueue Spotify items.",
		Example:      "  sonos discover\n  sonos status --name \"Kitchen\"\n  sonos search spotify \"miles davis so what\"\n  sonos open --name \"Kitchen\" spotify:track:6NmXV4o6bmp704aPGyTVVG\n  sonos volume set --name \"Kitchen\" 25",
		SilenceUsage: true,
		Version:      Version,
	}
	rootCmd.SetVersionTemplate("sonos {{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&flags.IP, "ip", "", "Target speaker IP address")
	rootCmd.PersistentFlags().StringVar(&flags.Name, "name", "", "Target speaker name")
	rootCmd.PersistentFlags().DurationVar(&flags.Timeout, "timeout", 5*time.Second, "Timeout for discovery and network calls")
	rootCmd.PersistentFlags().BoolVar(&flags.JSON, "json", false, "Output JSON where supported")
	rootCmd.PersistentFlags().BoolVar(&flags.Debug, "debug", false, "Enable debug logging")

	rootCmd.AddCommand(newDiscoverCmd(flags))
	rootCmd.AddCommand(newStatusCmd(flags))
	rootCmd.AddCommand(newPlayCmd(flags))
	rootCmd.AddCommand(newPauseCmd(flags))
	rootCmd.AddCommand(newStopCmd(flags))
	rootCmd.AddCommand(newNextCmd(flags))
	rootCmd.AddCommand(newPrevCmd(flags))
	rootCmd.AddCommand(newOpenCmd(flags))
	rootCmd.AddCommand(newEnqueueCmd(flags))
	rootCmd.AddCommand(newSearchCmd(flags))
	rootCmd.AddCommand(newGroupCmd(flags))
	rootCmd.AddCommand(newSceneCmd(flags))
	rootCmd.AddCommand(newFavoritesCmd(flags))
	rootCmd.AddCommand(newPlayURICmd(flags))
	rootCmd.AddCommand(newLineInCmd(flags))
	rootCmd.AddCommand(newTVCmd(flags))
	rootCmd.AddCommand(newQueueCmd(flags))
	rootCmd.AddCommand(newVolumeCmd(flags))
	rootCmd.AddCommand(newMuteCmd(flags))

	ctx := context.Background()
	rootCmd.SetContext(ctx)

	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

func validateTarget(flags *rootFlags) error {
	if flags.IP == "" && flags.Name == "" {
		return errors.New("provide --ip or --name (or run `sonos discover`)")
	}
	return nil
}

func resolveTargetCoordinatorIP(ctx context.Context, flags *rootFlags) (string, error) {
	if err := validateTarget(flags); err != nil {
		return "", err
	}

	// If IP is provided, attempt to resolve to coordinator, but fall back.
	if flags.IP != "" {
		c := sonos.NewClient(flags.IP, flags.Timeout)
		top, err := c.GetTopology(ctx)
		if err != nil {
			return flags.IP, nil
		}
		if coordIP, ok := top.CoordinatorIPFor(flags.IP); ok {
			return coordIP, nil
		}
		return flags.IP, nil
	}

	// Name-based selection: discover a speaker, then use topology.
	devs, err := sonos.Discover(ctx, sonos.DiscoverOptions{Timeout: flags.Timeout})
	if err != nil {
		return "", err
	}
	if len(devs) == 0 {
		return "", errors.New("no speakers found")
	}

	c := sonos.NewClient(devs[0].IP, flags.Timeout)
	top, err := c.GetTopology(ctx)
	if err != nil {
		return "", err
	}
	coordIP, ok := top.CoordinatorIPForName(flags.Name)
	if !ok {
		return "", errors.New("speaker name not found in topology: " + flags.Name)
	}
	return coordIP, nil
}

func coordinatorClient(ctx context.Context, flags *rootFlags) (*sonos.Client, error) {
	ip, err := resolveTargetCoordinatorIP(ctx, flags)
	if err != nil {
		return nil, err
	}
	return sonos.NewClient(ip, flags.Timeout), nil
}
