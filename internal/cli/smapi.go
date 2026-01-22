package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

func newSMAPICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "smapi",
		Short: "Sonos music-service browsing/search via SMAPI",
		Long: "SMAPI lets Sonos controllers browse/search linked music services (e.g. Spotify) without Spotify Web API credentials. " +
			"Some services require a one-time DeviceLink/AppLink authentication flow before search works.",
	}

	cmd.AddCommand(newSMAPIServicesCmd(flags))
	cmd.AddCommand(newSMAPICategoriesCmd(flags))
	cmd.AddCommand(newSMAPIBrowseCmd(flags))
	cmd.AddCommand(newSMAPIAuthCmd(flags)) // kept for backwards-compat; hidden below
	cmd.AddCommand(newSMAPISearchCmd(flags))
	return cmd
}

func anySpeakerClient(ctx context.Context, flags *rootFlags) (*sonos.Client, error) {
	if strings.TrimSpace(flags.IP) != "" {
		return newSonosClient(strings.TrimSpace(flags.IP), flags.Timeout), nil
	}

	devs, err := sonosDiscover(ctx, sonos.DiscoverOptions{Timeout: flags.Timeout})
	if err != nil {
		return nil, err
	}
	if len(devs) == 0 {
		return nil, errors.New("no speakers found")
	}
	if strings.TrimSpace(flags.Name) == "" {
		return newSonosClient(devs[0].IP, flags.Timeout), nil
	}

	// Prefer topology resolution by name (more reliable than SSDP name matching).
	c := newSonosClient(devs[0].IP, flags.Timeout)
	top, err := c.GetTopology(ctx)
	if err != nil {
		return c, nil
	}
	mem, ok := top.FindByName(flags.Name)
	if !ok {
		// Try case-insensitive match.
		for k, v := range top.ByName {
			if strings.EqualFold(k, flags.Name) {
				mem = v
				ok = true
				break
			}
		}
	}
	if ok && mem.IP != "" {
		return newSonosClient(mem.IP, flags.Timeout), nil
	}
	return nil, errors.New("speaker name not found: " + flags.Name)
}

func newSMAPIServicesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "services",
		Short: "List available Sonos music services",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := anySpeakerClient(ctx, flags)
			if err != nil {
				return err
			}
			services, err := c.ListAvailableServices(ctx)
			if err != nil {
				return err
			}
			sort.Slice(services, func(i, j int) bool {
				return strings.ToLower(services[i].Name) < strings.ToLower(services[j].Name)
			})

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"speakerIP": c.IP,
					"services":  services,
				})
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tAUTH\tID\tSERVICETYPE\tSECURE_URI")
			for _, s := range services {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Auth, s.ID, s.ServiceType, s.SecureURI)
			}
			return w.Flush()
		},
	}
}

func newSMAPICategoriesCmd(flags *rootFlags) *cobra.Command {
	var serviceName string
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "List SMAPI search categories for a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			speaker, err := anySpeakerClient(ctx, flags)
			if err != nil {
				return err
			}
			services, err := speaker.ListAvailableServices(ctx)
			if err != nil {
				return err
			}
			svc, err := findServiceByName(services, serviceName)
			if err != nil {
				return err
			}

			store, err := newSMAPITokenStore()
			if err != nil {
				return err
			}
			sm, err := sonos.NewSMAPIClient(ctx, speaker, svc, store)
			if err != nil {
				return err
			}
			cats, err := sm.SearchCategories(ctx)
			if err != nil {
				return err
			}

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"speakerIP":   speaker.IP,
					"serviceName": svc.Name,
					"categories":  cats,
				})
			}
			for _, c := range cats {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), c)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serviceName, "service", "Spotify", "Music service name (as shown in `sonos smapi services`)")
	return cmd
}

func newSMAPIAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "auth",
		Short:  "Authenticate a Sonos music service (DeviceLink/AppLink)",
		Hidden: true, // use `sonos auth smapi ...`
	}
	cmd.AddCommand(newSMAPIAuthBeginCmd(flags))
	cmd.AddCommand(newSMAPIAuthCompleteCmd(flags))
	return cmd
}

func findServiceByName(services []sonos.MusicServiceDescriptor, name string) (sonos.MusicServiceDescriptor, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return sonos.MusicServiceDescriptor{}, errors.New("--service is required")
	}
	var exact *sonos.MusicServiceDescriptor
	for i := range services {
		s := services[i]
		if strings.EqualFold(strings.TrimSpace(s.Name), name) {
			exact = &s
			break
		}
	}
	if exact != nil {
		return *exact, nil
	}

	var matches []sonos.MusicServiceDescriptor
	for _, s := range services {
		if strings.Contains(strings.ToLower(s.Name), strings.ToLower(name)) {
			matches = append(matches, s)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Name)
		}
		sort.Strings(names)
		return sonos.MusicServiceDescriptor{}, fmt.Errorf("ambiguous --service %q; matches: %s", name, strings.Join(names, ", "))
	}
	return sonos.MusicServiceDescriptor{}, errors.New("service not found: " + name)
}

func newSMAPIAuthBeginCmd(flags *rootFlags) *cobra.Command {
	var serviceName string
	cmd := &cobra.Command{
		Use:   "begin",
		Short: "Start a music-service linking flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			speaker, err := anySpeakerClient(ctx, flags)
			if err != nil {
				return err
			}
			services, err := speaker.ListAvailableServices(ctx)
			if err != nil {
				return err
			}
			svc, err := findServiceByName(services, serviceName)
			if err != nil {
				return err
			}

			store, err := newSMAPITokenStore()
			if err != nil {
				return err
			}
			sm, err := sonos.NewSMAPIClient(ctx, speaker, svc, store)
			if err != nil {
				return err
			}
			res, err := sm.BeginAuthentication(ctx)
			if err != nil {
				return err
			}

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"speakerIP": speaker.IP,
					"service":   svc,
					"auth":      res,
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Service: %s\n", svc.Name)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Open this URL and link your account:\n  %s\n", res.RegURL)
			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(),
				"Then run:\n  sonos auth smapi complete --service %q --code %s --wait 5m\n",
				svc.Name,
				res.LinkCode,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&serviceName, "service", "Spotify", "Music service name (as shown in `sonos smapi services`)")
	return cmd
}

func newSMAPIAuthCompleteCmd(flags *rootFlags) *cobra.Command {
	var (
		serviceName  string
		linkCode     string
		linkDeviceID string
		wait         time.Duration
	)
	cmd := &cobra.Command{
		Use:   "complete",
		Short: "Complete a music-service linking flow and store tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			speaker, err := anySpeakerClient(ctx, flags)
			if err != nil {
				return err
			}
			services, err := speaker.ListAvailableServices(ctx)
			if err != nil {
				return err
			}
			svc, err := findServiceByName(services, serviceName)
			if err != nil {
				return err
			}

			store, err := newSMAPITokenStore()
			if err != nil {
				return err
			}
			sm, err := sonos.NewSMAPIClient(ctx, speaker, svc, store)
			if err != nil {
				return err
			}
			attempts := 0
			printedWaitHint := false
			pair, err := completeSMAPIAuth(ctx, wait, func(ctx context.Context) (sonos.SMAPITokenPair, error) {
				attempts++
				pair, err := sm.CompleteAuthentication(ctx, linkCode, linkDeviceID)
				if wait > 0 && !isJSON(flags) && isSMAPILinkPending(err) {
					if !printedWaitHint {
						_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Waiting for linking to complete...")
						printedWaitHint = true
					}
					// Don't spam; add a dot per retry and a newline occasionally.
					if attempts%10 == 0 {
						_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Still waiting...")
					} else {
						_, _ = fmt.Fprint(cmd.ErrOrStderr(), ".")
					}
				}
				return pair, err
			})
			if err != nil {
				if isSMAPIInvalidLinkCode(err) {
					return fmt.Errorf("link code is invalid or expired; re-run `sonos auth smapi begin` and use the new code: %w", err)
				}
				return err
			}

			if printedWaitHint {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr())
			}

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"speakerIP": speaker.IP,
					"service":   svc,
					"token":     pair,
				})
			}

			return writeOK(cmd, flags, "smapi_auth_complete", map[string]any{
				"speakerIP":   speaker.IP,
				"serviceName": svc.Name,
				"updatedAt":   pair.UpdatedAt,
			})
		},
	}
	cmd.Flags().StringVar(&serviceName, "service", "Spotify", "Music service name (as shown in `sonos smapi services`)")
	cmd.Flags().StringVar(&linkCode, "code", "", "Link code from `sonos auth smapi begin`")
	cmd.Flags().StringVar(&linkDeviceID, "link-device-id", "", "Optional link device id (returned by begin; usually not needed)")
	cmd.Flags().DurationVar(&wait, "wait", 0, "Wait up to this duration for linking to complete (polls periodically)")
	_ = cmd.MarkFlagRequired("code")
	return cmd
}

func completeSMAPIAuth(
	ctx context.Context,
	wait time.Duration,
	attempt func(context.Context) (sonos.SMAPITokenPair, error),
) (sonos.SMAPITokenPair, error) {
	if wait <= 0 {
		return attempt(ctx)
	}

	deadline := time.Now().Add(wait)

	interval := 2 * time.Second
	if wait < interval {
		// If the user is waiting only a short time, poll more frequently so the
		// command can still succeed before the deadline.
		interval = wait / 5
		if interval < 10*time.Millisecond {
			interval = 10 * time.Millisecond
		}
	}

	for {
		pair, err := attempt(ctx)
		if err == nil {
			return pair, nil
		}
		if !isSMAPILinkPending(err) {
			return sonos.SMAPITokenPair{}, err
		}
		now := time.Now()
		if now.After(deadline) {
			return sonos.SMAPITokenPair{}, fmt.Errorf("timed out waiting for service link to complete: %w", err)
		}

		slog.Debug("smapi auth: waiting for link completion", "err", err.Error())

		sleep := interval
		if remaining := deadline.Sub(now); remaining < sleep {
			sleep = remaining
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return sonos.SMAPITokenPair{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func isSMAPILinkPending(err error) bool {
	if err == nil {
		return false
	}
	// Common error from Spotify (and other services) before the user completes linking.
	// Example: "smapi fault: SOAP-ENV:Server: NOT_LINKED_RETRY"
	msg := err.Error()
	return strings.Contains(msg, "NOT_LINKED_RETRY") || strings.Contains(msg, "NOT_LINKED")
}

func isSMAPIInvalidLinkCode(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(strings.ToLower(msg), "invalid linkcode")
}

func newSMAPISearchCmd(flags *rootFlags) *cobra.Command {
	var (
		serviceName string
		category    string
		limit       int
		doOpen      bool
		doEnqueue   bool
		index       int
	)

	cmd := &cobra.Command{
		Use:          "search <query>",
		Short:        "Search a linked Sonos music service (SMAPI)",
		Long:         "Searches a linked service (e.g. Spotify) via Sonos SMAPI. Does not require Spotify Web API credentials.",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if doOpen && doEnqueue {
				return errors.New("use only one of --open or --enqueue")
			}
			if (doOpen || doEnqueue) && flags.IP == "" && flags.Name == "" {
				return errors.New("--open/--enqueue require --ip or --name")
			}
			if index <= 0 {
				index = 1
			}

			ctx := cmd.Context()
			speaker, err := anySpeakerClient(ctx, flags)
			if err != nil {
				return err
			}
			services, err := speaker.ListAvailableServices(ctx)
			if err != nil {
				return err
			}
			svc, err := findServiceByName(services, serviceName)
			if err != nil {
				return err
			}

			store, err := newSMAPITokenStore()
			if err != nil {
				return err
			}
			sm, err := sonos.NewSMAPIClient(ctx, speaker, svc, store)
			if err != nil {
				return err
			}

			query := strings.TrimSpace(strings.Join(args, " "))
			res, err := sm.Search(ctx, category, query, 0, limit)
			if err != nil {
				return err
			}
			flat := append([]sonos.SMAPIItem{}, res.MediaMetadata...)
			flat = append(flat, res.MediaCollection...)
			if len(flat) == 0 {
				return errors.New("no results")
			}

			if doOpen || doEnqueue {
				if index > len(flat) {
					return fmt.Errorf("--index %d out of range (got %d results)", index, len(flat))
				}
				selected := flat[index-1]
				ref := selected.ID
				if _, ok := sonos.ParseSpotifyRef(ref); !ok {
					return errors.New("selected result is not a supported Spotify ref: " + ref)
				}
				c, err := newSonosEnqueuer(ctx, flags)
				if err != nil {
					return err
				}
				_, err = c.EnqueueSpotify(ctx, ref, sonos.EnqueueOptions{
					PlayNow: doOpen,
				})
				if err != nil {
					return err
				}
			}

			if isJSON(flags) {
				if doOpen || doEnqueue {
					selected := flat[index-1]
					return writeJSON(cmd, map[string]any{
						"speakerIP": speaker.IP,
						"service":   svc,
						"category":  category,
						"query":     query,
						"result":    res,
						"selected":  selected,
						"action": map[string]any{
							"enqueue": true,
							"playNow": doOpen,
						},
					})
				}
				return writeJSON(cmd, map[string]any{
					"speakerIP": speaker.IP,
					"service":   svc,
					"category":  category,
					"query":     query,
					"result":    res,
				})
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "INDEX\tTYPE\tTITLE\tID")
			for i, r := range flat {
				title := r.Title
				if title == "" {
					title = r.Summary
				}
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, r.ItemType, title, r.ID)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "Spotify", "Music service name (as shown in `sonos smapi services`)")
	cmd.Flags().StringVar(&category, "category", "tracks", "Search category (service dependent, e.g. tracks|albums|artists|playlists)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results (1-200 depending on service)")
	cmd.Flags().BoolVar(&doOpen, "open", false, "Open the selected result on Sonos (requires --name/--ip)")
	cmd.Flags().BoolVar(&doEnqueue, "enqueue", false, "Enqueue the selected result on Sonos (requires --name/--ip)")
	cmd.Flags().IntVar(&index, "index", 1, "Which search result to use with --open/--enqueue (1-based)")

	return cmd
}

func newSMAPIBrowseCmd(flags *rootFlags) *cobra.Command {
	var (
		serviceName string
		id          string
		limit       int
		recursive   bool
		doOpen      bool
		doEnqueue   bool
		index       int
	)

	cmd := &cobra.Command{
		Use:          "browse",
		Short:        "Browse a music-service container (SMAPI getMetadata)",
		Long:         "Browses a music service via SMAPI getMetadata. Use `--id root` for the root container, then drill down by passing a returned id.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if doOpen && doEnqueue {
				return errors.New("use only one of --open or --enqueue")
			}
			if (doOpen || doEnqueue) && flags.IP == "" && flags.Name == "" {
				return errors.New("--open/--enqueue require --ip or --name")
			}
			if index <= 0 {
				index = 1
			}

			ctx := cmd.Context()
			speaker, err := anySpeakerClient(ctx, flags)
			if err != nil {
				return err
			}
			services, err := speaker.ListAvailableServices(ctx)
			if err != nil {
				return err
			}
			svc, err := findServiceByName(services, serviceName)
			if err != nil {
				return err
			}

			store, err := newSMAPITokenStore()
			if err != nil {
				return err
			}
			sm, err := sonos.NewSMAPIClient(ctx, speaker, svc, store)
			if err != nil {
				return err
			}

			res, err := sm.GetMetadata(ctx, id, 0, limit, recursive)
			if err != nil {
				return err
			}
			flat := append([]sonos.SMAPIItem{}, res.MediaCollection...)
			flat = append(flat, res.MediaMetadata...)
			if len(flat) == 0 {
				return errors.New("no results")
			}

			if doOpen || doEnqueue {
				if index > len(flat) {
					return fmt.Errorf("--index %d out of range (got %d results)", index, len(flat))
				}
				selected := flat[index-1]
				ref := selected.ID
				if _, ok := sonos.ParseSpotifyRef(ref); !ok {
					return errors.New("selected result is not a supported Spotify ref: " + ref)
				}
				c, err := newSonosEnqueuer(ctx, flags)
				if err != nil {
					return err
				}
				_, err = c.EnqueueSpotify(ctx, ref, sonos.EnqueueOptions{
					PlayNow: doOpen,
				})
				if err != nil {
					return err
				}
			}

			if isJSON(flags) {
				if doOpen || doEnqueue {
					selected := flat[index-1]
					return writeJSON(cmd, map[string]any{
						"speakerIP": speaker.IP,
						"service":   svc,
						"browse":    res,
						"selected":  selected,
						"action": map[string]any{
							"enqueue": true,
							"playNow": doOpen,
						},
					})
				}
				return writeJSON(cmd, map[string]any{
					"speakerIP": speaker.IP,
					"service":   svc,
					"browse":    res,
				})
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "INDEX\tTYPE\tTITLE\tID")
			for i, r := range flat {
				title := r.Title
				if title == "" {
					title = r.Summary
				}
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, r.ItemType, title, r.ID)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "Spotify", "Music service name (as shown in `sonos smapi services`)")
	cmd.Flags().StringVar(&id, "id", "root", "Container/item id to browse (default: root)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "Recursively browse (service dependent)")
	cmd.Flags().BoolVar(&doOpen, "open", false, "Open the selected result on Sonos (requires --name/--ip)")
	cmd.Flags().BoolVar(&doEnqueue, "enqueue", false, "Enqueue the selected result on Sonos (requires --name/--ip)")
	cmd.Flags().IntVar(&index, "index", 1, "Which result to use with --open/--enqueue (1-based)")

	return cmd
}
