package logging

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLevelFiltering(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantDebug bool
		wantInfo  bool
	}{
		{"debug shows debug and info", "debug", true, true},
		{"info hides debug, shows info", "info", false, true},
		{"warn hides debug and info", "warn", false, false},
		{"error hides debug and info", "error", false, false},
		{"unknown level defaults to info", "verbose", false, true},
		{"empty level defaults to info", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(&buf, tt.level)

			logger.Debug("debug msg")
			if got := strings.Contains(buf.String(), "debug msg"); got != tt.wantDebug {
				t.Errorf("debug line present = %v, want %v", got, tt.wantDebug)
			}

			buf.Reset()
			logger.Info("info msg")
			if got := strings.Contains(buf.String(), "info msg"); got != tt.wantInfo {
				t.Errorf("info line present = %v, want %v", got, tt.wantInfo)
			}
		})
	}
}

func TestNewEmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "info")
	logger.Info("hello", slog.String("key", "value"))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if line["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", line["msg"])
	}
	if line["key"] != "value" {
		t.Errorf("key = %v, want value", line["key"])
	}
}

func TestWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := WithRequestID(New(&buf, "info"), "abc123")
	logger.Info("probe received")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if line["request_id"] != "abc123" {
		t.Errorf("request_id = %v, want abc123", line["request_id"])
	}
}

func TestNewRequestID(t *testing.T) {
	a := NewRequestID()
	b := NewRequestID()

	if a == b {
		t.Errorf("two calls returned the same ID: %q", a)
	}
	if len(a) != 16 {
		t.Errorf("len(NewRequestID()) = %d, want 16", len(a))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Errorf("NewRequestID() = %q, not valid hex: %v", a, err)
	}
}
