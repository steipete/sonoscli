package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

var discoverFunc = sonos.Discover

func newDiscoverCmd(flags *rootFlags) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover Sonos speakers on the local network",
		Long:  "Sends an SSDP M-SEARCH query and resolves each response to a speaker name via the device description endpoint.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			devices, err := discoverFunc(ctx, sonos.DiscoverOptions{
				Timeout:          flags.Timeout,
				IncludeInvisible: all,
			})
			if err != nil {
				return err
			}

			if devices == nil {
				devices = []sonos.Device{}
			}

			sort.Slice(devices, func(i, j int) bool {
				if devices[i].Name == devices[j].Name {
					return devices[i].IP < devices[j].IP
				}
				return devices[i].Name < devices[j].Name
			})

			if len(devices) == 0 {
				// JSON output has a sensible empty representation (`[]`) for scripts.
				if isJSON(flags) {
					return writeJSON(cmd, devices)
				}
				return errors.New("no speakers found (try increasing --timeout)")
			}

			if isJSON(flags) {
				return writeJSON(cmd, devices)
			}

			for _, d := range devices {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", d.Name, d.IP, d.UDN)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Include invisible/bonded devices (advanced)")
	return cmd
}
