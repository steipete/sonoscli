package streamproxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Server struct {
	cfg        ServerConfig
	mu         sync.Mutex
	active     int
	served     bool
	lastActive time.Time
	completed  map[string]bool
	done       chan struct{}
	once       sync.Once
}

func NewServer(cfg ServerConfig) *Server {
	cfg = cfg.withDefaults()
	return &Server{
		cfg:        cfg,
		lastActive: time.Now(),
		completed:  make(map[string]bool),
		done:       make(chan struct{}),
	}
}

// allTracksCompleted reports whether every configured track has been served
// to natural EOF at least once. Callers must hold s.mu.
func (s *Server) allTracksCompleted() bool {
	if len(s.cfg.Tracks) == 0 {
		return false
	}
	for _, track := range s.cfg.Tracks {
		if !s.completed[track.Path] {
			return false
		}
	}
	return true
}

func (s *Server) Serve(ctx context.Context) error {
	if err := s.Preflight(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	for i, track := range s.cfg.Tracks {
		mux.HandleFunc(track.Path, func(w http.ResponseWriter, r *http.Request) {
			s.handleTrackStream(w, r, track)
		})
		log.Printf("stream proxy track %d path=%s source=%q title=%q provider=%q", i+1, track.Path, track.Source.URL, track.Source.DisplayTitle(), track.Source.DisplayProvider())
	}
	mux.HandleFunc(HealthPath, s.handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()
	log.Printf("stream proxy listening on %s tracks=%d", ln.Addr(), len(s.cfg.Tracks))

	go s.shutdownWhenDone(ctx, srv)

	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Preflight(ctx context.Context) error {
	cfg, err := s.cfg.Normalize(ctx)
	if err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}

func (s *Server) shutdownWhenDone(ctx context.Context, srv *http.Server) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			_ = srv.Shutdown(shutdownCtx)
			cancel()
			return
		case <-s.done:
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			_ = srv.Shutdown(shutdownCtx)
			cancel()
			return
		case <-ticker.C:
			s.mu.Lock()
			active := s.active
			multi := s.cfg.MultiTrack()
			allDone := s.allTracksCompleted()
			idleFor := time.Since(s.lastActive)
			s.mu.Unlock()
			if active > 0 {
				continue
			}
			// In multi-track mode, never shut down until every configured
			// track has been delivered to natural EOF at least once. Sonos
			// pre-fetches upcoming queue items briefly to determine size
			// and disconnects, then refetches them later when it actually
			// plays them — if we shut down between those events, Sonos
			// gets a connection refused on the second fetch and stops. Bound
			// that grace so stopped/abandoned playlists are still reaped.
			if multi && !allDone && idleFor < s.cfg.incompletePlaylistIdleTimeout() {
				continue
			}
			if idleFor >= s.cfg.IdleTimeout {
				shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
				_ = srv.Shutdown(shutdownCtx)
				cancel()
				return
			}
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodHead && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if token := strings.TrimSpace(s.cfg.HealthToken); token != "" && r.URL.Query().Get(HealthTokenQuery) != token {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	s.handleTrackStream(w, r, Track{Path: s.cfg.Path, Source: s.cfg.Source})
}

func (s *Server) handleTrackStream(w http.ResponseWriter, r *http.Request, track Track) {
	wantICY := requestWantsICY(r)
	streamICY := wantICY && !track.Source.IsFiniteTrack()
	if r.Method == http.MethodHead {
		s.writeHeaders(w, track.Source, streamICY)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.active++
	s.served = true
	s.lastActive = time.Now()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.active--
		s.lastActive = time.Now()
		s.mu.Unlock()
	}()

	var ytCmd *exec.Cmd
	var cmd *exec.Cmd
	if track.Source.UseYTDLP {
		log.Printf("client %q requested %s; piping yt-dlp source=%q", r.RemoteAddr, track.Path, track.Source.URL) //nolint:gosec // diagnostic log only; values are quoted.
		ytCmd = s.ytDLPDownloadCommand(r.Context(), track.Source.URL)
		ytStdout, err := ytCmd.StdoutPipe()
		if err != nil {
			log.Printf("yt-dlp stdout pipe failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ytCmd.Stderr = os.Stderr
		if err := ytCmd.Start(); err != nil {
			log.Printf("yt-dlp start failed: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		cmd = s.ffmpegStdinCommand(r.Context())
		cmd.Stdin = ytStdout
	} else {
		streamURL := strings.TrimSpace(track.Source.InputURL)
		if streamURL == "" {
			streamURL = strings.TrimSpace(track.Source.URL)
		}
		log.Printf("client %q requested %s; resolved input=%q", r.RemoteAddr, track.Path, streamURL) //nolint:gosec // diagnostic log only; values are quoted.
		cmd = s.ffmpegCommand(r.Context(), streamURL)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if ytCmd != nil {
			_ = ytCmd.Process.Kill()
			_ = ytCmd.Wait()
		}
		log.Printf("ffmpeg stdout pipe failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		if ytCmd != nil {
			_ = ytCmd.Process.Kill()
			_ = ytCmd.Wait()
		}
		log.Printf("ffmpeg start failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if ytCmd != nil {
			_ = ytCmd.Process.Kill()
			_ = ytCmd.Wait()
		}
	}()

	writer := io.Writer(w)
	if hj, ok := w.(http.Hijacker); ok {
		conn, rw, err := hj.Hijack()
		if err != nil {
			log.Printf("http hijack failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer func() { _ = conn.Close() }()
		s.writeRawHeaders(rw, track.Source, streamICY)
		writer = conn
	} else {
		s.writeHeaders(w, track.Source, streamICY)
	}

	var naturalEOF bool
	var copyErr error
	if streamICY {
		naturalEOF, copyErr = writeICY(writer, stdout, track.Source.DisplayTitle(), track.Source.URL)
	} else {
		_, copyErr = io.Copy(writer, stdout)
		naturalEOF = copyErr == nil
	}
	if copyErr != nil {
		log.Printf("client %q stream copy failed: %v", r.RemoteAddr, copyErr) //nolint:gosec // diagnostic log only; values are quoted.
	}
	log.Printf("client %q stream ended path=%s naturalEOF=%v", r.RemoteAddr, track.Path, naturalEOF) //nolint:gosec // diagnostic log only; values are quoted.
	if naturalEOF {
		s.mu.Lock()
		s.completed[track.Path] = true
		s.mu.Unlock()
		// Single-track mode keeps its fast-shutdown behaviour: once the
		// only source has been delivered in full, signal the daemon to
		// exit immediately. Multi-track mode relies on the idle-timeout
		// loop in shutdownWhenDone, which waits until every configured
		// track has completed before honouring the idle timer.
		if !s.cfg.MultiTrack() {
			s.once.Do(func() { close(s.done) })
		}
	}
}

func requestWantsICY(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("Icy-MetaData")) == "1"
}
