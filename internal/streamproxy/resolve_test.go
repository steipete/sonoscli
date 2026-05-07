package streamproxy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveAutoYouTubePropagatesYTDLPFailure(t *testing.T) {
	t.Parallel()

	missingYTDLP := filepath.Join(t.TempDir(), "missing-yt-dlp")
	_, err := (Resolver{YTDLPPath: missingYTDLP}).Resolve(context.Background(), "https://www.youtube.com/watch?v=abc", "auto")
	if err == nil {
		t.Fatalf("expected yt-dlp error")
	}
	if !strings.Contains(err.Error(), "yt-dlp failed") {
		t.Fatalf("expected yt-dlp failure, got %v", err)
	}
}

func TestResolveAutoPageURLPropagatesYTDLPFailure(t *testing.T) {
	t.Parallel()

	missingYTDLP := filepath.Join(t.TempDir(), "missing-yt-dlp")
	_, err := (Resolver{YTDLPPath: missingYTDLP}).Resolve(context.Background(), "https://soundcloud.com/artist/track", "auto")
	if err == nil {
		t.Fatalf("expected yt-dlp error")
	}
	if !strings.Contains(err.Error(), "yt-dlp failed") {
		t.Fatalf("expected yt-dlp failure, got %v", err)
	}
}

func TestResolverPolicy(t *testing.T) {
	t.Parallel()

	if !resolverRequired("https://www.youtube.com/watch?v=abc") {
		t.Fatalf("expected YouTube to require resolver")
	}
	if resolverRequired("https://example.com/episode.mp3") {
		t.Fatalf("expected direct media not to require resolver")
	}
	if resolverUseful("https://example.com/episode.mp3") {
		t.Fatalf("expected direct media not to try resolver")
	}
	if !resolverUseful("https://example.com/watch/episode") {
		t.Fatalf("expected unknown page URL to try resolver")
	}
}

func TestResolveModes(t *testing.T) {
	t.Parallel()

	src, err := (Resolver{}).Resolve(context.Background(), "https://example.com/file.mp3", "direct")
	if err != nil {
		t.Fatalf("unexpected direct error: %v", err)
	}
	if src.InputURL != "https://example.com/file.mp3" {
		t.Fatalf("unexpected direct source: %+v", src)
	}
	if _, err := (Resolver{}).Resolve(context.Background(), "", "auto"); err == nil {
		t.Fatalf("expected empty URL error")
	}
	if _, err := (Resolver{}).Resolve(context.Background(), "https://example.com/file.mp3", "bogus"); err == nil {
		t.Fatalf("expected invalid mode error")
	}
}

func TestResolveYTDLPSuccess(t *testing.T) {
	t.Parallel()

	ytDLP := fakeYTDLP(t)
	src, err := (Resolver{YTDLPPath: ytDLP}).Resolve(context.Background(), "https://soundcloud.com/artist/track", "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !src.UseYTDLP {
		t.Fatalf("expected yt-dlp source")
	}
	if src.URL != "https://soundcloud.com/artist/track" {
		t.Fatalf("unexpected webpage URL: %q", src.URL)
	}
	if src.InputURL != "https://cdn.example/audio.m4a" {
		t.Fatalf("unexpected input URL: %q", src.InputURL)
	}
	if src.Provider != "SoundCloud" || src.FormatID != "140" || src.ACodec != "mp4a.40.2" {
		t.Fatalf("unexpected source: %+v", src)
	}
}

func TestResolveStreamURL(t *testing.T) {
	t.Parallel()

	resolver := Resolver{YTDLPPath: fakeYTDLP(t)}

	got, err := resolver.ResolveStreamURL(context.Background(), Source{InputURL: " https://cdn.example/direct.mp3 "})
	if err != nil {
		t.Fatalf("unexpected direct input error: %v", err)
	}
	if got != "https://cdn.example/direct.mp3" {
		t.Fatalf("direct input = %q", got)
	}

	got, err = resolver.ResolveStreamURL(context.Background(), Source{URL: "https://example.com/page", UseYTDLP: true})
	if err != nil {
		t.Fatalf("unexpected yt-dlp stream error: %v", err)
	}
	if got != "https://cdn.example/from-g.m4a" {
		t.Fatalf("yt-dlp stream = %q", got)
	}

	got, err = resolver.ResolveStreamURL(context.Background(), Source{URL: " https://example.com/file.mp3 "})
	if err != nil {
		t.Fatalf("unexpected direct source error: %v", err)
	}
	if got != "https://example.com/file.mp3" {
		t.Fatalf("direct source = %q", got)
	}
}

func TestDirectSourceProvider(t *testing.T) {
	t.Parallel()

	src := directSource("https://www.youtube.com/watch?v=abc")
	if src.Provider != "YouTube" {
		t.Fatalf("provider = %q, want YouTube", src.Provider)
	}
	if got := providerFromURL("not a url"); got != "URL" {
		t.Fatalf("provider = %q, want URL", got)
	}
}

func fakeYTDLP(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "yt-dlp")
	script := `#!/bin/sh
case " $* " in
  *" -g "*)
    printf '%s\n' 'https://cdn.example/from-g.m4a'
    ;;
  *" --warmup "*)
    exit 0
    ;;
  *)
    cat <<'JSON'
{"title":"Track Title","url":"https://cdn.example/audio.m4a","webpage_url":"https://soundcloud.com/artist/track","extractor_key":"SoundCloud","thumbnail":"https://img.example/t.jpg","format_id":"140","ext":"m4a","acodec":"mp4a.40.2"}
JSON
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	waitForExecutable(t, path)
	return path
}

// waitForExecutable spins up to a second waiting for a freshly written
// script to become exec'able. Without this, Linux can return ETXTBSY
// ("text file busy") when the file's writer fd hasn't been flushed yet.
func waitForExecutable(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if err := exec.Command(path, "--warmup").Run(); err == nil { //nolint:gosec // test-owned helper script.
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake binary at %s did not become executable", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
