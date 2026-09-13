package capture

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSink struct {
	records []Record
}

func (s *fakeSink) Capture(rec Record) {
	s.records = append(s.records, rec)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMiddlewareCapturesOneRecordPerRequest(t *testing.T) {
	sink := &fakeSink{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
	handler := Middleware(sink, discardLogger(), 1024, "ollama")(next)

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"model":"llama3"}`))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("response status = %d, want 201", rr.Code)
	}
	if len(sink.records) != 1 {
		t.Fatalf("sink received %d records, want 1", len(sink.records))
	}

	rec := sink.records[0]
	if rec.Service != "ollama" {
		t.Errorf("Service = %q, want ollama", rec.Service)
	}
	if rec.Method != http.MethodPost || rec.Path != "/api/generate" {
		t.Errorf("Method/Path = %s %s, want POST /api/generate", rec.Method, rec.Path)
	}
	if rec.ResponseStatus != http.StatusCreated {
		t.Errorf("ResponseStatus = %d, want 201", rec.ResponseStatus)
	}
	if rec.RequestID == "" {
		t.Error("RequestID is empty")
	}
	if rec.LatencyMS < 0 {
		t.Errorf("LatencyMS = %d, want >= 0", rec.LatencyMS)
	}
	if rec.ConnectionDurationMS != rec.LatencyMS {
		t.Errorf("ConnectionDurationMS = %d, want equal to LatencyMS (%d) for a non-streaming response",
			rec.ConnectionDurationMS, rec.LatencyMS)
	}
}

func TestMiddlewareDefaultsStatusTo200WhenHandlerOnlyWrites(t *testing.T) {
	sink := &fakeSink{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("implicit 200"))
	})
	handler := Middleware(sink, discardLogger(), 1024, "ollama")(next)

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if len(sink.records) != 1 {
		t.Fatalf("sink received %d records, want 1", len(sink.records))
	}
	if sink.records[0].ResponseStatus != http.StatusOK {
		t.Errorf("ResponseStatus = %d, want 200", sink.records[0].ResponseStatus)
	}
}

func TestMiddlewareServesNormallyAndSkipsCaptureOnRecordError(t *testing.T) {
	sink := &fakeSink{}
	served := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	})
	handler := Middleware(sink, discardLogger(), 1024, "ollama")(next)

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	req.RemoteAddr = "not-a-valid-address" // makes RecordFromRequest fail
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !served {
		t.Error("next handler was not called despite a capture-layer error")
	}
	if len(sink.records) != 0 {
		t.Errorf("sink received %d records, want 0 when the record could not be built", len(sink.records))
	}
	if rr.Code != http.StatusOK {
		t.Errorf("response status = %d, want 200 — a capture failure must not change the response", rr.Code)
	}
}
