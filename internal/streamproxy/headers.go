package streamproxy

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
)

var streamHeaderKeys = []string{
	"Content-Type",
	"Cache-Control",
	"Connection",
	"icy-name",
	"icy-description",
	"icy-genre",
	"icy-br",
	"icy-metaint",
}

func (s *Server) streamHeaders(icy bool) http.Header {
	h := make(http.Header)
	h.Set("Content-Type", "audio/mpeg")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "close")
	h.Set("icy-name", "Sonos CLI")
	h.Set("icy-description", s.cfg.Source.DisplayTitle())
	h.Set("icy-genre", s.cfg.Source.DisplayProvider())
	h.Set("icy-br", strings.TrimSuffix(s.cfg.Bitrate, "k"))
	if icy {
		h.Set("icy-metaint", fmt.Sprintf("%d", ICYMetaInterval))
	}
	return h
}

func (s *Server) writeRawHeaders(rw *bufio.ReadWriter, icy bool) {
	_, _ = fmt.Fprint(rw, "HTTP/1.0 200 OK\r\n")
	headers := s.streamHeaders(icy)
	for _, key := range streamHeaderKeys {
		for _, value := range headers.Values(key) {
			_, _ = fmt.Fprintf(rw, "%s: %s\r\n", key, sanitizeHeaderValue(value)) //nolint:gosec // header value is CR/LF sanitized.
		}
	}
	_, _ = fmt.Fprint(rw, "\r\n")
	_ = rw.Flush()
}

func (s *Server) writeHeaders(w http.ResponseWriter, icy bool) {
	for key, values := range s.streamHeaders(icy) {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func sanitizeHeaderValue(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}
