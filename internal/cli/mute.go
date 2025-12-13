package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newMuteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "mute <on|off|toggle|get>",
		Short: "Get or set mute",
		Long:  "Controls RenderingControl mute on the group coordinator.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}

			switch strings.ToLower(args[0]) {
			case "get":
				v, err := c.GetMute(ctx)
				if err != nil {
					return err
				}
				if isJSON(flags) {
					return writeJSON(cmd, map[string]any{"mute": v, "coordinatorIP": c.IP})
				}
				if isTSV(flags) {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mute\t%v\n", v)
					return nil
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), v)
				return nil
			case "on":
				if err := c.SetMute(ctx, true); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mute.on", map[string]any{"coordinatorIP": c.IP})
			case "off":
				if err := c.SetMute(ctx, false); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mute.off", map[string]any{"coordinatorIP": c.IP})
			case "toggle":
				v, err := c.GetMute(ctx)
				if err != nil {
					return err
				}
				if err := c.SetMute(ctx, !v); err != nil {
					return err
				}
				return writeOK(cmd, flags, "mute.toggle", map[string]any{"coordinatorIP": c.IP, "mute": !v})
			default:
				return errors.New("expected on|off|toggle|get")
			}
		},
	}
}
