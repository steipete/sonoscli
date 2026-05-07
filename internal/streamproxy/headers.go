package streamproxy

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var streamHeaderKeys = []string{
	"Content-Type",
	"Cache-Control",
	"Connection",
	"Content-Length",
	"Accept-Ranges",
	"icy-name",
	"icy-description",
	"icy-genre",
	"icy-br",
	"icy-metaint",
}

func (s *Server) streamHeaders(src Source, icy bool) http.Header {
	h := make(http.Header)
	h.Set("Content-Type", "audio/mpeg")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "close")
	// Finite tracks (e.g. playlist queue items) skip icy-* metadata —
	// Sonos uses those to identify radio stations and refuses to advance
	// the queue when it sees them — and instead expose a Content-Length
	// derived from the bitrate × duration so Sonos treats the response
	// as a finite file and schedules the advance to the next queue entry.
	if src.IsFiniteTrack() {
		if length := estimateMP3Length(s.cfg.Bitrate, src.DurationSeconds); length > 0 {
			h.Set("Content-Length", strconv.FormatInt(length, 10))
			h.Set("Accept-Ranges", "none")
		}
		return h
	}
	h.Set("icy-name", "Sonos CLI")
	h.Set("icy-description", src.DisplayTitle())
	h.Set("icy-genre", src.DisplayProvider())
	h.Set("icy-br", strings.TrimSuffix(s.cfg.Bitrate, "k"))
	if icy {
		h.Set("icy-metaint", fmt.Sprintf("%d", ICYMetaInterval))
	}
	return h
}

// estimateMP3Length computes an approximate Content-Length for an MP3 of the
// given duration encoded at the given libmp3lame bitrate (e.g. "192k"). This
// is intentionally an estimate — ffmpeg's output is close to but not exactly
// bitrate × duration / 8 — and is only used to mark the response as finite
// for downstream players that gate "advance to next track" behaviour on a
// declared length. A return value of 0 means the bitrate could not be parsed
// or the duration was non-positive; callers should then omit Content-Length.
func estimateMP3Length(bitrateSpec string, durationSeconds float64) int64 {
	if durationSeconds <= 0 {
		return 0
	}
	spec := strings.TrimSpace(strings.ToLower(bitrateSpec))
	multiplier := 1
	switch {
	case strings.HasSuffix(spec, "k"):
		multiplier = 1000
		spec = strings.TrimSuffix(spec, "k")
	case strings.HasSuffix(spec, "m"):
		multiplier = 1_000_000
		spec = strings.TrimSuffix(spec, "m")
	}
	bitrate, err := strconv.Atoi(strings.TrimSpace(spec))
	if err != nil || bitrate <= 0 {
		return 0
	}
	bps := int64(bitrate * multiplier)
	return int64(float64(bps) / 8.0 * durationSeconds)
}

func (s *Server) writeRawHeaders(rw *bufio.ReadWriter, src Source, icy bool) {
	_, _ = fmt.Fprint(rw, "HTTP/1.0 200 OK\r\n")
	headers := s.streamHeaders(src, icy)
	for _, key := range streamHeaderKeys {
		for _, value := range headers.Values(key) {
			_, _ = fmt.Fprintf(rw, "%s: %s\r\n", key, sanitizeHeaderValue(value)) //nolint:gosec // header value is CR/LF sanitized.
		}
	}
	_, _ = fmt.Fprint(rw, "\r\n")
	_ = rw.Flush()
}

func (s *Server) writeHeaders(w http.ResponseWriter, src Source, icy bool) {
	for key, values := range s.streamHeaders(src, icy) {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func sanitizeHeaderValue(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}
