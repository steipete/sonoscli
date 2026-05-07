package streamproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

type Resolver struct {
	YTDLPPath string
	Format    string
}

func (r Resolver) Resolve(ctx context.Context, rawURL string, mode string) (Source, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return Source{}, fmt.Errorf("url is required")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}

	switch mode {
	case "direct":
		return directSource(rawURL), nil
	case "yt-dlp":
		return r.ResolveYTDLP(ctx, rawURL)
	case "auto":
		if resolverRequired(rawURL) {
			return r.ResolveYTDLP(ctx, rawURL)
		}
		if resolverUseful(rawURL) {
			return r.ResolveYTDLP(ctx, rawURL)
		}
		return directSource(rawURL), nil
	default:
		return Source{}, fmt.Errorf("invalid resolver %q (expected auto|direct|yt-dlp)", mode)
	}
}

func resolverRequired(rawURL string) bool {
	return LooksLikeYouTube(rawURL)
}

func resolverUseful(rawURL string) bool {
	return !LooksLikeDirectMedia(rawURL)
}

func (r Resolver) ResolveYTDLP(ctx context.Context, rawURL string) (Source, error) {
	path := strings.TrimSpace(r.YTDLPPath)
	if path == "" {
		path = "yt-dlp"
	}
	format := strings.TrimSpace(r.Format)
	if format == "" {
		format = DefaultFormat
	}

	cmd := exec.CommandContext(ctx, path, "--no-playlist", "--no-warnings", "-f", format, "-j", rawURL) //nolint:gosec // executable is configured by CLI flag; args are fixed except the requested URL.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return Source{}, fmt.Errorf("yt-dlp failed: %s", detail)
	}

	var raw struct {
		Title      string `json:"title"`
		URL        string `json:"url"`
		WebpageURL string `json:"webpage_url"`
		Extractor  string `json:"extractor_key"`
		Thumbnail  string `json:"thumbnail"`
		FormatID   string `json:"format_id"`
		Ext        string `json:"ext"`
		ACodec     string `json:"acodec"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &raw); err != nil {
		return Source{}, fmt.Errorf("parse yt-dlp json: %w", err)
	}
	if strings.TrimSpace(raw.URL) == "" {
		return Source{}, fmt.Errorf("yt-dlp did not return a playable stream url")
	}
	if raw.WebpageURL == "" {
		raw.WebpageURL = rawURL
	}

	provider := strings.TrimSpace(raw.Extractor)
	if provider == "" {
		provider = providerFromURL(raw.WebpageURL)
	}

	return Source{
		URL:       raw.WebpageURL,
		InputURL:  raw.URL,
		Title:     raw.Title,
		Provider:  provider,
		Thumbnail: raw.Thumbnail,
		UseYTDLP:  true,
		FormatID:  raw.FormatID,
		Ext:       raw.Ext,
		ACodec:    raw.ACodec,
	}, nil
}

func (r Resolver) ResolveStreamURL(ctx context.Context, src Source) (string, error) {
	if strings.TrimSpace(src.InputURL) != "" {
		return strings.TrimSpace(src.InputURL), nil
	}
	if !src.UseYTDLP {
		return strings.TrimSpace(src.URL), nil
	}
	path := strings.TrimSpace(r.YTDLPPath)
	if path == "" {
		path = "yt-dlp"
	}
	format := strings.TrimSpace(r.Format)
	if format == "" {
		format = DefaultFormat
	}
	cmd := exec.CommandContext(ctx, path, "--no-playlist", "--no-warnings", "-f", format, "-g", src.URL) //nolint:gosec // executable is configured by CLI flag; args are fixed except the requested URL.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("yt-dlp stream url failed: %s", detail)
	}
	line := strings.TrimSpace(strings.Split(strings.TrimSpace(stdout.String()), "\n")[0])
	if line == "" {
		return "", fmt.Errorf("yt-dlp returned empty stream url")
	}
	return line, nil
}

func directSource(rawURL string) Source {
	return Source{
		URL:      rawURL,
		InputURL: rawURL,
		Provider: providerFromURL(rawURL),
	}
}

func providerFromURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return "URL"
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	if LooksLikeYouTube(rawURL) {
		return "YouTube"
	}
	return host
}
