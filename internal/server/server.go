// Package server runs Nightman's HTTP listeners. Each configured port
// gets its own supervised http.Server, bound to the (already composed)
// handler the caller built for it; Run starts all of them and shuts all
// of them down together when its context is cancelled.
//
// Routing individual mocked services onto a shared port, the capture
// middleware, and per-IP rate limiting are layered on top of this in
// later build steps — see docs/architecture/overview.md.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server supervises one http.Server per port.
type Server struct {
	logger  *slog.Logger
	entries []entry
}

type entry struct {
	listener net.Listener
	http     *http.Server
}

// New binds a listener for every port in handlers, pairing each with the
// handler given for that port. Binding happens here, synchronously, so a
// port already in use is reported immediately rather than once Run
// starts; any listeners already bound are closed before returning the
// error. It is an error to pass an empty map.
func New(handlers map[int]http.Handler, logger *slog.Logger) (*Server, error) {
	if len(handlers) == 0 {
		return nil, errors.New("server: no listeners configured")
	}

	s := &Server{logger: logger}
	for port, handler := range handlers {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			s.closeListeners()
			return nil, fmt.Errorf("server: binding port %d: %w", port, err)
		}
		s.entries = append(s.entries, entry{
			listener: ln,
			http: &http.Server{
				Handler:           handler,
				ReadHeaderTimeout: 5 * time.Second,
			},
		})
	}
	return s, nil
}

// Addrs returns the bound address of each listener, in the order New
// received them. Binding port 0 lets the OS assign a free port; Addrs is
// how the caller (or a test) finds out which one it got.
func (s *Server) Addrs() []string {
	addrs := make([]string, len(s.entries))
	for i, e := range s.entries {
		addrs[i] = e.listener.Addr().String()
	}
	return addrs
}

// Run serves every listener until ctx is cancelled, then shuts all of
// them down, allowing up to shutdownTimeout for in-flight requests to
// finish. It returns the first unexpected error encountered while
// serving, if any; failing that, the first shutdown error; otherwise nil.
func (s *Server) Run(ctx context.Context, shutdownTimeout time.Duration) error {
	errCh := make(chan error, len(s.entries))
	for _, e := range s.entries {
		go func(e entry) {
			s.logger.Info("listening", slog.String("addr", e.listener.Addr().String()))
			err := e.http.Serve(e.listener)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("server: serving %s: %w", e.listener.Addr(), err)
				return
			}
			errCh <- nil
		}(e)
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var shutdownErr error
	for _, e := range s.entries {
		if err := e.http.Shutdown(shutdownCtx); err != nil && shutdownErr == nil {
			shutdownErr = fmt.Errorf("server: shutting down %s: %w", e.listener.Addr(), err)
		}
	}

	// Shutdown above makes every Serve call return, so each goroutine is
	// guaranteed to send exactly once — drain them all so none leak.
	var serveErr error
	for range s.entries {
		if err := <-errCh; err != nil && serveErr == nil {
			serveErr = err
		}
	}

	if serveErr != nil {
		return serveErr
	}
	return shutdownErr
}

func (s *Server) closeListeners() {
	for _, e := range s.entries {
		_ = e.listener.Close()
	}
}
