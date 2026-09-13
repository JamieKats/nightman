package capture

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecordFromRequest(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		target       string
		body         string
		headers      map[string]string
		remoteAddr   string // "" keeps httptest's default ("192.0.2.1:1234")
		maxBodyBytes int64
		wantErr      string
		check        func(t *testing.T, rec Record, req *http.Request)
	}{
		{
			name:         "valid JSON body and headers are captured",
			method:       http.MethodPost,
			target:       "/api/generate",
			body:         `{"model":"llama3","prompt":"hi"}`,
			headers:      map[string]string{"Authorization": "Bearer sk-fake", "User-Agent": "curl/8.0"},
			maxBodyBytes: 1024,
			check: func(t *testing.T, rec Record, req *http.Request) {
				if rec.Method != http.MethodPost {
					t.Errorf("Method = %q, want POST", rec.Method)
				}
				if rec.Path != "/api/generate" {
					t.Errorf("Path = %q", rec.Path)
				}
				if string(rec.RawBody) != `{"model":"llama3","prompt":"hi"}` {
					t.Errorf("RawBody = %q", rec.RawBody)
				}
				if rec.Truncated {
					t.Error("Truncated = true, want false")
				}
				parsed, ok := rec.ParsedBody.(map[string]any)
				if !ok || parsed["model"] != "llama3" {
					t.Errorf("ParsedBody = %#v, want model=llama3", rec.ParsedBody)
				}
				if rec.Headers.Get("Authorization") != "Bearer sk-fake" {
					t.Errorf("Authorization header = %q", rec.Headers.Get("Authorization"))
				}
				if rec.SourceIP != "192.0.2.1" || rec.SourcePort != 1234 {
					t.Errorf("source = %s:%d, want 192.0.2.1:1234", rec.SourceIP, rec.SourcePort)
				}
				b, err := io.ReadAll(req.Body)
				if err != nil || string(b) != `{"model":"llama3","prompt":"hi"}` {
					t.Errorf("req.Body after capture = %q, err = %v; body must stay readable downstream", b, err)
				}
			},
		},
		{
			name:         "non-JSON body is captured but not parsed",
			method:       http.MethodPost,
			target:       "/api/chat",
			body:         "not json at all {{{",
			maxBodyBytes: 1024,
			check: func(t *testing.T, rec Record, _ *http.Request) {
				if string(rec.RawBody) != "not json at all {{{" {
					t.Errorf("RawBody = %q", rec.RawBody)
				}
				if rec.ParsedBody != nil {
					t.Errorf("ParsedBody = %#v, want nil", rec.ParsedBody)
				}
				if rec.Truncated {
					t.Error("Truncated = true, want false")
				}
			},
		},
		{
			name:         "empty body",
			method:       http.MethodGet,
			target:       "/api/tags",
			body:         "",
			maxBodyBytes: 1024,
			check: func(t *testing.T, rec Record, _ *http.Request) {
				if len(rec.RawBody) != 0 {
					t.Errorf("RawBody = %q, want empty", rec.RawBody)
				}
				if rec.ParsedBody != nil {
					t.Errorf("ParsedBody = %#v, want nil", rec.ParsedBody)
				}
			},
		},
		{
			name:         "oversized body is truncated, not rejected",
			method:       http.MethodPost,
			target:       "/api/generate",
			body:         strings.Repeat("a", 100),
			maxBodyBytes: 10,
			check: func(t *testing.T, rec Record, req *http.Request) {
				if !rec.Truncated {
					t.Error("Truncated = false, want true")
				}
				if len(rec.RawBody) != 10 {
					t.Errorf("len(RawBody) = %d, want 10", len(rec.RawBody))
				}
				// Downstream still sees exactly the capped body, not an error.
				b, err := io.ReadAll(req.Body)
				if err != nil || len(b) != 10 {
					t.Errorf("req.Body after capture: len=%d err=%v, want len=10", len(b), err)
				}
			},
		},
		{
			name:         "unparsable remote addr is an error",
			method:       http.MethodGet,
			target:       "/api/tags",
			remoteAddr:   "not-a-valid-address",
			maxBodyBytes: 1024,
			wantErr:      "parsing remote addr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}

			rec, err := RecordFromRequest(req, tt.maxBodyBytes)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("RecordFromRequest() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RecordFromRequest() unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, rec, req)
			}
		})
	}
}

func TestParseRemoteAddr(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		wantIP     string
		wantPort   int
		wantErr    bool
	}{
		{"ipv4 with port", "203.0.113.5:54321", "203.0.113.5", 54321, false},
		{"ipv6 with port", "[2001:db8::1]:443", "2001:db8::1", 443, false},
		{"missing port", "203.0.113.5", "", 0, true},
		{"empty string", "", "", 0, true},
		{"non-numeric port", "203.0.113.5:notaport", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, port, err := parseRemoteAddr(tt.remoteAddr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseRemoteAddr(%q) error = nil, want error", tt.remoteAddr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRemoteAddr(%q) unexpected error: %v", tt.remoteAddr, err)
			}
			if ip != tt.wantIP || port != tt.wantPort {
				t.Errorf("parseRemoteAddr(%q) = (%q, %d), want (%q, %d)", tt.remoteAddr, ip, port, tt.wantIP, tt.wantPort)
			}
		})
	}
}
