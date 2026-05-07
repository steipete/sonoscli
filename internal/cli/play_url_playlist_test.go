package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/steipete/sonoscli/internal/streamproxy"
)

type fakePlaylistClient struct {
	mu          sync.Mutex
	cleared     bool
	added       []string
	addedMeta   []string
	playedTrack int
}

func (f *fakePlaylistClient) RemoveAllTracksFromQueue(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared = true
	return nil
}

func (f *fakePlaylistClient) AddURIToQueue(ctx context.Context, enqueuedURI, enqueuedMeta string, desiredFirstTrackNumber int, enqueueAsNext bool) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, enqueuedURI)
	f.addedMeta = append(f.addedMeta, enqueuedMeta)
	return len(f.added), nil
}

func (f *fakePlaylistClient) PlayFromQueueTrack(ctx context.Context, oneBasedTrackNumber int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.playedTrack = oneBasedTrackNumber
	return nil
}

func TestPlayURLPlaylistCmdEnqueuesAllTracksAndPlaysFromFirst(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
	cmd := newPlayURLCmd(flags)

	fake := &fakePlaylistClient{}
	var gotCfg streamproxy.ServerConfig

	origEnumerator := playlistEnumerator
	origTarget := newPlaylistTarget
	origLaunch := launchStreamDaemon
	origLocalIP := chooseLocalIP
	t.Cleanup(func() {
		playlistEnumerator = origEnumerator
		newPlaylistTarget = origTarget
		launchStreamDaemon = origLaunch
		chooseLocalIP = origLocalIP
	})

	playlistEnumerator = func(ctx context.Context, ytDLPPath, rawURL string, limit int) ([]playlistTrack, error) {
		return []playlistTrack{
			{ID: "aaa", Title: "Alpha", URL: "https://www.youtube.com/watch?v=aaa", Provider: "YouTube"},
			{ID: "bbb", Title: "Bravo", URL: "https://www.youtube.com/watch?v=bbb", Provider: "YouTube"},
		}, nil
	}
	newPlaylistTarget = func(ctx context.Context, flags *rootFlags) (playlistTarget, error) {
		return playlistTarget{client: fake, ip: "192.168.0.21"}, nil
	}
	chooseLocalIP = func(remoteIP string) (string, error) {
		return "192.168.0.25", nil
	}
	launchStreamDaemon = func(ctx context.Context, cfg streamproxy.ServerConfig, publicURL string) (streamDaemonInfo, error) {
		gotCfg = cfg
		return streamDaemonInfo{PID: 1234, PublicURL: publicURL, LogPath: "/tmp/streamd.log"}, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--port", "8877", "https://music.youtube.com/playlist?list=foo"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fake.cleared {
		t.Fatalf("expected RemoveAllTracksFromQueue to be called")
	}
	if len(fake.added) != 2 {
		t.Fatalf("AddURIToQueue calls = %d, want 2", len(fake.added))
	}
	if fake.added[0] != "http://192.168.0.25:8877/track-001.mp3" || fake.added[1] != "http://192.168.0.25:8877/track-002.mp3" {
		t.Fatalf("unexpected queue URIs: %+v", fake.added)
	}
	if fake.playedTrack != 1 {
		t.Fatalf("playedTrack = %d, want 1", fake.playedTrack)
	}
	if len(gotCfg.Tracks) != 2 {
		t.Fatalf("daemon Tracks = %d, want 2", len(gotCfg.Tracks))
	}
	if gotCfg.Tracks[0].Source.URL != "https://www.youtube.com/watch?v=aaa" {
		t.Fatalf("daemon track 0 source = %q", gotCfg.Tracks[0].Source.URL)
	}
	if !strings.Contains(fake.addedMeta[0], "Alpha") {
		t.Fatalf("track 0 meta missing title: %q", fake.addedMeta[0])
	}
}

func TestPlayURLPlaylistCmdReturnsErrorWhenPlaylistEmpty(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
	cmd := newPlayURLCmd(flags)

	origEnumerator := playlistEnumerator
	t.Cleanup(func() { playlistEnumerator = origEnumerator })
	playlistEnumerator = func(ctx context.Context, ytDLPPath, rawURL string, limit int) ([]playlistTrack, error) {
		return nil, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"https://music.youtube.com/playlist?list=foo"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no playlist items") {
		t.Fatalf("expected empty-playlist error, got %v", err)
	}
}

func TestPlayURLPlaylistCmdReturnsErrorWhenEnumerationFails(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
	cmd := newPlayURLCmd(flags)

	origEnumerator := playlistEnumerator
	t.Cleanup(func() { playlistEnumerator = origEnumerator })
	playlistEnumerator = func(ctx context.Context, ytDLPPath, rawURL string, limit int) ([]playlistTrack, error) {
		return nil, errors.New("boom")
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"https://music.youtube.com/playlist?list=foo"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnumerateYTDLPPlaylistParsesIDsTitlesAndDurations(t *testing.T) {
	t.Parallel()

	ytDLP := writeFakeYTDLPPlaylist(t, `https://music.youtube.com/watch?v=aaa	https://music.youtube.com/watch?v=aaa	Youtube	aaa	191	First Track
https://music.youtube.com/watch?v=bbb	https://music.youtube.com/watch?v=bbb	Youtube	bbb	245.5	Second Track
NA	NA	Youtube	ccc	NA	Third Track
`)

	tracks, err := enumerateYTDLPPlaylist(context.Background(), ytDLP, "https://music.youtube.com/playlist?list=foo", 0)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("len(tracks) = %d, want 3", len(tracks))
	}
	if tracks[0].ID != "aaa" || tracks[0].Title != "First Track" {
		t.Fatalf("tracks[0] = %+v", tracks[0])
	}
	if tracks[0].Duration != 191*time.Second {
		t.Fatalf("tracks[0].Duration = %s, want 3m11s", tracks[0].Duration)
	}
	if tracks[1].Duration != time.Duration(245.5*float64(time.Second)) {
		t.Fatalf("tracks[1].Duration = %s, want 4m05.5s", tracks[1].Duration)
	}
	if tracks[2].Duration != 0 {
		t.Fatalf("tracks[2].Duration = %s, want zero (NA)", tracks[2].Duration)
	}
	if tracks[2].Provider != "Youtube" {
		t.Fatalf("tracks[2].Provider = %q", tracks[2].Provider)
	}
	if tracks[2].URL != "https://www.youtube.com/watch?v=ccc" {
		t.Fatalf("tracks[2].URL = %q", tracks[2].URL)
	}
}

func TestEnumerateYTDLPPlaylistFallsBackToIDWhenTitleMissing(t *testing.T) {
	t.Parallel()

	// Two lines: one with only an id+missing duration, one fully populated.
	ytDLP := writeFakeYTDLPPlaylist(t, "NA\tNA\tYoutube\tloneid\tNA\t\nNA\tNA\tYoutube\tidtwo\t120\tWith Title\n")

	tracks, err := enumerateYTDLPPlaylist(context.Background(), ytDLP, "https://example.com/playlist", 0)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	if tracks[0].Title != "loneid" {
		t.Fatalf("expected fallback title = id, got %q", tracks[0].Title)
	}
	if tracks[1].Title != "With Title" || tracks[1].Duration != 2*time.Minute {
		t.Fatalf("tracks[1] = %+v", tracks[1])
	}
}

func TestEnumerateYTDLPPlaylistUsesGenericWebpageURLs(t *testing.T) {
	t.Parallel()

	ytDLP := writeFakeYTDLPPlaylist(t, "https://example.com/watch/1\tNA\tExample	ex1\t42\tExternal Track\n")

	tracks, err := enumerateYTDLPPlaylist(context.Background(), ytDLP, "https://example.com/playlist", 0)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(tracks))
	}
	if tracks[0].URL != "https://example.com/watch/1" {
		t.Fatalf("URL = %q", tracks[0].URL)
	}
	if tracks[0].Provider != "Example" {
		t.Fatalf("Provider = %q", tracks[0].Provider)
	}
}

func TestEnumerateYTDLPPlaylistErrorsWithoutUsableNonYouTubeURL(t *testing.T) {
	t.Parallel()

	ytDLP := writeFakeYTDLPPlaylist(t, "NA\tlocal-id\tGeneric\tabc\t12\tNo URL\n")

	_, err := enumerateYTDLPPlaylist(context.Background(), ytDLP, "https://example.com/playlist", 0)
	if err == nil || !strings.Contains(err.Error(), "usable URL") {
		t.Fatalf("expected usable URL error, got %v", err)
	}
}

func TestLooksLikePlaylistURLDoesNotAutoDetectYoutuBeVideoLinks(t *testing.T) {
	t.Parallel()

	if looksLikePlaylistURL("https://youtu.be/abc123?list=PL123") {
		t.Fatalf("youtu.be video link with list should stay single-track unless --playlist is set")
	}
	if !looksLikePlaylistURL("https://music.youtube.com/playlist?list=PL123") {
		t.Fatalf("music.youtube.com playlist should auto-detect")
	}
}

func TestEnumerateYTDLPPlaylistReturnsErrorWhenYTDLPFails(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "yt-dlp")
	script := "#!/bin/sh\necho 'ERROR: invalid url' 1>&2\nexit 1\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake: %v", err)
	}

	_, err := enumerateYTDLPPlaylist(context.Background(), path, "https://example.com/x", 0)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func writeFakeYTDLPPlaylist(t *testing.T, output string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "yt-dlp")
	script := "#!/bin/sh\ncat <<'EOF'\n" + output + "EOF\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	return path
}
