package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newVolumeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Get or set volume",
		Long:  "Controls RenderingControl volume on the group coordinator (0-100).",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get volume",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			v, err := c.GetVolume(ctx)
			if err != nil {
				return err
			}
			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{"volume": v, "coordinatorIP": c.IP})
			}
			if isTSV(flags) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "volume\t%d\n", v)
				return nil
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set <0-100>",
		Short: "Set volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			v, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}
			if err := c.SetVolume(ctx, v); err != nil {
				return err
			}
			return writeOK(cmd, flags, "volume.set", map[string]any{"coordinatorIP": c.IP, "volume": v})
		},
	})

	return cmd
}
