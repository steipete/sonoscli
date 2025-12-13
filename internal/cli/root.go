package cli

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/appconfig"
	"github.com/steipete/sonoscli/internal/sonos"
)

type rootFlags struct {
	IP      string
	Name    string
	Timeout time.Duration
	Format  string
	JSON    bool // Deprecated: use --format json
	Debug   bool
}

func Execute() error {
	rootCmd, _, err := newRootCmd()
	if err != nil {
		return err
	}
	ctx := context.Background()
	rootCmd.SetContext(ctx)

	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

var newSonosClient = sonos.NewClient
var sonosDiscover = sonos.Discover

var loadAppConfig = func() (appconfig.Config, error) {
	s, err := appconfig.NewDefaultStore()
	if err != nil {
		return appconfig.Config{}, err
	}
	return s.Load()
}

func newRootCmd() (*cobra.Command, *rootFlags, error) {
	flags := &rootFlags{}

	cfg, err := loadAppConfig()
	if err != nil {
		return nil, nil, err
	}
	cfg = cfg.Normalize()

	rootCmd := &cobra.Command{
		Use:          "sonos",
		Short:        "Control Sonos speakers from the command line",
		Long:         "Control Sonos speakers over your local network (UPnP/SOAP): discover devices, show status, control playback, manage groups/queue, and play Spotify (plus Sonos-side SMAPI search).",
		Example:      "  sonos discover\n  sonos status --name \"Kitchen\"\n  sonos smapi search --service \"Spotify\" --category tracks \"miles davis\"\n  sonos open --name \"Kitchen\" spotify:track:6NmXV4o6bmp704aPGyTVVG\n  sonos volume set --name \"Kitchen\" 25",
		SilenceUsage: true,
		Version:      Version,
	}
	rootCmd.SetVersionTemplate("sonos {{.Version}}\n")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.Debug {
			enableDebugLogging()
		}

		format := strings.TrimSpace(flags.Format)
		if format == "" {
			format = formatPlain
		}
		format = strings.ToLower(format)
		if flags.JSON && format == formatPlain {
			format = formatJSON
		}
		norm, err := normalizeFormat(format)
		if err != nil {
			return err
		}
		flags.Format = norm
		return nil
	}

	rootCmd.PersistentFlags().StringVar(&flags.IP, "ip", "", "Target speaker IP address")
	rootCmd.PersistentFlags().StringVar(&flags.Name, "name", cfg.DefaultRoom, "Target speaker name")
	rootCmd.PersistentFlags().DurationVar(&flags.Timeout, "timeout", 5*time.Second, "Timeout for discovery and network calls")
	rootCmd.PersistentFlags().StringVar(&flags.Format, "format", cfg.Format, "Output format: plain|json|tsv")
	rootCmd.PersistentFlags().BoolVar(&flags.JSON, "json", false, "Deprecated: use --format json")
	_ = rootCmd.PersistentFlags().MarkDeprecated("json", "use --format json")
	rootCmd.PersistentFlags().BoolVar(&flags.Debug, "debug", false, "Enable debug logging")

	if err := rootCmd.RegisterFlagCompletionFunc("name", nameFlagCompletion(flags)); err != nil {
		return nil, nil, err
	}

	rootCmd.AddCommand(newDiscoverCmd(flags))
	rootCmd.AddCommand(newConfigCmd(flags))
	rootCmd.AddCommand(newStatusCmd(flags))
	rootCmd.AddCommand(newPlayCmd(flags))
	rootCmd.AddCommand(newPauseCmd(flags))
	rootCmd.AddCommand(newStopCmd(flags))
	rootCmd.AddCommand(newNextCmd(flags))
	rootCmd.AddCommand(newPrevCmd(flags))
	rootCmd.AddCommand(newOpenCmd(flags))
	rootCmd.AddCommand(newEnqueueCmd(flags))
	rootCmd.AddCommand(newSearchCmd(flags))
	rootCmd.AddCommand(newAuthCmd(flags))
	rootCmd.AddCommand(newSMAPICmd(flags))
	rootCmd.AddCommand(newGroupCmd(flags))
	rootCmd.AddCommand(newSceneCmd(flags))
	rootCmd.AddCommand(newFavoritesCmd(flags))
	rootCmd.AddCommand(newPlayURICmd(flags))
	rootCmd.AddCommand(newLineInCmd(flags))
	rootCmd.AddCommand(newTVCmd(flags))
	rootCmd.AddCommand(newQueueCmd(flags))
	rootCmd.AddCommand(newVolumeCmd(flags))
	rootCmd.AddCommand(newMuteCmd(flags))
	rootCmd.AddCommand(newWatchCmd(flags))

	return rootCmd, flags, nil
}

func nameFlagCompletion(flags *rootFlags) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		timeout := completionTimeoutForFlags(flags)
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()

		devs, err := sonosDiscover(ctx, sonos.DiscoverOptions{Timeout: timeout})
		if err != nil || len(devs) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		needle := strings.ToLower(strings.TrimSpace(toComplete))
		seen := map[string]struct{}{}
		out := make([]string, 0, len(devs))
		for _, d := range devs {
			name := strings.TrimSpace(d.Name)
			if name == "" {
				continue
			}
			if needle != "" && !strings.HasPrefix(strings.ToLower(name), needle) {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
		sort.Strings(out)
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

func completionTimeoutForFlags(flags *rootFlags) time.Duration {
	const maxCompletionTimeout = 1 * time.Second

	if flags == nil || flags.Timeout <= 0 {
		return maxCompletionTimeout
	}
	if flags.Timeout < maxCompletionTimeout {
		return flags.Timeout
	}
	return maxCompletionTimeout
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
		c := newSonosClient(flags.IP, flags.Timeout)
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
	devs, err := sonosDiscover(ctx, sonos.DiscoverOptions{Timeout: flags.Timeout})
	if err != nil {
		return "", err
	}
	if len(devs) == 0 {
		return "", errors.New("no speakers found")
	}

	c := newSonosClient(devs[0].IP, flags.Timeout)
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
	return newSonosClient(ip, flags.Timeout), nil
}
