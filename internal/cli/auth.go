package cli

import "github.com/spf13/cobra"

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication helpers (grouped)",
		Long:  "Authentication flows for external systems used by Sonos or this CLI.",
	}

	cmd.AddCommand(newAuthSMAPICmd(flags))
	cmd.AddCommand(newAuthAppleMusicCmd(flags))
	return cmd
}

func newAuthSMAPICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "smapi",
		Short: "Authenticate a Sonos music service (SMAPI DeviceLink/AppLink)",
		Long:  "Authenticate a Sonos music service for SMAPI browsing/search (DeviceLink/AppLink). This is required for some services (e.g. Spotify) before `sonos smapi search` works.",
	}

	cmd.AddCommand(newSMAPIAuthBeginCmd(flags))
	cmd.AddCommand(newSMAPIAuthCompleteCmd(flags))
	return cmd
}
