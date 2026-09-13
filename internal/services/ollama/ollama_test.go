package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamieKats/nightman/internal/response"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	reg, err := response.Load()
	if err != nil {
		t.Fatalf("response.Load() error = %v", err)
	}
	svc, err := New(reg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return svc
}

// goldenTemplates reads every *.json file straight off disk for one
// endpoint — the same files response.Load embeds — so a test can assert
// a response is byte-for-byte one of the known templates.
func goldenTemplates(t *testing.T, endpoint string) [][]byte {
	t.Helper()
	dir := filepath.Join("..", "..", "response", "templates", "ollama", endpoint)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading golden dir %s: %v", dir, err)
	}
	var out [][]byte
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading golden file: %v", err)
		}
		out = append(out, data)
	}
	return out
}

func assertOneOfGolden(t *testing.T, got []byte, endpoint string) {
	t.Helper()
	for _, g := range goldenTemplates(t, endpoint) {
		if string(got) == string(g) {
			return
		}
	}
	t.Errorf("response body did not match any golden template for %q:\n%s", endpoint, got)
}

func TestRoutes(t *testing.T) {
	mux := newTestService(t).Routes()

	tests := []struct {
		name        string
		method      string
		path        string
		endpoint    string
		wantJSONKey string
	}{
		{"tags", http.MethodGet, "/api/tags", "tags", "models"},
		{"show", http.MethodPost, "/api/show", "show", "modelfile"},
		{"generate", http.MethodPost, "/api/generate", "generate", "response"},
		{"chat", http.MethodPost, "/api/chat", "chat", "message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			body := rr.Body.Bytes()
			var parsed map[string]any
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if _, ok := parsed[tt.wantJSONKey]; !ok {
				t.Errorf("response missing expected key %q: %s", tt.wantJSONKey, body)
			}

			assertOneOfGolden(t, body, tt.endpoint)
		})
	}
}

func TestRoutesRejectWrongMethod(t *testing.T) {
	mux := newTestService(t).Routes()

	req := httptest.NewRequest(http.MethodPost, "/api/tags", nil) // tags is GET-only
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}

func TestRoutesUnknownPathIs404(t *testing.T) {
	mux := newTestService(t).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestName(t *testing.T) {
	if got := newTestService(t).Name(); got != "ollama" {
		t.Errorf("Name() = %q, want ollama", got)
	}
}

func TestResponsesVaryAcrossRequests(t *testing.T) {
	mux := newTestService(t).Routes()

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		seen[rr.Body.String()] = true
	}
	if len(seen) < 2 {
		t.Errorf("saw only %d distinct response(s) over 100 requests for a multi-template pool", len(seen))
	}
}

func TestRequestBodyDoesNotAffectResponse(t *testing.T) {
	mux := newTestService(t).Routes()

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"stream": true, "garbage": "{{{not json`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 regardless of a malformed/unexpected body", rr.Code)
	}
	assertOneOfGolden(t, rr.Body.Bytes(), "generate")
}
