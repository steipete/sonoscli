package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
	"github.com/STop211650/sonoscli/internal/spotify"
)

type fakeSpotifySearcher struct {
	results []spotify.Result
	err     error
	calls   int
}

func (f *fakeSpotifySearcher) Search(ctx context.Context, query string, typ spotify.SearchType, limit int, market string) ([]spotify.Result, error) {
	f.calls++
	return f.results, f.err
}

type fakeSonosEnqueuer struct {
	lastInput string
	lastOpts  sonos.EnqueueOptions
	calls     int
	err       error
}

func (f *fakeSonosEnqueuer) EnqueueSpotify(ctx context.Context, input string, opts sonos.EnqueueOptions) (int, error) {
	f.calls++
	f.lastInput = input
	f.lastOpts = opts
	return 1, f.err
}

func execute(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceErrors = true
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestSearchSpotify_NoCredentials(t *testing.T) {
	flags := &rootFlags{Timeout: 2}
	cmd := newSearchSpotifyCmd(flags)

	orig := newSpotifySearcher
	t.Cleanup(func() { newSpotifySearcher = orig })
	newSpotifySearcher = func(flags *rootFlags, clientID, clientSecret string) (spotifySearcher, error) {
		return nil, errors.New("missing SPOTIFY_CLIENT_ID / SPOTIFY_CLIENT_SECRET")
	}

	_, err := execute(t, cmd, "hello")
	if err == nil || !strings.Contains(err.Error(), "missing SPOTIFY_CLIENT_ID") {
		t.Fatalf("expected missing creds error, got: %v", err)
	}
}

func TestSearchSpotify_JSONOutput(t *testing.T) {
	flags := &rootFlags{Timeout: 2, Format: formatJSON}
	cmd := newSearchSpotifyCmd(flags)

	orig := newSpotifySearcher
	t.Cleanup(func() { newSpotifySearcher = orig })
	newSpotifySearcher = func(flags *rootFlags, clientID, clientSecret string) (spotifySearcher, error) {
		return &fakeSpotifySearcher{results: []spotify.Result{
			{Type: spotify.TypeTrack, ID: "t1", URI: "spotify:track:t1", Title: "Song 1"},
		}}, nil
	}

	out, err := execute(t, cmd, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\"uri\": \"spotify:track:t1\"") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestSearchSpotify_TableOutput(t *testing.T) {
	flags := &rootFlags{Timeout: 2}
	cmd := newSearchSpotifyCmd(flags)

	orig := newSpotifySearcher
	t.Cleanup(func() { newSpotifySearcher = orig })
	newSpotifySearcher = func(flags *rootFlags, clientID, clientSecret string) (spotifySearcher, error) {
		return &fakeSpotifySearcher{results: []spotify.Result{
			{Type: spotify.TypeTrack, ID: "t1", URI: "spotify:track:t1", Title: "Song 1", Subtitle: "Artist"},
		}}, nil
	}

	out, err := execute(t, cmd, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "INDEX") || !strings.Contains(out, "spotify:track:t1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestSearchSpotify_OpenRequiresTarget(t *testing.T) {
	flags := &rootFlags{Timeout: 2}
	cmd := newSearchSpotifyCmd(flags)
	_, err := execute(t, cmd, "--open", "hello")
	if err == nil || !strings.Contains(err.Error(), "require --ip or --name") {
		t.Fatalf("expected missing target error, got: %v", err)
	}
}

func TestSearchSpotify_OpenCallsSonos(t *testing.T) {
	flags := &rootFlags{Timeout: 2, Name: "Kitchen"}
	cmd := newSearchSpotifyCmd(flags)

	origS := newSpotifySearcher
	origC := newSonosEnqueuer
	t.Cleanup(func() {
		newSpotifySearcher = origS
		newSonosEnqueuer = origC
	})

	newSpotifySearcher = func(flags *rootFlags, clientID, clientSecret string) (spotifySearcher, error) {
		return &fakeSpotifySearcher{results: []spotify.Result{
			{Type: spotify.TypeTrack, ID: "t1", URI: "spotify:track:t1", Title: "Song 1"},
		}}, nil
	}
	fakeSonos := &fakeSonosEnqueuer{}
	newSonosEnqueuer = func(ctx context.Context, flags *rootFlags) (sonosEnqueuer, error) {
		return fakeSonos, nil
	}

	_, err := execute(t, cmd, "--open", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fakeSonos.calls != 1 {
		t.Fatalf("expected 1 enqueue call, got %d", fakeSonos.calls)
	}
	if fakeSonos.lastInput != "spotify:track:t1" {
		t.Fatalf("unexpected input: %q", fakeSonos.lastInput)
	}
	if !fakeSonos.lastOpts.PlayNow {
		t.Fatalf("expected PlayNow true")
	}
}

func TestSearchSpotify_EnqueueCallsSonos(t *testing.T) {
	flags := &rootFlags{Timeout: 2, IP: "192.168.0.1"}
	cmd := newSearchSpotifyCmd(flags)

	origS := newSpotifySearcher
	origC := newSonosEnqueuer
	t.Cleanup(func() {
		newSpotifySearcher = origS
		newSonosEnqueuer = origC
	})

	newSpotifySearcher = func(flags *rootFlags, clientID, clientSecret string) (spotifySearcher, error) {
		return &fakeSpotifySearcher{results: []spotify.Result{
			{Type: spotify.TypePlaylist, ID: "p1", URI: "spotify:playlist:p1", Title: "Playlist 1"},
		}}, nil
	}
	fakeSonos := &fakeSonosEnqueuer{}
	newSonosEnqueuer = func(ctx context.Context, flags *rootFlags) (sonosEnqueuer, error) {
		return fakeSonos, nil
	}

	_, err := execute(t, cmd, "--enqueue", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fakeSonos.calls != 1 {
		t.Fatalf("expected 1 enqueue call, got %d", fakeSonos.calls)
	}
	if fakeSonos.lastOpts.PlayNow {
		t.Fatalf("expected PlayNow false")
	}
}
