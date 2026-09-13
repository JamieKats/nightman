// Package logging builds Nightman's operational structured logger — JSON
// lines to a writer, filtered by level, with a helper to tag every line
// from one request with a shared request ID. This is the process's own
// log; it is distinct from request *capture* (internal/capture), which
// records the probes themselves.
package logging

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// New builds a JSON slog.Logger writing to w, filtered at level (one of
// "debug", "info", "warn", "error" — see config.Config.LogLevel).
// An unrecognized level falls back to info rather than failing startup.
func New(w io.Writer, level string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)}))
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithRequestID returns a logger that tags every line with id, so one
// probe's log lines can be correlated with each other.
func WithRequestID(l *slog.Logger, id string) *slog.Logger {
	return l.With(slog.String("request_id", id))
}

// NewRequestID returns a short random hex identifier for correlating one
// request's log lines.
func NewRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read practically never fails on a supported OS;
		// fall back rather than returning an empty or duplicate ID.
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
