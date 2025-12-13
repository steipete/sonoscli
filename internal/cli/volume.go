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
			fmt.Println(v)
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
			return c.SetVolume(ctx, v)
		},
	})

	return cmd
}
