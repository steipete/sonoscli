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
	cfg    ServerConfig
	mu     sync.Mutex
	active int
	served bool
	done   chan struct{}
	once   sync.Once
}

func NewServer(cfg ServerConfig) *Server {
	cfg = cfg.withDefaults()
	return &Server{
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

func (s *Server) Serve(ctx context.Context) error {
	if err := s.Preflight(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(s.cfg.Path, s.handleStream)
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
	log.Printf("stream proxy listening on %s path=%s source=%q title=%q provider=%q", ln.Addr(), s.cfg.Path, s.cfg.Source.URL, s.cfg.Source.DisplayTitle(), s.cfg.Source.DisplayProvider())

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
	lastIdle := time.Now()
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
			served := s.served
			s.mu.Unlock()
			if active > 0 {
				lastIdle = time.Now()
				continue
			}
			if served && time.Since(lastIdle) >= s.cfg.IdleTimeout {
				shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
				_ = srv.Shutdown(shutdownCtx)
				cancel()
				return
			}
			if !served && time.Since(lastIdle) >= s.cfg.IdleTimeout {
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
	wantICY := requestWantsICY(r)
	if r.Method == http.MethodHead {
		s.writeHeaders(w, wantICY)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.active++
	s.served = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
	}()

	var ytCmd *exec.Cmd
	var cmd *exec.Cmd
	if s.cfg.Source.UseYTDLP {
		log.Printf("client %q requested stream; piping yt-dlp source=%q", r.RemoteAddr, s.cfg.Source.URL) //nolint:gosec // diagnostic log only; values are quoted.
		ytCmd = s.ytDLPDownloadCommand(r.Context(), s.cfg.Source.URL)
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
		streamURL := strings.TrimSpace(s.cfg.Source.InputURL)
		if streamURL == "" {
			streamURL = strings.TrimSpace(s.cfg.Source.URL)
		}
		log.Printf("client %q requested stream; resolved input=%q", r.RemoteAddr, streamURL) //nolint:gosec // diagnostic log only; values are quoted.
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
		s.writeRawHeaders(rw, wantICY)
		writer = conn
	} else {
		s.writeHeaders(w, wantICY)
	}

	var naturalEOF bool
	var copyErr error
	if wantICY {
		naturalEOF, copyErr = writeICY(writer, stdout, s.cfg.Source.DisplayTitle(), s.cfg.Source.URL)
	} else {
		_, copyErr = io.Copy(writer, stdout)
		naturalEOF = copyErr == nil
	}
	if copyErr != nil {
		log.Printf("client %q stream copy failed: %v", r.RemoteAddr, copyErr) //nolint:gosec // diagnostic log only; values are quoted.
	}
	log.Printf("client %q stream ended naturalEOF=%v", r.RemoteAddr, naturalEOF) //nolint:gosec // diagnostic log only; values are quoted.
	if naturalEOF {
		s.once.Do(func() { close(s.done) })
	}
}

func requestWantsICY(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("Icy-MetaData")) == "1"
}
