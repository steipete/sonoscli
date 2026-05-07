package streamproxy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func preflightFFmpeg(ctx context.Context, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultFFmpegPath
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return "", fmt.Errorf("ffmpeg preflight failed for %q: %w", path, err)
	}

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, resolved, "-version") //nolint:gosec // ffmpeg path is an explicit CLI option validated before serving.
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(out.String())
		if detail == "" {
			detail = err.Error()
		}
		if checkCtx.Err() != nil {
			return "", fmt.Errorf("ffmpeg preflight timed out for %q: %w", path, checkCtx.Err())
		}
		return "", fmt.Errorf("ffmpeg preflight failed for %q: %s", path, detail)
	}
	if !strings.Contains(strings.ToLower(out.String()), "ffmpeg") {
		return "", fmt.Errorf("ffmpeg preflight failed for %q: unexpected version output", path)
	}
	return resolved, nil
}

func (s *Server) ffmpegCommand(ctx context.Context, streamURL string) *exec.Cmd {
	//nolint:gosec // ffmpeg path is an explicit CLI option; streamURL is the requested media source.
	return exec.CommandContext(ctx, strings.TrimSpace(s.cfg.FFmpegPath),
		"-hide_banner",
		"-loglevel", "error",
		"-re",
		"-i", streamURL,
		"-vn",
		"-ac", "2",
		"-ar", "44100",
		"-codec:a", "libmp3lame",
		"-b:a", s.cfg.Bitrate,
		"-f", "mp3",
		"pipe:1",
	)
}
