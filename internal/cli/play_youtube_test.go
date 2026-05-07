package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeYouTubeResolver struct {
	stream youtubeStream
	err    error
	gotURL string
	gotOpt youtubeResolveOptions
}

func (f *fakeYouTubeResolver) Resolve(ctx context.Context, url string, opts youtubeResolveOptions) (youtubeStream, error) {
	f.gotURL = url
	f.gotOpt = opts
	return f.stream, f.err
}

func TestPlayYouTubeCmd(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newPlayYouTubeCmd(flags)

	fakeSource := &fakeSourceClient{}
	fakeResolver := &fakeYouTubeResolver{
		stream: youtubeStream{
			Title:    "Set Title",
			URL:      "https://rr.example/audio.m4a",
			FormatID: "140",
			Ext:      "m4a",
			ACodec:   "mp4a.40.2",
		},
	}

	origClient := newSourceClient
	origResolver := newYouTubeResolver
	t.Cleanup(func() {
		newSourceClient = origClient
		newYouTubeResolver = origResolver
	})
	newSourceClient = func(ctx context.Context, flags *rootFlags) (sourceClient, error) {
		return fakeSource, nil
	}
	newYouTubeResolver = func(path string) youtubeResolver {
		if path != "/tmp/yt-dlp" {
			t.Fatalf("unexpected yt-dlp path: %q", path)
		}
		return fakeResolver
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{
		"--yt-dlp", "/tmp/yt-dlp",
		"--media-format", "bestaudio",
		"https://www.youtube.com/watch?v=abc",
	})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeResolver.gotURL != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("unexpected resolver url: %q", fakeResolver.gotURL)
	}
	if fakeResolver.gotOpt.Format != "bestaudio" {
		t.Fatalf("unexpected format: %q", fakeResolver.gotOpt.Format)
	}
	if fakeSource.uri != "https://rr.example/audio.m4a" {
		t.Fatalf("unexpected uri: %q", fakeSource.uri)
	}
	if !strings.Contains(fakeSource.meta, "Set Title") {
		t.Fatalf("expected title metadata, got: %q", fakeSource.meta)
	}
	if fakeSource.setCalls != 1 || fakeSource.playCalls != 1 {
		t.Fatalf("expected set+play once, got set=%d play=%d", fakeSource.setCalls, fakeSource.playCalls)
	}
}

func TestPlayYouTubeCmdRadio(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newPlayYouTubeCmd(flags)

	fakeSource := &fakeSourceClient{}
	fakeResolver := &fakeYouTubeResolver{
		stream: youtubeStream{
			Title: "Set Title",
			URL:   "https://rr.example/audio.m4a",
		},
	}

	origClient := newSourceClient
	origResolver := newYouTubeResolver
	t.Cleanup(func() {
		newSourceClient = origClient
		newYouTubeResolver = origResolver
	})
	newSourceClient = func(ctx context.Context, flags *rootFlags) (sourceClient, error) {
		return fakeSource, nil
	}
	newYouTubeResolver = func(path string) youtubeResolver {
		return fakeResolver
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--radio", "https://www.youtube.com/watch?v=abc"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeSource.uri != "x-rincon-mp3radio://rr.example/audio.m4a" {
		t.Fatalf("unexpected radio uri: %q", fakeSource.uri)
	}
	if !strings.Contains(fakeSource.meta, "Set Title") {
		t.Fatalf("expected title metadata, got: %q", fakeSource.meta)
	}
}

func TestPlayYouTubeCmdResolverError(t *testing.T) {
	flags := &rootFlags{Name: "Kitchen", Timeout: 2 * time.Second}
	cmd := newPlayYouTubeCmd(flags)

	fakeResolver := &fakeYouTubeResolver{err: errors.New("boom")}

	origResolver := newYouTubeResolver
	t.Cleanup(func() { newYouTubeResolver = origResolver })
	newYouTubeResolver = func(path string) youtubeResolver {
		return fakeResolver
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"https://www.youtube.com/watch?v=abc"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected resolver error, got: %v", err)
	}
}

func TestParseYTDLPStream(t *testing.T) {
	stream, err := parseYTDLPStream([]byte(`{
		"title": "Video Title",
		"url": "https://rr.example/audio.m4a",
		"webpage_url": "https://www.youtube.com/watch?v=abc",
		"format_id": "140",
		"ext": "m4a",
		"acodec": "mp4a.40.2"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.Title != "Video Title" || stream.URL != "https://rr.example/audio.m4a" || stream.FormatID != "140" {
		t.Fatalf("unexpected stream: %+v", stream)
	}
}

func TestYTDLPResolverResolve(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "yt-dlp")
	script := `#!/bin/sh
if [ "${1:-}" = "fail" ]; then
  echo nope >&2
  exit 1
fi
cat <<'JSON'
{"title":"Video Title","url":"https://rr.example/audio.m4a","webpage_url":"https://www.youtube.com/watch?v=abc","format_id":"140","ext":"m4a","acodec":"mp4a.40.2"}
JSON
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}

	stream, err := (ytDLPResolver{path: path}).Resolve(context.Background(), "https://www.youtube.com/watch?v=abc", youtubeResolveOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.SourceURL != "https://www.youtube.com/watch?v=abc" || stream.URL != "https://rr.example/audio.m4a" {
		t.Fatalf("unexpected stream: %+v", stream)
	}
}
