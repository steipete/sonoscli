package streamproxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

const (
	DefaultAddr                          = "127.0.0.1:0"
	DefaultPath                          = "/stream.mp3"
	DefaultFFmpegPath                    = "ffmpeg"
	DefaultYTDLPPath                     = "yt-dlp"
	DefaultBitrate                       = "192k"
	DefaultIdleTimeout                   = 20 * time.Second
	DefaultIncompletePlaylistIdleTimeout = 5 * time.Minute
	HealthPath                           = "/healthz"
	HealthTokenQuery                     = "token"
)

type ServerConfig struct {
	Source      Source
	Tracks      []Track
	Addr        string
	Path        string
	YTDLPPath   string
	FFmpegPath  string
	Format      string
	Bitrate     string
	IdleTimeout time.Duration
	HealthToken string
}

// Track is one item in a multi-track stream proxy. Each track has its own
// HTTP path on the daemon and its own source. When ServerConfig.Tracks is
// empty, the daemon serves a single Source at ServerConfig.Path.
type Track struct {
	Path   string
	Source Source
}

func (cfg ServerConfig) Normalize(ctx context.Context) (ServerConfig, error) {
	cfg = cfg.withDefaults()
	ffmpegPath, err := preflightFFmpeg(ctx, cfg.FFmpegPath)
	if err != nil {
		return ServerConfig{}, err
	}
	cfg.FFmpegPath = ffmpegPath
	return cfg, nil
}

func (cfg ServerConfig) withDefaults() ServerConfig {
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = DefaultAddr
	}
	if strings.TrimSpace(cfg.Path) == "" {
		cfg.Path = DefaultPath
	}
	if strings.TrimSpace(cfg.FFmpegPath) == "" {
		cfg.FFmpegPath = DefaultFFmpegPath
	}
	if strings.TrimSpace(cfg.YTDLPPath) == "" {
		cfg.YTDLPPath = DefaultYTDLPPath
	}
	if strings.TrimSpace(cfg.Format) == "" {
		cfg.Format = DefaultFormat
	}
	if strings.TrimSpace(cfg.Bitrate) == "" {
		cfg.Bitrate = DefaultBitrate
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	if len(cfg.Tracks) == 0 {
		cfg.Tracks = []Track{{Path: cfg.Path, Source: cfg.Source}}
	}
	return cfg
}

// MultiTrack reports whether the daemon is serving more than one source.
func (cfg ServerConfig) MultiTrack() bool {
	return len(cfg.Tracks) > 1
}

func (cfg ServerConfig) incompletePlaylistIdleTimeout() time.Duration {
	timeout := cfg.IdleTimeout * 5
	if timeout < DefaultIncompletePlaylistIdleTimeout {
		timeout = DefaultIncompletePlaylistIdleTimeout
	}
	return timeout
}

func NewHealthToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
