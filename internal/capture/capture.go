// Package capture records everything about an incoming probe: source IP
// and port, method, path, full headers (esp. Authorization, User-Agent,
// X-Api-Key), raw and parsed body, timestamp, response status/latency,
// and connection duration. Middleware wraps an http.Handler to build one
// Record per request and hand it to a Sink — v1's is StdoutSink; the
// Postgres-backed one lands in Phase 3.
package capture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Record is the full audit trail for one probe, matching the `requests`
// table (see migrations/0001_init.sql).
//
// RecordFromRequest fills in everything knowable from the incoming
// request. Service and the response-side fields are filled in by the
// caller (the capture middleware, internal/server) once the matched
// service and the response are known.
type Record struct {
	// Request-side, set by RecordFromRequest.
	Timestamp  time.Time
	SourceIP   string
	SourcePort int
	Method     string
	Path       string
	Headers    http.Header
	RawBody    []byte
	ParsedBody any  // nil if the body was empty or not valid JSON
	Truncated  bool // true if RawBody was capped at maxBodyBytes

	// RequestID correlates this record with the operational log lines
	// for the same request (see internal/logging). Set by Middleware.
	RequestID string

	// Service is static per mounted handler, so Middleware is given it
	// once at construction rather than it being inferred per request.
	Service string

	// Response-side, set by Middleware after the wrapped handler has run.
	ResponseStatus       int
	LatencyMS            int64
	ConnectionDurationMS int64
}

// RecordFromRequest extracts everything capturable from an incoming
// request: source address, method, path, headers, and up to maxBodyBytes
// of the body (kept raw, and parsed as JSON when it is valid JSON — a
// probe sending garbage is still logged, just without ParsedBody).
//
// It never fails on malformed or oversized input — only on a genuine
// read error or an unparsable RemoteAddr, neither of which untrusted
// request data controls. The body is re-buffered onto r.Body so a
// handler further down the chain can still read it in full (or in
// truncated form, if it was capped).
func RecordFromRequest(r *http.Request, maxBodyBytes int64) (Record, error) {
	ip, port, err := parseRemoteAddr(r.RemoteAddr)
	if err != nil {
		return Record{}, fmt.Errorf("capture: parsing remote addr %q: %w", r.RemoteAddr, err)
	}

	raw, truncated, err := readLimitedBody(r.Body, maxBodyBytes)
	if err != nil {
		return Record{}, fmt.Errorf("capture: reading body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	return Record{
		Timestamp:  time.Now().UTC(),
		SourceIP:   ip,
		SourcePort: port,
		Method:     r.Method,
		Path:       r.URL.Path,
		Headers:    r.Header.Clone(),
		RawBody:    raw,
		ParsedBody: parseJSONBody(raw),
		Truncated:  truncated,
	}, nil
}

// parseRemoteAddr splits an http.Request.RemoteAddr ("host:port", IPv6
// hosts bracketed) into its IP and port.
func parseRemoteAddr(remoteAddr string) (ip string, port int, err error) {
	host, portStr, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return "", 0, err
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	return host, port, nil
}

// readLimitedBody reads up to maxBytes from body. If there was more to
// read, it reports truncated=true and returns exactly maxBytes — it
// never returns an error just because the body was too big.
func readLimitedBody(body io.Reader, maxBytes int64) (raw []byte, truncated bool, err error) {
	raw, err = io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(raw)) > maxBytes {
		return raw[:maxBytes], true, nil
	}
	return raw, false, nil
}

// parseJSONBody returns raw decoded as JSON, or nil if raw is empty or
// isn't valid JSON. A probe's body not being JSON is expected and not an
// error — RawBody still holds whatever was sent.
func parseJSONBody(raw []byte) any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	return parsed
}
