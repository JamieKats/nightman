// Package ollama mocks an unauthenticated Ollama instance: /api/tags,
// /api/show, /api/generate, /api/chat. Highest expected scan volume —
// Ollama is often exposed with zero auth.
//
// v1 responses are non-streaming only, served byte-for-byte from
// internal/response's template pools regardless of the probe's own
// "stream" field or body content — see docs/IMPLEMENTATION_PLAN.md,
// Phase 4 step 15 for streaming variants.
package ollama

import (
	"net/http"

	"github.com/JamieKats/nightman/internal/response"
	"github.com/JamieKats/nightman/internal/services"
)

const name = "ollama"

var _ services.Service = (*Service)(nil)

// Service mocks the Ollama-compatible API surface.
type Service struct {
	tags     *response.Pool
	show     *response.Pool
	generate *response.Pool
	chat     *response.Pool
}

// New resolves all four endpoints' template pools up front, so a pool
// missing from reg is a construction-time (startup) error rather than a
// surprise against a real probe.
func New(reg *response.Registry) (*Service, error) {
	var s Service
	var err error
	if s.tags, err = reg.Pool("ollama/tags"); err != nil {
		return nil, err
	}
	if s.show, err = reg.Pool("ollama/show"); err != nil {
		return nil, err
	}
	if s.generate, err = reg.Pool("ollama/generate"); err != nil {
		return nil, err
	}
	if s.chat, err = reg.Pool("ollama/chat"); err != nil {
		return nil, err
	}
	return &s, nil
}

// Name identifies this service in captured records.
func (s *Service) Name() string { return name }

// Routes returns the handler for every route this service mocks. Method
// mismatches (e.g. GET on /api/generate) and unknown paths are handled
// by http.ServeMux itself — 405 and 404 respectively.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tags", s.handleTags)
	mux.HandleFunc("POST /api/show", s.handleShow)
	mux.HandleFunc("POST /api/generate", s.handleGenerate)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	return mux
}

func (s *Service) handleTags(w http.ResponseWriter, _ *http.Request)     { respond(w, s.tags) }
func (s *Service) handleShow(w http.ResponseWriter, _ *http.Request)     { respond(w, s.show) }
func (s *Service) handleGenerate(w http.ResponseWriter, _ *http.Request) { respond(w, s.generate) }
func (s *Service) handleChat(w http.ResponseWriter, _ *http.Request)     { respond(w, s.chat) }

// respond writes one of pool's canned templates as the response body. It
// never reads or varies based on the request body — v1 serves every
// template byte-for-byte regardless of what was probed.
func respond(w http.ResponseWriter, pool *response.Pool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pool.Random())
}
