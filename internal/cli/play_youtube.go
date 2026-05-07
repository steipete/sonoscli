package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/sonos"
)

const defaultYouTubeFormat = "bestaudio[ext=m4a][acodec!=none]/bestaudio[acodec^=mp4a]/bestaudio[acodec!=none]/bestaudio"

type youtubeStream struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	SourceURL string `json:"sourceUrl"`
	FormatID  string `json:"formatId"`
	Ext       string `json:"ext"`
	ACodec    string `json:"acodec"`
}

type youtubeResolver interface {
	Resolve(ctx context.Context, url string, opts youtubeResolveOptions) (youtubeStream, error)
}

type youtubeResolveOptions struct {
	Format string
}

type ytDLPResolver struct {
	path string
}

var newYouTubeResolver = func(path string) youtubeResolver {
	return ytDLPResolver{path: path}
}

func newPlayYouTubeCmd(flags *rootFlags) *cobra.Command {
	var ytDLPPath string
	var mediaFormat string
	var title string
	var radio bool

	cmd := &cobra.Command{
		Use:          "youtube <url>",
		Short:        "Play a YouTube URL via yt-dlp",
		Long:         "Resolves a YouTube URL to a temporary audio stream with yt-dlp, sets it as the current Sonos transport URI, and starts playback.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}

			sourceURL := strings.TrimSpace(args[0])
			if sourceURL == "" {
				return errors.New("youtube url is required")
			}

			resolver := newYouTubeResolver(strings.TrimSpace(ytDLPPath))
			stream, err := resolver.Resolve(cmd.Context(), sourceURL, youtubeResolveOptions{
				Format: strings.TrimSpace(mediaFormat),
			})
			if err != nil {
				return err
			}
			if strings.TrimSpace(stream.URL) == "" {
				return errors.New("yt-dlp did not return a playable stream url")
			}

			c, err := newSourceClient(cmd.Context(), flags)
			if err != nil {
				return err
			}

			displayTitle := strings.TrimSpace(title)
			if displayTitle == "" {
				displayTitle = strings.TrimSpace(stream.Title)
			}

			uri := stream.URL
			meta := ""
			if radio {
				if displayTitle == "" {
					displayTitle = sourceURL
				}
				uri = sonos.ForceRadioURI(uri)
				meta = sonos.BuildRadioMeta(displayTitle)
			} else if displayTitle != "" {
				meta = sonos.BuildRadioMeta(displayTitle)
			}

			if err := c.SetAVTransportURI(cmd.Context(), uri, meta); err != nil {
				return err
			}
			if err := c.Play(cmd.Context()); err != nil {
				return err
			}

			return writeOK(cmd, flags, "play-youtube", map[string]any{
				"title":     displayTitle,
				"uri":       uri,
				"sourceUrl": sourceURL,
				"formatId":  stream.FormatID,
				"ext":       stream.Ext,
				"acodec":    stream.ACodec,
				"radio":     radio,
			})
		},
	}

	cmd.Flags().StringVar(&ytDLPPath, "yt-dlp", "yt-dlp", "Path to yt-dlp")
	cmd.Flags().StringVar(&mediaFormat, "media-format", defaultYouTubeFormat, "yt-dlp media format selector")
	cmd.Flags().StringVar(&title, "title", "", "Optional display title (defaults to the YouTube title)")
	cmd.Flags().BoolVar(&radio, "radio", false, "Force radio-style playback for the resolved stream")
	return cmd
}

func (r ytDLPResolver) Resolve(ctx context.Context, url string, opts youtubeResolveOptions) (youtubeStream, error) {
	path := strings.TrimSpace(r.path)
	if path == "" {
		path = "yt-dlp"
	}
	format := strings.TrimSpace(opts.Format)
	if format == "" {
		format = defaultYouTubeFormat
	}

	args := []string{
		"--no-playlist",
		"--no-warnings",
		"-f", format,
		"-j",
		url,
	}

	cmd := exec.CommandContext(ctx, path, args...) //nolint:gosec // path is user-configurable by flag; args are fixed except the requested media URL.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return youtubeStream{}, fmt.Errorf("yt-dlp failed: %s", detail)
	}

	stream, err := parseYTDLPStream(stdout.Bytes())
	if err != nil {
		return youtubeStream{}, err
	}
	stream.SourceURL = url
	return stream, nil
}

func parseYTDLPStream(data []byte) (youtubeStream, error) {
	var raw struct {
		Title      string `json:"title"`
		URL        string `json:"url"`
		WebpageURL string `json:"webpage_url"`
		FormatID   string `json:"format_id"`
		Ext        string `json:"ext"`
		ACodec     string `json:"acodec"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &raw); err != nil {
		return youtubeStream{}, fmt.Errorf("parse yt-dlp json: %w", err)
	}
	return youtubeStream{
		Title:     raw.Title,
		URL:       raw.URL,
		SourceURL: raw.WebpageURL,
		FormatID:  raw.FormatID,
		Ext:       raw.Ext,
		ACodec:    raw.ACodec,
	}, nil
}
