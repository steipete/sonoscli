package cli

import "github.com/spf13/cobra"

func newPlayCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "play",
		Short: "Resume playback",
		Long:  "Sends AVTransport.Play to the group coordinator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			return c.Play(ctx)
		},
	}
}

func newPauseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "Pause playback",
		Long:  "Sends AVTransport.Pause to the group coordinator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			return c.Pause(ctx)
		},
	}
}

func newStopCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop playback",
		Long:  "Sends AVTransport.Stop to the group coordinator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			return c.Stop(ctx)
		},
	}
}

func newNextCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Skip to next track",
		Long:  "Sends AVTransport.Next to the group coordinator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			return c.Next(ctx)
		},
	}
}

func newPrevCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "prev",
		Short: "Go to previous track",
		Long:  "Sends AVTransport.Previous to the group coordinator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			return c.Previous(ctx)
		},
	}
}
