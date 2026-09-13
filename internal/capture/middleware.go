package capture

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/JamieKats/nightman/internal/logging"
)

// Middleware builds a decorator that captures every request passing
// through the handler it wraps: it times the request, builds a Record
// via RecordFromRequest (capped at maxBodyBytes), lets the wrapped
// handler serve the response, then fills in the response-side fields and
// hands the finished Record to sink.
//
// service is static for one mounted handler (e.g. "ollama"), so it is
// supplied here once rather than discovered per request.
//
// If RecordFromRequest itself fails — a genuine body-read error or an
// unparsable RemoteAddr, never something a probe's input controls — the
// request is still served normally; only that one probe goes uncaptured
// (logged as an operational error). A capture-layer problem must never
// change what a probe sees, which could itself be a fingerprinting
// signal.
func Middleware(sink Sink, logger *slog.Logger, maxBodyBytes int64, service string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := logging.NewRequestID()

			rec, err := RecordFromRequest(r, maxBodyBytes)
			captured := err == nil
			if err != nil {
				logging.WithRequestID(logger, reqID).Error("capture: could not build request record", slog.Any("err", err))
			} else {
				rec.RequestID = reqID
				rec.Service = service
			}

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			if !captured {
				return
			}
			rec.ResponseStatus = sw.status
			rec.LatencyMS = time.Since(start).Milliseconds()
			// v1 has no streaming yet, so one request's connection lasts
			// exactly as long as it took to serve; they diverge once
			// streaming (SSE/NDJSON) lands — see docs/plans/response-breadth.md.
			rec.ConnectionDurationMS = rec.LatencyMS

			sink.Capture(rec)
		})
	}
}

// statusWriter wraps http.ResponseWriter to record the status code that
// was actually sent, including the implicit 200 when a handler calls
// Write without ever calling WriteHeader.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
