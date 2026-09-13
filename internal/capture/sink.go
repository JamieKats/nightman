package capture

import "log/slog"

// Sink receives a finished Record after a request has been fully
// handled. Implementations must not block the request path: v1's
// StdoutSink just logs; the future Postgres-backed sink batches writes
// asynchronously and drops records under backpressure rather than
// stalling a caller (see docs/IMPLEMENTATION_PLAN.md, Phase 3).
type Sink interface {
	Capture(rec Record)
}

// StdoutSink logs every Record as one structured JSON line. It is the v1
// sink; the real Postgres-backed sink lands in Phase 3.
type StdoutSink struct {
	logger *slog.Logger
}

// NewStdoutSink builds a StdoutSink that logs through logger.
func NewStdoutSink(logger *slog.Logger) *StdoutSink {
	return &StdoutSink{logger: logger}
}

// Capture logs rec as one line, mirroring the columns of the `requests`
// table (see migrations/0001_init.sql) so the two stay easy to compare.
func (s *StdoutSink) Capture(rec Record) {
	s.logger.Info("probe captured",
		slog.String("request_id", rec.RequestID),
		slog.Time("ts", rec.Timestamp),
		slog.String("source_ip", rec.SourceIP),
		slog.Int("source_port", rec.SourcePort),
		slog.String("service", rec.Service),
		slog.String("method", rec.Method),
		slog.String("path", rec.Path),
		slog.Any("headers", rec.Headers),
		slog.String("raw_body", string(rec.RawBody)),
		slog.Any("body", rec.ParsedBody),
		slog.Bool("truncated", rec.Truncated),
		slog.Int("response_status", rec.ResponseStatus),
		slog.Int64("latency_ms", rec.LatencyMS),
		slog.Int64("connection_duration_ms", rec.ConnectionDurationMS),
	)
}
