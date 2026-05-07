package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
	"github.com/steipete/sonoscli/internal/streamproxy"
)

type playlistTrack struct {
	ID       string
	Title    string
	URL      string
	Provider string
	Duration time.Duration // optional; zero when yt-dlp didn't report a duration
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

// playlistRunOptions bundles the per-call configuration for runPlayURLPlaylist.
type playlistRunOptions struct {
	YTDLPPath   string
	FFmpegPath  string
	MediaFormat string
	Bitrate     string
	Port        int
	Limit       int
}

// looksLikePlaylistURL returns true when a URL is unambiguously a YouTube or
// YouTube Music playlist page: `?list=` is present and there's no video id to
// fall back to. Other yt-dlp playlist sources can be opted into with
// `--playlist`.
func looksLikePlaylistURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	switch host {
	case "youtube.com", "music.youtube.com":
	default:
		return false
	}
	q := u.Query()
	return strings.TrimSpace(q.Get("list")) != "" && strings.TrimSpace(q.Get("v")) == ""
}

// runPlayURLPlaylist enumerates rawURL with yt-dlp, queues every track on the
// targeted speaker, and starts playback at track 1. It is invoked by the
// play-url command when playlist mode is selected (auto-detect or `--playlist`).
func runPlayURLPlaylist(cmd *cobra.Command, flags *rootFlags, rawURL string, opts playlistRunOptions) error {
	if opts.Limit < 0 {
		return fmt.Errorf("--playlist-limit must be >= 0")
	}
	tracks, err := playlistEnumerator(cmd.Context(), strings.TrimSpace(opts.YTDLPPath), rawURL, opts.Limit)
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
	listenPort := opts.Port
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
		provider := strings.TrimSpace(t.Provider)
		if provider == "" {
			provider = "yt-dlp"
		}
		proxyTracks = append(proxyTracks, streamproxy.Track{
			Path: trackPath,
			Source: streamproxy.Source{
				URL:             t.URL,
				Title:           t.Title,
				Provider:        provider,
				UseYTDLP:        true,
				DurationSeconds: t.Duration.Seconds(),
			},
		})
		queueURIs = append(queueURIs, fmt.Sprintf("http://%s:%d%s", localIP, listenPort, trackPath))
	}

	cfg := streamproxy.ServerConfig{
		Tracks:      proxyTracks,
		Addr:        fmt.Sprintf("0.0.0.0:%d", listenPort),
		YTDLPPath:   opts.YTDLPPath,
		FFmpegPath:  opts.FFmpegPath,
		Format:      opts.MediaFormat,
		Bitrate:     opts.Bitrate,
		IdleTimeout: 60 * time.Second,
	}
	// Use the first track's path for daemon readiness probing.
	daemon, err := launchStreamDaemon(cmd.Context(), cfg, queueURIs[0])
	if err != nil {
		return err
	}

	// Sonos AddURIToQueue probes each enqueued URI (HEAD, sometimes a short
	// GET) before returning. With long playlists and the proxy on the same
	// LAN this comfortably outruns the default 5 s --timeout, especially
	// when the speaker is also pre-fetching audio for the currently playing
	// track. Use a per-operation timeout that covers the whole queue
	// build-out independently of --timeout.
	queueCtx, cancelQueue := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancelQueue()

	if err := target.client.RemoveAllTracksFromQueue(queueCtx); err != nil {
		return err
	}
	firstTrackNum := 0
	for i, uri := range queueURIs {
		provider := strings.TrimSpace(tracks[i].Provider)
		if provider == "" {
			provider = "yt-dlp"
		}
		meta := sonos.BuildStreamProxyTrackMeta(tracks[i].Title, provider, uri, tracks[i].Duration)
		first, err := target.client.AddURIToQueue(queueCtx, uri, meta, 0, false)
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
	if err := target.client.PlayFromQueueTrack(queueCtx, firstTrackNum); err != nil {
		return err
	}

	out := make([]map[string]any, len(tracks))
	for i, t := range tracks {
		out[i] = map[string]any{"index": i + 1, "title": t.Title, "uri": queueURIs[i], "sourceUrl": t.URL}
	}
	return writeOK(cmd, flags, "play-url", map[string]any{
		"sourceUrl":  rawURL,
		"playlist":   true,
		"trackCount": len(tracks),
		"tracks":     out,
		"publicUrl":  daemon.PublicURL,
		"pid":        daemon.PID,
		"logPath":    daemon.LogPath,
	})
}

func enumerateYTDLPPlaylist(ctx context.Context, ytDLPPath, rawURL string, limit int) ([]playlistTrack, error) {
	path := strings.TrimSpace(ytDLPPath)
	if path == "" {
		path = "yt-dlp"
	}

	args := []string{
		"--no-warnings",
		"--flat-playlist",
		"--print",
		"%(webpage_url)s\t%(url)s\t%(ie_key)s\t%(id)s\t%(duration)s\t%(title)s",
	}
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
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 6)
		if len(parts) < 6 {
			return nil, fmt.Errorf("parse yt-dlp playlist output: expected 6 tab-separated fields, got %d in %q", len(parts), line)
		}
		webpageURL := cleanYTDLPField(parts[0])
		entryURL := cleanYTDLPField(parts[1])
		provider := cleanYTDLPField(parts[2])
		id := cleanYTDLPField(parts[3])
		if id == "" {
			continue
		}
		var duration time.Duration
		if secs, err := strconv.ParseFloat(cleanYTDLPField(parts[4]), 64); err == nil && secs > 0 {
			duration = time.Duration(secs * float64(time.Second))
		}
		title := id
		if t := cleanYTDLPField(parts[5]); t != "" {
			title = t
		}
		sourceURL := choosePlaylistEntryURL(rawURL, webpageURL, entryURL, provider, id)
		if sourceURL == "" {
			return nil, fmt.Errorf("yt-dlp playlist item %q did not include a usable URL", id)
		}
		tracks = append(tracks, playlistTrack{
			ID:       id,
			Title:    title,
			URL:      sourceURL,
			Provider: providerFromYTDLP(provider, sourceURL),
			Duration: duration,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read yt-dlp output: %w", err)
	}
	return tracks, nil
}

func cleanYTDLPField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "NA" {
		return ""
	}
	return value
}

func choosePlaylistEntryURL(rawPlaylistURL, webpageURL, entryURL, provider, id string) string {
	if isHTTPURL(webpageURL) {
		return webpageURL
	}
	if isHTTPURL(entryURL) {
		return entryURL
	}
	if streamproxy.LooksLikeYouTube(rawPlaylistURL) || strings.EqualFold(provider, "Youtube") {
		return youTubeWatchPrefix + url.QueryEscape(id)
	}
	return ""
}

func providerFromYTDLP(provider, sourceURL string) string {
	provider = strings.TrimSpace(provider)
	if provider != "" {
		return provider
	}
	if streamproxy.LooksLikeYouTube(sourceURL) {
		return "YouTube"
	}
	if u, err := url.Parse(strings.TrimSpace(sourceURL)); err == nil && u.Host != "" {
		return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	}
	return "yt-dlp"
}

func isHTTPURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
