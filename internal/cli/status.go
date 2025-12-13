package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
)

type statusOutput struct {
	Device    sonos.Device        `json:"device"`
	Transport sonos.TransportInfo `json:"transport"`
	Position  sonos.PositionInfo  `json:"position"`
	Volume    int                 `json:"volume"`
	Mute      bool                `json:"mute"`
}

func newStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current playback status",
		Long:  "Prints coordinator status (transport state, track URI, position, volume, mute). Use --json for machine-readable output.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}

			dev, _ := c.GetDeviceDescription(ctx)
			transport, _ := c.GetTransportInfo(ctx)
			position, _ := c.GetPositionInfo(ctx)
			vol, _ := c.GetVolume(ctx)
			mute, _ := c.GetMute(ctx)

			out := statusOutput{
				Device:    dev,
				Transport: transport,
				Position:  position,
				Volume:    vol,
				Mute:      mute,
			}

			if flags.JSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Printf("Speaker:\t%s (%s)\n", dev.Name, dev.IP)
			if dev.UDN != "" {
				fmt.Printf("UDN:\t\t%s\n", dev.UDN)
			}
			fmt.Printf("State:\t\t%s\n", transport.State)
			fmt.Printf("Track:\t\t%s\n", position.Track)
			fmt.Printf("URI:\t\t%s\n", position.TrackURI)
			fmt.Printf("Time:\t\t%s / %s\n", position.RelTime, position.TrackDuration)
			fmt.Printf("Volume:\t\t%d\n", vol)
			fmt.Printf("Mute:\t\t%v\n", mute)
			return nil
		},
	}
}
