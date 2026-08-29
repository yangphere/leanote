package httpserver

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"
)

// DefaultShutdownTimeout is the graceful-stop bound when
// http.shutdownTimeoutMs is absent from app.conf.
const DefaultShutdownTimeout = 30 * time.Second

// Server owns the listener lifecycle for the first-party HTTP stack.
type Server struct {
	http            *http.Server
	shutdownTimeout time.Duration
}

// NewServer assembles the listener; shutdownTimeout bounds how long a
// graceful stop waits for in-flight requests.
func NewServer(addr string, handler http.Handler, shutdownTimeout time.Duration) *Server {
	if shutdownTimeout <= 0 {
		shutdownTimeout = DefaultShutdownTimeout
	}
	return &Server{
		http: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 30 * time.Second,
		},
		shutdownTimeout: shutdownTimeout,
	}
}

// Run serves until a signal arrives on signals or stop is closed, then
// shuts down gracefully within the bound. A shutdown timeout returns a
// non-nil error (design §6); a clean stop returns nil. A startup failure
// (e.g. port busy) is never masked by a concurrent shutdown trigger.
func (s *Server) Run(signals <-chan os.Signal, stop <-chan struct{}) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.http.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-signals:
	case <-stop:
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	if err := s.http.Shutdown(ctx); err != nil {
		return err
	}
	err := <-serveErr
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// ShutdownTimeout resolves http.shutdownTimeoutMs from config.
func ShutdownTimeout(cfg *Config) time.Duration {
	ms := cfg.IntDefault("http.shutdownTimeoutMs", int(DefaultShutdownTimeout/time.Millisecond))
	if ms <= 0 {
		return DefaultShutdownTimeout
	}
	return time.Duration(ms) * time.Millisecond
}
