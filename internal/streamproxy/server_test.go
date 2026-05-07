package streamproxy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestICYBlock(t *testing.T) {
	t.Parallel()

	block := icyBlock("A Title", "https://example.com")
	if len(block) == 0 {
		t.Fatalf("expected block")
	}
	if (len(block)-1)%16 != 0 {
		t.Fatalf("block should be padded to 16-byte chunks, len=%d", len(block))
	}
	if int(block[0])*16 != len(block)-1 {
		t.Fatalf("length byte mismatch")
	}
	if !bytes.Contains(block, []byte("StreamTitle='A Title';")) {
		t.Fatalf("missing title in %q", string(block))
	}
}

func TestPreflightFailsForMissingFFmpeg(t *testing.T) {
	t.Parallel()

	missingFFmpeg := filepath.Join(t.TempDir(), "missing-ffmpeg")
	srv := NewServer(ServerConfig{FFmpegPath: missingFFmpeg})
	if err := srv.Preflight(context.Background()); err == nil {
		t.Fatalf("expected missing ffmpeg error")
	}
}

func TestHeadHonorsICYMetadataRequest(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{
		Source: Source{URL: "https://example.com/episode.mp3", Title: "Episode"},
	})
	req := httptest.NewRequest(http.MethodHead, "/stream.mp3", nil)
	req.Header.Set("Icy-MetaData", "1")
	rec := httptest.NewRecorder()

	srv.handleStream(rec, req)

	if got := rec.Result().Header.Get("icy-metaint"); got != "8192" {
		t.Fatalf("icy-metaint = %q, want 8192", got)
	}
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{HealthToken: "abc123"})
	req := httptest.NewRequest(http.MethodHead, HealthPath+"?"+HealthTokenQuery+"=abc123", nil)
	rec := httptest.NewRecorder()

	srv.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Result().Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
}

func TestHealthEndpointRejectsWrongToken(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{HealthToken: "abc123"})
	req := httptest.NewRequest(http.MethodHead, HealthPath+"?"+HealthTokenQuery+"=wrong", nil)
	rec := httptest.NewRecorder()

	srv.handleHealth(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHealthEndpointRejectsMethod(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{})
	req := httptest.NewRequest(http.MethodPost, HealthPath, nil)
	rec := httptest.NewRecorder()

	srv.handleHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleStreamCopiesFFmpegOutput(t *testing.T) {
	t.Parallel()

	ffmpeg := fakeFFmpeg(t, "hello stream")
	srv := NewServer(ServerConfig{
		Source:     Source{URL: "https://example.com/episode.mp3", InputURL: "https://example.com/episode.mp3"},
		FFmpegPath: ffmpeg,
	})
	req := httptest.NewRequest(http.MethodGet, "/stream.mp3", nil)
	rec := httptest.NewRecorder()

	srv.handleStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := strings.TrimSuffix(rec.Body.String(), "\n"); got != "hello stream" {
		t.Fatalf("body = %q", got)
	}
}

func TestHandleStreamErrors(t *testing.T) {
	t.Parallel()

	t.Run("method", func(t *testing.T) {
		srv := NewServer(ServerConfig{})
		req := httptest.NewRequest(http.MethodPost, "/stream.mp3", nil)
		rec := httptest.NewRecorder()
		srv.handleStream(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("resolve", func(t *testing.T) {
		srv := NewServer(ServerConfig{
			Source:    Source{URL: "https://example.com/page", UseYTDLP: true},
			YTDLPPath: filepath.Join(t.TempDir(), "missing-yt-dlp"),
		})
		req := httptest.NewRequest(http.MethodGet, "/stream.mp3", nil)
		rec := httptest.NewRecorder()
		srv.handleStream(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
		}
	})

	t.Run("ffmpeg", func(t *testing.T) {
		srv := NewServer(ServerConfig{
			Source:     Source{URL: "https://example.com/episode.mp3", InputURL: "https://example.com/episode.mp3"},
			FFmpegPath: filepath.Join(t.TempDir(), "missing-ffmpeg"),
		})
		req := httptest.NewRequest(http.MethodGet, "/stream.mp3", nil)
		rec := httptest.NewRecorder()
		srv.handleStream(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
		}
	})
}

func TestHandleStreamWritesICYMetadata(t *testing.T) {
	t.Parallel()

	audio := strings.Repeat("a", ICYMetaInterval)
	ffmpeg := fakeFFmpeg(t, audio)
	srv := NewServer(ServerConfig{
		Source:     Source{URL: "https://example.com/episode.mp3", InputURL: "https://example.com/episode.mp3", Title: "Episode"},
		FFmpegPath: ffmpeg,
	})
	req := httptest.NewRequest(http.MethodGet, "/stream.mp3", nil)
	req.Header.Set("Icy-MetaData", "1")
	rec := httptest.NewRecorder()

	srv.handleStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Result().Header.Get("icy-metaint"); got != "8192" {
		t.Fatalf("icy-metaint = %q, want 8192", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("StreamTitle='Episode';")) {
		t.Fatalf("expected ICY metadata in body")
	}
}

func TestWriteRawHeadersSanitizesValues(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&buf))
	srv := NewServer(ServerConfig{
		Source:  Source{URL: "https://example.com/episode.mp3", Title: "Title\r\nInjected: bad", Provider: "Provider"},
		Bitrate: "192k",
	})

	srv.writeRawHeaders(rw, true)

	got := buf.String()
	if !strings.Contains(got, "HTTP/1.0 200 OK\r\n") || !strings.Contains(got, "icy-metaint: 8192\r\n") {
		t.Fatalf("unexpected raw headers: %q", got)
	}
	if strings.Contains(got, "\r\nInjected: bad") {
		t.Fatalf("header injection was not sanitized: %q", got)
	}
}

func TestPreflightSucceedsForFakeFFmpeg(t *testing.T) {
	t.Parallel()

	ffmpeg := fakeFFmpeg(t, "")
	srv := NewServer(ServerConfig{FFmpegPath: ffmpeg})
	if err := srv.Preflight(context.Background()); err != nil {
		t.Fatalf("unexpected preflight error: %v", err)
	}
	if srv.cfg.FFmpegPath != ffmpeg {
		t.Fatalf("ffmpeg path = %q, want %q", srv.cfg.FFmpegPath, ffmpeg)
	}
}

func TestPreflightRejectsUnexpectedVersionOutput(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho nope\n"), 0o700); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}
	srv := NewServer(ServerConfig{FFmpegPath: path})
	if err := srv.Preflight(context.Background()); err == nil {
		t.Fatalf("expected version output error")
	}
}

func TestServeHealthAndShutdown(t *testing.T) {
	t.Parallel()

	ffmpeg := fakeFFmpeg(t, "")
	port := freeTestTCPPort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(ServerConfig{
		Addr:        "127.0.0.1:" + port,
		FFmpegPath:  ffmpeg,
		HealthToken: "token",
		IdleTimeout: time.Second,
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx)
	}()

	healthURL := "http://127.0.0.1:" + port + HealthPath + "?" + HealthTokenQuery + "=token"
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Head(healthURL) //nolint:gosec // local test server URL.
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not stop")
	}
}

func TestServeListenError(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	srv := NewServer(ServerConfig{
		Addr:       ln.Addr().String(),
		FFmpegPath: fakeFFmpeg(t, ""),
	})
	if err := srv.Serve(context.Background()); err == nil {
		t.Fatalf("expected listen error")
	}
}

func TestShutdownWhenDone(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{IdleTimeout: time.Millisecond})
	httpSrv := &http.Server{} //nolint:gosec // test-only server, never listens.
	done := make(chan struct{})
	go func() {
		srv.shutdownWhenDone(context.Background(), httpSrv)
		close(done)
	}()

	srv.once.Do(func() { close(srv.done) })

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("shutdown did not observe done channel")
	}
}

func TestShutdownWhenIdleAfterServed(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{IdleTimeout: time.Millisecond})
	srv.mu.Lock()
	srv.served = true
	srv.mu.Unlock()
	httpSrv := &http.Server{} //nolint:gosec // test-only server, never listens.
	done := make(chan struct{})
	go func() {
		srv.shutdownWhenDone(context.Background(), httpSrv)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("shutdown did not observe idle timeout")
	}
}

func TestShutdownWhenIdleBeforeServed(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{IdleTimeout: time.Millisecond})
	httpSrv := &http.Server{} //nolint:gosec // test-only server, never listens.
	done := make(chan struct{})
	go func() {
		srv.shutdownWhenDone(context.Background(), httpSrv)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("shutdown did not observe idle timeout")
	}
}

func TestNewHealthToken(t *testing.T) {
	t.Parallel()

	token, err := NewHealthToken()
	if err != nil {
		t.Fatalf("unexpected token error: %v", err)
	}
	if len(token) != 32 {
		t.Fatalf("token length = %d, want 32", len(token))
	}
}

func TestWriteICYShortAndWriteError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	natural, err := writeICY(&buf, strings.NewReader("short"), "Title", "https://example.com")
	if err != nil {
		t.Fatalf("unexpected short ICY error: %v", err)
	}
	if !natural || buf.String() != "short" {
		t.Fatalf("natural=%v body=%q", natural, buf.String())
	}

	natural, err = writeICY(errWriter{}, strings.NewReader("short"), "Title", "https://example.com")
	if err == nil {
		t.Fatalf("expected write error")
	}
	if natural {
		t.Fatalf("expected unnatural EOF on write error")
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func fakeFFmpeg(t *testing.T, output string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := `#!/bin/sh
if [ "${1:-}" = "-version" ]; then
  printf '%s\n' 'ffmpeg version test'
  exit 0
fi
cat <<'EOF'
` + output + `
EOF
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if err := exec.Command(path, "-version").Run(); err == nil { //nolint:gosec // test-owned helper script.
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake ffmpeg did not become executable")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return path
}

var freePortMu sync.Mutex

func freeTestTCPPort(t *testing.T) string {
	t.Helper()
	freePortMu.Lock()
	defer freePortMu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	return port
}
