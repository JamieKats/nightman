package capture

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestStdoutSinkCapture(t *testing.T) {
	var buf bytes.Buffer
	sink := NewStdoutSink(slog.New(slog.NewJSONHandler(&buf, nil)))

	rec := Record{
		RequestID:            "abc123",
		Timestamp:            time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		SourceIP:             "203.0.113.5",
		SourcePort:           5555,
		Service:              "ollama",
		Method:               "POST",
		Path:                 "/api/generate",
		Headers:              map[string][]string{"User-Agent": {"curl/8.0"}},
		RawBody:              []byte(`{"model":"llama3"}`),
		ParsedBody:           map[string]any{"model": "llama3"},
		ResponseStatus:       200,
		LatencyMS:            12,
		ConnectionDurationMS: 12,
	}
	sink.Capture(rec)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	want := map[string]any{
		"request_id":      "abc123",
		"source_ip":       "203.0.113.5",
		"service":         "ollama",
		"method":          "POST",
		"path":            "/api/generate",
		"raw_body":        `{"model":"llama3"}`,
		"truncated":       false,
		"response_status": float64(200),
	}
	for field, want := range want {
		if got := line[field]; got != want {
			t.Errorf("%s = %#v, want %#v", field, got, want)
		}
	}

	body, ok := line["body"].(map[string]any)
	if !ok || body["model"] != "llama3" {
		t.Errorf("body = %#v, want a map with model=llama3", line["body"])
	}
}
