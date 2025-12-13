package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
)

type topologyGetter interface {
	GetTopology(ctx context.Context) (sonos.Topology, error)
}

type groupingClient interface {
	JoinGroup(ctx context.Context, coordinatorUUID string) error
	LeaveGroup(ctx context.Context) error
}

var newTopologyGetter = func(ctx context.Context, timeout time.Duration) (topologyGetter, error) {
	devs, err := sonos.Discover(ctx, sonos.DiscoverOptions{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	if len(devs) == 0 {
		return nil, errors.New("no speakers found")
	}
	return sonos.NewClient(devs[0].IP, timeout), nil
}

var newGroupingClient = func(ip string, timeout time.Duration) groupingClient {
	return sonos.NewClient(ip, timeout)
}

func newGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Inspect and control grouping",
	}
	cmd.AddCommand(newGroupStatusCmd(flags))
	cmd.AddCommand(newGroupJoinCmd(flags))
	cmd.AddCommand(newGroupUnjoinCmd(flags))
	return cmd
}

func newGroupStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:          "status",
		Short:        "Show current groups and members",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tg, err := newTopologyGetter(cmd.Context(), flags.Timeout)
			if err != nil {
				return err
			}
			top, err := tg.GetTopology(cmd.Context())
			if err != nil {
				return err
			}

			if flags.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(top)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			for _, g := range top.Groups {
				coord := g.Coordinator
				_, _ = fmt.Fprintf(w, "Group:\t%s\t(%s)\n", coord.Name, coord.IP)
				for _, m := range g.Members {
					mark := " "
					if m.IsCoordinator {
						mark = "*"
					}
					_, _ = fmt.Fprintf(w, "  %s\t%s\t(%s)\n", mark, m.Name, m.IP)
				}
				_, _ = fmt.Fprintln(w)
			}
			return w.Flush()
		},
	}
}

func newGroupJoinCmd(flags *rootFlags) *cobra.Command {
	var to string

	cmd := &cobra.Command{
		Use:          "join --to <name-or-ip>",
		Short:        "Join another group",
		Long:         "Makes the target speaker (via --name/--ip) join the group coordinated by --to.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			to = strings.TrimSpace(to)
			if to == "" {
				return errors.New("--to is required")
			}

			tg, err := newTopologyGetter(cmd.Context(), flags.Timeout)
			if err != nil {
				return err
			}
			top, err := tg.GetTopology(cmd.Context())
			if err != nil {
				return err
			}

			joiner, err := resolveMember(top, flags.Name, flags.IP)
			if err != nil {
				return err
			}
			dest, err := resolveMember(top, to, "")
			if err != nil {
				return err
			}

			joinerGroup, _ := top.GroupForIP(joiner.IP)
			destGroup, ok := top.GroupForIP(dest.IP)
			if !ok {
				return errors.New("destination speaker not found in any group")
			}

			if joinerGroup.ID != "" && joinerGroup.ID == destGroup.ID {
				return nil
			}
			if destGroup.Coordinator.UUID == "" {
				return errors.New("destination group coordinator UUID missing")
			}

			c := newGroupingClient(joiner.IP, flags.Timeout)
			return c.JoinGroup(cmd.Context(), destGroup.Coordinator.UUID)
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "Destination speaker name or IP to join")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func newGroupUnjoinCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "unjoin",
		Short:        "Leave the current group",
		Long:         "Makes the target speaker (via --name/--ip) become a standalone coordinator.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			tg, err := newTopologyGetter(cmd.Context(), flags.Timeout)
			if err != nil {
				return err
			}
			top, err := tg.GetTopology(cmd.Context())
			if err != nil {
				return err
			}

			member, err := resolveMember(top, flags.Name, flags.IP)
			if err != nil {
				return err
			}
			c := newGroupingClient(member.IP, flags.Timeout)
			return c.LeaveGroup(cmd.Context())
		},
	}
	return cmd
}

func resolveMember(top sonos.Topology, name string, ip string) (sonos.Member, error) {
	if strings.TrimSpace(ip) != "" {
		mem, ok := top.FindByIP(strings.TrimSpace(ip))
		if !ok {
			return sonos.Member{}, errors.New("speaker ip not found in topology: " + ip)
		}
		return mem, nil
	}

	// If name looks like an IP address, treat it as such (for --to).
	if strings.TrimSpace(name) != "" && net.ParseIP(strings.TrimSpace(name)) != nil {
		mem, ok := top.FindByIP(strings.TrimSpace(name))
		if !ok {
			return sonos.Member{}, errors.New("speaker ip not found in topology: " + name)
		}
		return mem, nil
	}

	mem, ok := top.FindByName(name)
	if !ok {
		// Try case-insensitive match
		for k, v := range top.ByName {
			if strings.EqualFold(k, name) {
				mem = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return sonos.Member{}, errors.New("speaker name not found in topology: " + name)
	}
	return mem, nil
}
