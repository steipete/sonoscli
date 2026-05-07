package streamproxy

import "testing"

func TestStreamHeadersFiniteTrackOmitsICYAndContentLength(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{Bitrate: "192k"})
	src := Source{URL: "https://example.com/track", Title: "Track", Provider: "YouTube", DurationSeconds: 60}

	h := srv.streamHeaders(src, true /* icy */)

	if h.Get("icy-name") != "" || h.Get("icy-br") != "" || h.Get("icy-metaint") != "" {
		t.Fatalf("finite track should not emit icy-* headers, got %+v", h)
	}
	if got := h.Get("Content-Length"); got != "" {
		t.Fatalf("finite transcoded track should not declare estimated Content-Length, got %q", got)
	}
	if got := h.Get("Accept-Ranges"); got != "none" {
		t.Fatalf("Accept-Ranges = %q, want %q", got, "none")
	}
	if got := h.Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestStreamHeadersNonFiniteTrackKeepsICY(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{Bitrate: "256k"})
	src := Source{URL: "https://example.com/live.m3u8", Title: "Live", Provider: "Internet"}

	h := srv.streamHeaders(src, true /* icy */)

	if h.Get("icy-name") == "" {
		t.Fatalf("expected icy-name to be set for non-finite track")
	}
	if h.Get("icy-br") != "256" {
		t.Fatalf("icy-br = %q, want 256", h.Get("icy-br"))
	}
	if h.Get("icy-metaint") == "" {
		t.Fatalf("expected icy-metaint when icy=true")
	}
	if h.Get("Content-Length") != "" {
		t.Fatalf("non-finite track should not declare Content-Length, got %q", h.Get("Content-Length"))
	}
}
