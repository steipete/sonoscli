package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type watchEvent struct {
	Time    time.Time         `json:"time"`
	Service string            `json:"service"`
	SID     string            `json:"sid"`
	Seq     string            `json:"seq"`
	Vars    map[string]string `json:"vars"`
}

func listenIPForRemote(remoteIP string) (string, error) {
	conn, err := net.Dial("udp", net.JoinHostPort(remoteIP, "1900"))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || udpAddr.IP == nil {
		return "", errors.New("could not determine local listen ip")
	}
	return udpAddr.IP.String(), nil
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var duration time.Duration

	cmd := &cobra.Command{
		Use:          "watch",
		Short:        "Watch live Sonos events",
		Long:         "Subscribes to AVTransport and RenderingControl events and prints changes as they arrive (Ctrl+C to stop). Requires that Sonos speakers can reach your machine on the chosen callback port (firewall may prompt).",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}

			ctx := cmd.Context()
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
			defer stop()
			if duration > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, duration)
				defer cancel()
			}

			c, err := coordinatorClient(ctx, flags)
			if err != nil {
				return err
			}

			listenIP, err := listenIPForRemote(c.IP)
			if err != nil {
				return err
			}
			ln, err := net.Listen("tcp", net.JoinHostPort(listenIP, "0"))
			if err != nil {
				return err
			}
			defer ln.Close()
			port := ln.Addr().(*net.TCPAddr).Port

			callbackURL := fmt.Sprintf("http://%s:%d/notify", listenIP, port)

			events := make(chan watchEvent, 128)
			var sidToService sync.Map // sid -> service name

			mux := http.NewServeMux()
			mux.HandleFunc("/notify", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "NOTIFY" {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				sid := strings.TrimSpace(r.Header.Get("SID"))
				seq := strings.TrimSpace(r.Header.Get("SEQ"))
				body, _ := io.ReadAll(r.Body)
				_ = r.Body.Close()

				service := "unknown"
				if v, ok := sidToService.Load(sid); ok {
					service = v.(string)
				}

				vars, err := sonos.ParseEvent(body)
				if err != nil {
					vars = map[string]string{"parse_error": err.Error()}
				}

				select {
				case events <- watchEvent{
					Time:    time.Now().UTC(),
					Service: service,
					SID:     sid,
					Seq:     seq,
					Vars:    vars,
				}:
				default:
					// Drop if the consumer is too slow.
				}

				w.WriteHeader(http.StatusOK)
			})

			srv := &http.Server{
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
			}
			go func() { _ = srv.Serve(ln) }()
			defer func() { _ = srv.Shutdown(context.Background()) }()

			avtSub, err := c.SubscribeAVTransport(ctx, callbackURL, 0)
			if err != nil {
				return err
			}
			defer func() { _ = c.Unsubscribe(context.Background(), avtSub) }()
			sidToService.Store(avtSub.SID, "avtransport")

			rcSub, err := c.SubscribeRenderingControl(ctx, callbackURL, 0)
			if err != nil {
				return err
			}
			defer func() { _ = c.Unsubscribe(context.Background(), rcSub) }()
			sidToService.Store(rcSub.SID, "renderingcontrol")

			if !isJSON(flags) && !isTSV(flags) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Watching events (callback %s). Press Ctrl+C to stop.\n", callbackURL)
			}

			for {
				select {
				case <-ctx.Done():
					return nil
				case ev := <-events:
					if isJSON(flags) {
						_ = writeJSONLine(cmd, ev)
						continue
					}
					if isTSV(flags) {
						keys := make([]string, 0, len(ev.Vars))
						for k := range ev.Vars {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						for _, k := range keys {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", ev.Time.Format(time.RFC3339Nano), ev.Service, ev.SID, k, ev.Vars[k])
						}
						continue
					}

					keys := make([]string, 0, len(ev.Vars))
					for k := range ev.Vars {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					parts := make([]string, 0, len(keys))
					for _, k := range keys {
						parts = append(parts, fmt.Sprintf("%s=%s", k, ev.Vars[k]))
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s [%s] %s\n", ev.Time.Format(time.RFC3339), ev.Service, strings.Join(parts, " "))
				}
			}
		},
	}

	cmd.Flags().DurationVar(&duration, "duration", 0, "Stop after this duration (0 = until Ctrl+C)")
	return cmd
}
