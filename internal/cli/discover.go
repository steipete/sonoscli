package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
)

func newDiscoverCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Discover Sonos speakers on the local network",
		Long:  "Sends an SSDP M-SEARCH query and resolves each response to a speaker name via the device description endpoint.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			devices, err := sonos.Discover(ctx, sonos.DiscoverOptions{
				Timeout: flags.Timeout,
			})
			if err != nil {
				return err
			}

			sort.Slice(devices, func(i, j int) bool {
				if devices[i].Name == devices[j].Name {
					return devices[i].IP < devices[j].IP
				}
				return devices[i].Name < devices[j].Name
			})

			if flags.JSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(devices)
			}

			for _, d := range devices {
				fmt.Printf("%s\t%s\t%s\n", d.Name, d.IP, d.UDN)
			}
			return nil
		},
	}
}
