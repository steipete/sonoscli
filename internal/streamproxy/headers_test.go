package streamproxy

import (
	"testing"
)

func TestEstimateMP3LengthHandlesCommonBitrateFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		bitrate  string
		duration float64
		want     int64
	}{
		{name: "kbps with k suffix", bitrate: "192k", duration: 60, want: 1_440_000},
		{name: "kbps as bare number", bitrate: "192000", duration: 60, want: 1_440_000},
		{name: "mbps with m suffix", bitrate: "1m", duration: 8, want: 1_000_000},
		{name: "fractional duration", bitrate: "192k", duration: 1.5, want: 36_000},
		{name: "zero duration", bitrate: "192k", duration: 0, want: 0},
		{name: "negative duration", bitrate: "192k", duration: -1, want: 0},
		{name: "garbage bitrate", bitrate: "abc", duration: 10, want: 0},
		{name: "empty bitrate", bitrate: "", duration: 10, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := estimateMP3Length(tc.bitrate, tc.duration)
			if got != tc.want {
				t.Fatalf("estimateMP3Length(%q, %v) = %d, want %d", tc.bitrate, tc.duration, got, tc.want)
			}
		})
	}
}

func TestStreamHeadersFiniteTrackOmitsICYAndSetsContentLength(t *testing.T) {
	t.Parallel()

	srv := NewServer(ServerConfig{Bitrate: "192k"})
	src := Source{URL: "https://example.com/track", Title: "Track", Provider: "YouTube", DurationSeconds: 60}

	h := srv.streamHeaders(src, true /* icy */)

	if h.Get("icy-name") != "" || h.Get("icy-br") != "" || h.Get("icy-metaint") != "" {
		t.Fatalf("finite track should not emit icy-* headers, got %+v", h)
	}
	if got := h.Get("Content-Length"); got != "1440000" {
		t.Fatalf("Content-Length = %q, want %q", got, "1440000")
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
