package streamproxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

const (
	DefaultAddr        = "127.0.0.1:0"
	DefaultPath        = "/stream.mp3"
	DefaultFFmpegPath  = "ffmpeg"
	DefaultYTDLPPath   = "yt-dlp"
	DefaultBitrate     = "192k"
	DefaultIdleTimeout = 20 * time.Second
	HealthPath         = "/healthz"
	HealthTokenQuery   = "token"
)

type ServerConfig struct {
	Source      Source
	Addr        string
	Path        string
	YTDLPPath   string
	FFmpegPath  string
	Format      string
	Bitrate     string
	IdleTimeout time.Duration
	HealthToken string
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
	return cfg
}

func NewHealthToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
