package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
	"github.com/steipete/sonoscli/internal/streamproxy"
)

type playlistTrack struct {
	ID    string
	Title string
	URL   string
}

type playlistClient interface {
	RemoveAllTracksFromQueue(ctx context.Context) error
	AddURIToQueue(ctx context.Context, enqueuedURI, enqueuedMeta string, desiredFirstTrackNumber int, enqueueAsNext bool) (firstTrackNumber int, err error)
	PlayFromQueueTrack(ctx context.Context, oneBasedTrackNumber int) error
}

type playlistTarget struct {
	client playlistClient
	ip     string
}

var (
	playlistEnumerator = enumerateYTDLPPlaylist
	youTubeWatchPrefix = "https://www.youtube.com/watch?v="

	newPlaylistTarget = func(ctx context.Context, flags *rootFlags) (playlistTarget, error) {
		c, err := coordinatorClient(ctx, flags)
		if err != nil {
			return playlistTarget{}, err
		}
		return playlistTarget{client: c, ip: c.IP}, nil
	}
)

func newPlayURLPlaylistCmd(flags *rootFlags) *cobra.Command {
	var ytDLPPath string
	var ffmpegPath string
	var mediaFormat string
	var bitrate string
	var port int
	var limit int

	cmd := &cobra.Command{
		Use:          "play-url-playlist <url>",
		Short:        "Play a YouTube/yt-dlp playlist URL by enqueueing every track through the local stream proxy",
		Long:         "Enumerates a yt-dlp-supported playlist URL, starts a single local stream proxy that exposes one HTTP path per track, then clears the speaker's queue, adds each track URL, and plays from the first.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			rawURL := strings.TrimSpace(args[0])
			if rawURL == "" {
				return errors.New("playlist url is required")
			}

			tracks, err := playlistEnumerator(cmd.Context(), strings.TrimSpace(ytDLPPath), rawURL, limit)
			if err != nil {
				return err
			}
			if len(tracks) == 0 {
				return fmt.Errorf("yt-dlp returned no playlist items for %q", rawURL)
			}

			target, err := newPlaylistTarget(cmd.Context(), flags)
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

			proxyTracks := make([]streamproxy.Track, 0, len(tracks))
			queueURIs := make([]string, 0, len(tracks))
			for i, t := range tracks {
				trackPath := fmt.Sprintf("/track-%03d.mp3", i+1)
				proxyTracks = append(proxyTracks, streamproxy.Track{
					Path: trackPath,
					Source: streamproxy.Source{
						URL:      t.URL,
						Title:    t.Title,
						Provider: "YouTube",
						UseYTDLP: true,
					},
				})
				queueURIs = append(queueURIs, fmt.Sprintf("http://%s:%d%s", localIP, listenPort, trackPath))
			}

			cfg := streamproxy.ServerConfig{
				Tracks:      proxyTracks,
				Addr:        fmt.Sprintf("0.0.0.0:%d", listenPort),
				YTDLPPath:   ytDLPPath,
				FFmpegPath:  ffmpegPath,
				Format:      mediaFormat,
				Bitrate:     bitrate,
				IdleTimeout: 60 * time.Second,
			}
			// Use the first track's path for daemon readiness probing.
			daemon, err := launchStreamDaemon(cmd.Context(), cfg, queueURIs[0])
			if err != nil {
				return err
			}

			if err := target.client.RemoveAllTracksFromQueue(cmd.Context()); err != nil {
				return err
			}
			firstTrackNum := 0
			for i, uri := range queueURIs {
				meta := sonos.BuildStreamProxyMeta(tracks[i].Title, "YouTube")
				first, err := target.client.AddURIToQueue(cmd.Context(), uri, meta, 0, false)
				if err != nil {
					return fmt.Errorf("AddURIToQueue track %d: %w", i+1, err)
				}
				if i == 0 && first > 0 {
					firstTrackNum = first
				}
			}
			if firstTrackNum <= 0 {
				firstTrackNum = 1
			}
			if err := target.client.PlayFromQueueTrack(cmd.Context(), firstTrackNum); err != nil {
				return err
			}

			out := make([]map[string]any, len(tracks))
			for i, t := range tracks {
				out[i] = map[string]any{"index": i + 1, "title": t.Title, "uri": queueURIs[i], "sourceUrl": t.URL}
			}
			return writeOK(cmd, flags, "play-url-playlist", map[string]any{
				"sourceUrl":  rawURL,
				"trackCount": len(tracks),
				"tracks":     out,
				"publicUrl":  daemon.PublicURL,
				"pid":        daemon.PID,
				"logPath":    daemon.LogPath,
			})
		},
	}

	cmd.Flags().StringVar(&ytDLPPath, "yt-dlp", "yt-dlp", "Path to yt-dlp")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg", "ffmpeg", "Path to ffmpeg")
	cmd.Flags().StringVar(&mediaFormat, "media-format", streamproxy.DefaultFormat, "yt-dlp format selector applied to every track")
	cmd.Flags().StringVar(&bitrate, "bitrate", "192k", "MP3 proxy bitrate")
	cmd.Flags().IntVar(&port, "port", 0, "Local proxy port (default: random free port)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of playlist items to enqueue (0 = no limit)")
	return cmd
}

func enumerateYTDLPPlaylist(ctx context.Context, ytDLPPath, rawURL string, limit int) ([]playlistTrack, error) {
	path := strings.TrimSpace(ytDLPPath)
	if path == "" {
		path = "yt-dlp"
	}

	args := []string{"--no-warnings", "--flat-playlist", "--print", "%(id)s\t%(title)s"}
	if limit > 0 {
		args = append(args, "--playlist-end", fmt.Sprintf("%d", limit))
	}
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, path, args...) //nolint:gosec // yt-dlp path is configurable by flag; rawURL is supplied by the user.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("yt-dlp playlist enumeration failed: %s", detail)
	}

	var tracks []playlistTrack
	scanner := bufio.NewScanner(strings.NewReader(stdout.String()))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		id := strings.TrimSpace(parts[0])
		if id == "" {
			continue
		}
		title := id
		if len(parts) == 2 {
			if t := strings.TrimSpace(parts[1]); t != "" {
				title = t
			}
		}
		tracks = append(tracks, playlistTrack{
			ID:    id,
			Title: title,
			URL:   youTubeWatchPrefix + url.QueryEscape(id),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read yt-dlp output: %w", err)
	}
	return tracks, nil
}
