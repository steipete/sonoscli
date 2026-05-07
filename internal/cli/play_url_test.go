package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steipete/sonoscli/internal/streamproxy"
)

func TestPlayURLCmdStartsDaemonAndPlaysProxy(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
	cmd := newPlayURLCmd(flags)

	fake := &fakeSourceClient{}
	var gotCfg streamproxy.ServerConfig
	var gotPublicURL string

	origTarget := newPlayURLTarget
	origLaunch := launchStreamDaemon
	origLocalIP := chooseLocalIP
	t.Cleanup(func() {
		newPlayURLTarget = origTarget
		launchStreamDaemon = origLaunch
		chooseLocalIP = origLocalIP
	})
	newPlayURLTarget = func(ctx context.Context, flags *rootFlags) (playURLTarget, error) {
		return playURLTarget{client: fake, ip: "192.168.0.21"}, nil
	}
	chooseLocalIP = func(remoteIP string) (string, error) {
		return "192.168.0.25", nil
	}
	launchStreamDaemon = func(ctx context.Context, cfg streamproxy.ServerConfig, publicURL string) (streamDaemonInfo, error) {
		gotCfg = cfg
		gotPublicURL = publicURL
		if cfg.Path != "/Sonos%20CLI.mp3" {
			t.Fatalf("unexpected proxy path: %q", cfg.Path)
		}
		return streamDaemonInfo{PID: 1234, PublicURL: publicURL, LogPath: "/tmp/streamd.log"}, nil
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--resolver", "direct", "--port", "8877", "https://example.com/episode.mp3"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPublicURL != "http://192.168.0.25:8877/Sonos%20CLI.mp3" {
		t.Fatalf("unexpected public URL: %q", gotPublicURL)
	}
	if gotCfg.Source.URL != "https://example.com/episode.mp3" {
		t.Fatalf("unexpected source: %+v", gotCfg.Source)
	}
	if fake.uri != "http://192.168.0.25:8877/Sonos%20CLI.mp3" {
		t.Fatalf("unexpected proxy URI: %q", fake.uri)
	}
	if !strings.Contains(fake.meta, "Sonos CLI") {
		t.Fatalf("expected Sonos CLI metadata, got %q", fake.meta)
	}
	if fake.playCalls != 1 {
		t.Fatalf("expected play once, got %d", fake.playCalls)
	}
}

func TestLaunchStreamProxyDaemonFailsBeforeSpawnForMissingFFmpeg(t *testing.T) {
	t.Parallel()

	missingFFmpeg := filepath.Join(t.TempDir(), "missing-ffmpeg")
	_, err := launchStreamProxyDaemon(context.Background(), streamproxy.ServerConfig{
		Source:     streamproxy.Source{URL: "https://example.com/episode.mp3"},
		FFmpegPath: missingFFmpeg,
	}, "http://127.0.0.1:1/Sonos%20CLI.mp3")
	if err == nil {
		t.Fatalf("expected missing ffmpeg error")
	}
	if !strings.Contains(err.Error(), "ffmpeg preflight failed") {
		t.Fatalf("expected ffmpeg preflight error, got %v", err)
	}
}

func TestStreamProxyHealthURL(t *testing.T) {
	t.Parallel()

	got, err := streamProxyHealthURL("http://192.168.0.25:8877/Sonos%20CLI.mp3", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://192.168.0.25:8877/healthz?token=abc123" {
		t.Fatalf("health URL = %q", got)
	}
}

func TestWaitForStreamProxyDetectsChildExitDespiteHealthyOldDaemon(t *testing.T) {
	t.Parallel()

	oldDaemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(oldDaemon.Close)

	procExited := make(chan error, 1)
	procExited <- errors.New("listen tcp: bind: address already in use")

	err := waitForStreamProxy(context.Background(), oldDaemon.URL, procExited, time.Second)
	if err == nil {
		t.Fatalf("expected child exit error")
	}
	if !strings.Contains(err.Error(), "stream proxy exited before readiness") {
		t.Fatalf("unexpected error: %v", err)
	}
}
