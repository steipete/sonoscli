package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/scenes"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type sceneTopologyGetter interface {
	GetTopology(ctx context.Context) (sonos.Topology, error)
}

type sceneSpeakerClient interface {
	LeaveGroup(ctx context.Context) error
	JoinGroup(ctx context.Context, coordinatorUUID string) error
	GetVolume(ctx context.Context) (int, error)
	SetVolume(ctx context.Context, volume int) error
	GetMute(ctx context.Context) (bool, error)
	SetMute(ctx context.Context, mute bool) error
}

var newSceneStore = func() (scenes.Store, error) {
	return scenes.NewFileStore()
}

var newSceneTopologyGetter = func(ctx context.Context, timeout time.Duration) (sceneTopologyGetter, error) {
	devs, err := sonos.Discover(ctx, sonos.DiscoverOptions{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	if len(devs) == 0 {
		return nil, errors.New("no speakers found")
	}
	return sonos.NewClient(devs[0].IP, timeout), nil
}

var newSceneSpeakerClient = func(ip string, timeout time.Duration) sceneSpeakerClient {
	return sonos.NewClient(ip, timeout)
}

func newSceneCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scene",
		Short: "Save and apply presets (grouping + volumes)",
		Long:  "Scenes capture grouping plus per-room volume/mute, and can be applied later to restore that state.",
	}
	cmd.AddCommand(newSceneListCmd(flags))
	cmd.AddCommand(newSceneSaveCmd(flags))
	cmd.AddCommand(newSceneApplyCmd(flags))
	cmd.AddCommand(newSceneDeleteCmd(flags))
	return cmd
}

func newSceneListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List saved scenes",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newSceneStore()
			if err != nil {
				return err
			}
			metas, err := store.List()
			if err != nil {
				return err
			}
			if isJSON(flags) {
				return writeJSON(cmd, metas)
			}
			if isTSV(flags) {
				for _, m := range metas {
					created := ""
					if !m.CreatedAt.IsZero() {
						created = m.CreatedAt.Format(time.RFC3339)
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", m.Name, created)
				}
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "NAME\tCREATED\n")
			for _, m := range metas {
				created := ""
				if !m.CreatedAt.IsZero() {
					created = m.CreatedAt.Format(time.RFC3339)
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\n", m.Name, created)
			}
			return w.Flush()
		},
	}
}

func newSceneSaveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "save <name>",
		Short:        "Save a scene from current state",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return errors.New("scene name is required")
			}

			store, err := newSceneStore()
			if err != nil {
				return err
			}
			tg, err := newSceneTopologyGetter(cmd.Context(), flags.Timeout)
			if err != nil {
				return err
			}
			top, err := tg.GetTopology(cmd.Context())
			if err != nil {
				return err
			}

			scene := scenes.Scene{
				Name:      name,
				CreatedAt: time.Now().UTC(),
			}

			// Group definition.
			for _, g := range top.Groups {
				coord := g.Coordinator
				memberUUIDs := make([]string, 0, len(g.Members))
				for _, m := range g.Members {
					// Scenes are intended to manage "rooms" (visible zones), not bonded
					// satellites/subs or other invisible devices.
					if !m.IsVisible {
						continue
					}
					if m.UUID != "" {
						memberUUIDs = append(memberUUIDs, m.UUID)
					}
				}
				sort.Strings(memberUUIDs)
				scene.Groups = append(scene.Groups, scenes.SceneGroup{
					ID:              g.ID,
					CoordinatorUUID: coord.UUID,
					CoordinatorName: coord.Name,
					MemberUUIDs:     memberUUIDs,
				})
			}

			// Per-device volume/mute.
			seen := map[string]bool{}
			for _, g := range top.Groups {
				for _, m := range g.Members {
					if !m.IsVisible {
						continue
					}
					if m.UUID == "" || seen[m.UUID] {
						continue
					}
					seen[m.UUID] = true
					c := newSceneSpeakerClient(m.IP, flags.Timeout)
					vol, _ := c.GetVolume(cmd.Context())
					mute, _ := c.GetMute(cmd.Context())
					scene.Devices = append(scene.Devices, scenes.SceneDevice{
						UUID:   m.UUID,
						Name:   m.Name,
						IP:     m.IP,
						Volume: vol,
						Mute:   mute,
					})
				}
			}
			sort.Slice(scene.Devices, func(i, j int) bool { return scene.Devices[i].UUID < scene.Devices[j].UUID })

			if err := store.Put(scene); err != nil {
				return err
			}
			return writeOK(cmd, flags, "scene.save", map[string]any{"name": scene.Name})
		},
	}
	return cmd
}

func newSceneApplyCmd(flags *rootFlags) *cobra.Command {
	var only string

	cmd := &cobra.Command{
		Use:          "apply <name>",
		Short:        "Apply a scene",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return errors.New("scene name is required")
			}

			store, err := newSceneStore()
			if err != nil {
				return err
			}
			scene, ok, err := store.Get(name)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("scene not found: " + name)
			}

			tg, err := newSceneTopologyGetter(cmd.Context(), flags.Timeout)
			if err != nil {
				return err
			}
			top, err := tg.GetTopology(cmd.Context())
			if err != nil {
				return err
			}

			// Map UUID -> IP from current topology. If missing, fall back to stored IP.
			uuidToMember := map[string]sonos.Member{}
			uuidToIP := map[string]string{}
			for _, m := range top.ByIP {
				if m.UUID != "" && m.IP != "" {
					uuidToIP[m.UUID] = m.IP
					uuidToMember[m.UUID] = m
				}
			}

			isVisible := func(uuid string) bool {
				m, ok := uuidToMember[uuid]
				if !ok {
					return false
				}
				return m.IsVisible
			}

			involved := map[string]bool{}
			for _, g := range scene.Groups {
				if g.CoordinatorUUID != "" {
					involved[g.CoordinatorUUID] = isVisible(g.CoordinatorUUID)
				}
				for _, u := range g.MemberUUIDs {
					if u != "" {
						involved[u] = isVisible(u)
					}
				}
			}

			// Optional filter: apply only to one room UUID (resolved by name).
			if strings.TrimSpace(only) != "" {
				mem, ok := top.FindByName(only)
				if !ok {
					for k, v := range top.ByName {
						if strings.EqualFold(k, only) {
							mem = v
							ok = true
							break
						}
					}
				}
				if !ok || mem.UUID == "" {
					return errors.New("speaker not found for --only: " + only)
				}
				for k := range involved {
					involved[k] = false
				}
				involved[mem.UUID] = mem.IsVisible
			}

			// Step 1: ungroup all involved devices.
			for _, dev := range scene.Devices {
				if !involved[dev.UUID] || !isVisible(dev.UUID) {
					continue
				}
				ip := uuidToIP[dev.UUID]
				if ip == "" {
					ip = dev.IP
				}
				if ip == "" {
					continue
				}
				_ = newSceneSpeakerClient(ip, flags.Timeout).LeaveGroup(cmd.Context())
			}

			// Step 2: rebuild groups.
			for _, g := range scene.Groups {
				if g.CoordinatorUUID == "" || !involved[g.CoordinatorUUID] {
					continue
				}
				coordIP := uuidToIP[g.CoordinatorUUID]
				if coordIP == "" {
					// Try to find stored coord IP via devices list.
					for _, d := range scene.Devices {
						if d.UUID == g.CoordinatorUUID {
							coordIP = d.IP
							break
						}
					}
				}
				if coordIP == "" {
					return errors.New("coordinator not found on network: " + g.CoordinatorUUID)
				}

				for _, memberUUID := range g.MemberUUIDs {
					if memberUUID == "" || memberUUID == g.CoordinatorUUID || !involved[memberUUID] || !isVisible(memberUUID) {
						continue
					}
					memberIP := uuidToIP[memberUUID]
					if memberIP == "" {
						for _, d := range scene.Devices {
							if d.UUID == memberUUID {
								memberIP = d.IP
								break
							}
						}
					}
					if memberIP == "" {
						return errors.New("member not found on network: " + memberUUID)
					}
					if err := newSceneSpeakerClient(memberIP, flags.Timeout).JoinGroup(cmd.Context(), g.CoordinatorUUID); err != nil {
						return err
					}
				}
			}

			// Step 3: restore per-device volume/mute.
			for _, dev := range scene.Devices {
				if !involved[dev.UUID] || !isVisible(dev.UUID) {
					continue
				}
				ip := uuidToIP[dev.UUID]
				if ip == "" {
					ip = dev.IP
				}
				if ip == "" {
					continue
				}
				c := newSceneSpeakerClient(ip, flags.Timeout)
				_ = c.SetMute(cmd.Context(), dev.Mute)
				_ = c.SetVolume(cmd.Context(), dev.Volume)
			}

			return writeOK(cmd, flags, "scene.apply", map[string]any{"name": scene.Name, "only": strings.TrimSpace(only)})
		},
	}

	cmd.Flags().StringVar(&only, "only", "", "Only apply to a single room name (experimental)")
	return cmd
}

func newSceneDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:          "delete <name>",
		Short:        "Delete a saved scene",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newSceneStore()
			if err != nil {
				return err
			}
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			return writeOK(cmd, flags, "scene.delete", map[string]any{"name": args[0]})
		},
	}
}
