package cli

import (
	"github.com/spf13/cobra"
)

func newPlayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "play",
		Short: "Resume playback",
		Long:  "Sends AVTransport.Play to the group coordinator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			if err := c.Play(ctx); err != nil {
				return err
			}
			return writeOK(cmd, flags, "play", map[string]any{"coordinatorIP": c.IP})
		},
	}

	cmd.AddCommand(newPlaySpotifyCmd(flags))
	cmd.AddCommand(newPlayAppleMusicCmd(flags))
	return cmd
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
			if err := c.Pause(ctx); err != nil {
				return err
			}
			return writeOK(cmd, flags, "pause", map[string]any{"coordinatorIP": c.IP})
		},
	}
}

func newStopCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop playback",
		Long:  "Sends AVTransport.Stop to the group coordinator. Some sources (e.g. TV input) do not support stop, in which case this becomes a no-op.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			if err := c.StopOrNoop(ctx); err != nil {
				return err
			}
			return writeOK(cmd, flags, "stop", map[string]any{"coordinatorIP": c.IP})
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
			if err := c.Next(ctx); err != nil {
				return err
			}
			return writeOK(cmd, flags, "next", map[string]any{"coordinatorIP": c.IP})
		},
	}
}

func newPrevCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "prev",
		Short: "Go to previous track",
		Long:  "Sends AVTransport.Previous to the group coordinator. If the source rejects previous (common for some streams), it restarts the current track.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}
			if err := c.PreviousOrRestart(ctx); err != nil {
				return err
			}
			return writeOK(cmd, flags, "prev", map[string]any{"coordinatorIP": c.IP})
		},
	}
}
