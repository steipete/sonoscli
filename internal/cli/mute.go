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
				fmt.Println(v)
				return nil
			case "on":
				return c.SetMute(ctx, true)
			case "off":
				return c.SetMute(ctx, false)
			case "toggle":
				v, err := c.GetMute(ctx)
				if err != nil {
					return err
				}
				return c.SetMute(ctx, !v)
			default:
				return errors.New("expected on|off|toggle|get")
			}
		},
	}
}
