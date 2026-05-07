package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
	"github.com/steipete/sonoscli/internal/streamproxy"
)

type playURLTarget struct {
	client sourceClient
	ip     string
}

type streamDaemonInfo struct {
	PID       int    `json:"pid"`
	PublicURL string `json:"publicUrl"`
	LogPath   string `json:"logPath"`
	Config    string `json:"config"`
}

var newPlayURLTarget = func(ctx context.Context, flags *rootFlags) (playURLTarget, error) {
	c, err := coordinatorClient(ctx, flags)
	if err != nil {
		return playURLTarget{}, err
	}
	return playURLTarget{client: c, ip: c.IP}, nil
}

var (
	launchStreamDaemon = launchStreamProxyDaemon
	chooseLocalIP      = localIPForRemote
)

func newPlayURLCmd(flags *rootFlags) *cobra.Command {
	var resolverMode string
	var ytDLPPath string
	var ffmpegPath string
	var mediaFormat string
	var title string
	var provider string
	var bitrate string
	var port int
	var playlistMode bool
	var noPlaylist bool
	var playlistLimit int

	cmd := &cobra.Command{
		Use:          "play-url <url>",
		Short:        "Play a URL through a Sonos-safe local stream proxy",
		Long:         "Resolves common media pages with yt-dlp when useful, starts a short-lived local MP3 proxy, points Sonos at it, and exits the proxy when playback ends or goes idle.\n\nUnambiguous YouTube / YouTube Music playlist URLs (`?list=…` with no video id) are auto-detected and every track is enqueued. Use --playlist or --no-playlist to override the detection on ambiguous watch+playlist URLs.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			rawURL := strings.TrimSpace(args[0])
			if rawURL == "" {
				return errors.New("url is required")
			}

			if playlistMode && noPlaylist {
				return errors.New("--playlist and --no-playlist are mutually exclusive")
			}
			runAsPlaylist := playlistMode || (!noPlaylist && looksLikePlaylistURL(rawURL))
			if runAsPlaylist {
				return runPlayURLPlaylist(cmd, flags, rawURL, playlistRunOptions{
					YTDLPPath:   ytDLPPath,
					FFmpegPath:  ffmpegPath,
					MediaFormat: mediaFormat,
					Bitrate:     bitrate,
					Port:        port,
					Limit:       playlistLimit,
				})
			}

			resolver := streamproxy.Resolver{YTDLPPath: ytDLPPath, Format: mediaFormat}
			src, err := resolver.Resolve(cmd.Context(), rawURL, resolverMode)
			if err != nil {
				return err
			}
			if strings.TrimSpace(title) != "" {
				src.Title = strings.TrimSpace(title)
			}
			if strings.TrimSpace(provider) != "" {
				src.Provider = strings.TrimSpace(provider)
			}

			target, err := newPlayURLTarget(cmd.Context(), flags)
			if err != nil {
				return err
			}

			localIP, err := chooseLocalIP(target.ip)
			if err != nil {
				return err
			}
			listenPort := port
			if listenPort == 0 {
				listenPort, err = freeTCPPort()
				if err != nil {
					return err
				}
			}

			proxyPath := "/Sonos%20CLI.mp3"
			publicURL := fmt.Sprintf("http://%s:%d%s", localIP, listenPort, proxyPath)
			cfg := streamproxy.ServerConfig{
				Source:      src,
				Addr:        fmt.Sprintf("0.0.0.0:%d", listenPort),
				Path:        proxyPath,
				YTDLPPath:   ytDLPPath,
				FFmpegPath:  ffmpegPath,
				Format:      mediaFormat,
				Bitrate:     bitrate,
				IdleTimeout: 20 * time.Second,
			}
			daemon, err := launchStreamDaemon(cmd.Context(), cfg, publicURL)
			if err != nil {
				return err
			}

			meta := sonos.BuildStreamProxyMeta("Sonos CLI", src.DisplayProvider())
			uri := publicURL
			if err := target.client.SetAVTransportURI(cmd.Context(), uri, meta); err != nil {
				return err
			}
			if err := target.client.Play(cmd.Context()); err != nil {
				return err
			}

			return writeOK(cmd, flags, "play-url", map[string]any{
				"title":     src.DisplayTitle(),
				"provider":  src.DisplayProvider(),
				"sourceUrl": rawURL,
				"publicUrl": daemon.PublicURL,
				"uri":       uri,
				"pid":       daemon.PID,
				"logPath":   daemon.LogPath,
				"useYTDLP":  src.UseYTDLP,
			})
		},
	}

	cmd.Flags().StringVar(&resolverMode, "resolver", "auto", "Resolver mode: auto|direct|yt-dlp")
	cmd.Flags().StringVar(&ytDLPPath, "yt-dlp", "yt-dlp", "Path to yt-dlp")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg", "ffmpeg", "Path to ffmpeg")
	cmd.Flags().StringVar(&mediaFormat, "media-format", streamproxy.DefaultFormat, "yt-dlp media format selector")
	cmd.Flags().StringVar(&title, "title", "", "Override display title")
	cmd.Flags().StringVar(&provider, "provider", "", "Override provider/source label")
	cmd.Flags().StringVar(&bitrate, "bitrate", "192k", "MP3 proxy bitrate")
	cmd.Flags().BoolVar(&playlistMode, "playlist", false, "Force playlist mode (enumerate every track and enqueue)")
	cmd.Flags().BoolVar(&noPlaylist, "no-playlist", false, "Force single-track mode for playlist URLs")
	cmd.Flags().IntVar(&playlistLimit, "playlist-limit", 0, "Maximum number of items to enqueue when in playlist mode (0 = no limit)")
	cmd.Flags().IntVar(&port, "port", 0, "Local proxy port (default: random free port)")
	return cmd
}

func launchStreamProxyDaemon(ctx context.Context, cfg streamproxy.ServerConfig, publicURL string) (streamDaemonInfo, error) {
	token, err := streamproxy.NewHealthToken()
	if err != nil {
		return streamDaemonInfo{}, err
	}
	cfg.HealthToken = token

	cfg, err = cfg.Normalize(ctx)
	if err != nil {
		return streamDaemonInfo{}, err
	}

	dir, err := streamProxyDir()
	if err != nil {
		return streamDaemonInfo{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return streamDaemonInfo{}, err
	}

	configPath := filepath.Join(dir, "current.json")
	logPath := filepath.Join(dir, "streamd.log")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return streamDaemonInfo{}, err
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return streamDaemonInfo{}, err
	}

	exe, err := os.Executable()
	if err != nil {
		return streamDaemonInfo{}, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // logPath is created under the user's cache directory.
	if err != nil {
		return streamDaemonInfo{}, err
	}
	defer func() { _ = logFile.Close() }()

	proc := exec.Command(exe, "stream-daemon", "serve", "--config", configPath) //nolint:gosec // re-executes the current sonos binary with a generated config file.
	proc.Stdout = logFile
	proc.Stderr = logFile
	detachProcess(proc)
	if err := proc.Start(); err != nil {
		return streamDaemonInfo{}, err
	}
	procExited := make(chan error, 1)
	go func() {
		procExited <- proc.Wait()
	}()

	healthURL, err := streamProxyHealthURL(publicURL, cfg.HealthToken)
	if err != nil {
		_ = proc.Process.Kill()
		return streamDaemonInfo{}, err
	}
	if err := waitForStreamProxy(ctx, healthURL, procExited, 5*time.Second); err != nil {
		_ = proc.Process.Kill()
		return streamDaemonInfo{}, err
	}

	return streamDaemonInfo{PID: proc.Process.Pid, PublicURL: publicURL, LogPath: logPath, Config: configPath}, nil
}

func streamProxyDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sonoscli", "stream-daemon"), nil
}

func streamProxyHealthURL(publicURL, token string) (string, error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return "", err
	}
	u.Path = streamproxy.HealthPath
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	if token != "" {
		q := u.Query()
		q.Set(streamproxy.HealthTokenQuery, token)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func waitForStreamProxy(ctx context.Context, healthURL string, procExited <-chan error, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		select {
		case err := <-procExited:
			return streamProxyExitedError(err)
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, healthURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req) //nolint:gosec // URL points to the local proxy URL this process just generated.
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				select {
				case err := <-procExited:
					return streamProxyExitedError(err)
				case <-time.After(150 * time.Millisecond):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		select {
		case err := <-procExited:
			return streamProxyExitedError(err)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return fmt.Errorf("stream proxy did not start at %s", healthURL)
}

func streamProxyExitedError(err error) error {
	if err == nil {
		return errors.New("stream proxy exited before readiness")
	}
	return fmt.Errorf("stream proxy exited before readiness: %w", err)
}

func localIPForRemote(remoteIP string) (string, error) {
	remoteIP = strings.TrimSpace(remoteIP)
	if remoteIP != "" {
		conn, err := net.DialTimeout("udp", net.JoinHostPort(remoteIP, "1400"), time.Second)
		if err == nil {
			defer func() { _ = conn.Close() }()
			if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
				return addr.IP.String(), nil
			}
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip != nil {
				return ip.String(), nil
			}
		}
	}
	return "", errors.New("could not determine a LAN IP for the stream proxy")
}

func freeTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = ln.Close() }()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}
	return n, nil
}
