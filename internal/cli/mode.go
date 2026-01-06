package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/steipete/sonoscli/internal/sonos"
	"github.com/spf13/cobra"
)

func newModeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "mode <get|shuffle|repeat|normal>",
		Short: "Get or set play mode (shuffle/repeat)",
		Long: `Controls playback mode (shuffle/repeat) on the group coordinator.

Modes:
  get              Show current play mode
  shuffle          Enable shuffle with repeat (SHUFFLE)
  shuffle-norepeat Enable shuffle without repeat (SHUFFLE_NOREPEAT)
  repeat           Enable repeat all without shuffle (REPEAT_ALL)
  repeat-one       Enable repeat single track (REPEAT_ONE)
  normal           Disable shuffle and repeat (NORMAL)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}

			switch strings.ToLower(args[0]) {
			case "get":
				settings, err := c.GetTransportSettings(ctx)
				if err != nil {
					return err
				}
				if isJSON(flags) {
					return writeJSON(cmd, map[string]any{
						"playMode":      string(settings.PlayMode),
						"coordinatorIP": c.IP,
					})
				}
				if isTSV(flags) {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "playMode\t%s\n", settings.PlayMode)
					return nil
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), settings.PlayMode)
				return nil

			case "shuffle":
				if err := c.SetPlayMode(ctx, sonos.PlayModeShuffle); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mode.shuffle", map[string]any{"coordinatorIP": c.IP})

			case "shuffle-norepeat":
				if err := c.SetPlayMode(ctx, sonos.PlayModeShuffleNoRepeat); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mode.shuffle-norepeat", map[string]any{"coordinatorIP": c.IP})

			case "repeat":
				if err := c.SetPlayMode(ctx, sonos.PlayModeRepeatAll); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mode.repeat", map[string]any{"coordinatorIP": c.IP})

			case "repeat-one":
				if err := c.SetPlayMode(ctx, sonos.PlayModeRepeatOne); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mode.repeat-one", map[string]any{"coordinatorIP": c.IP})

			case "normal":
				if err := c.SetPlayMode(ctx, sonos.PlayModeNormal); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mode.normal", map[string]any{"coordinatorIP": c.IP})

			default:
				return errors.New("expected get|shuffle|shuffle-norepeat|repeat|repeat-one|normal")
			}
		},
	}
}
