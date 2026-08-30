package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validYAML is a complete, valid config used as the base for most cases.
const validYAML = `
log_level: debug
listeners:
  - port: 11434
    services: [ollama]
  - port: 8080
    services: [openai, vllm, anthropic]
rate_limit:
  requests_per_second: 2
  burst: 10
capture:
  queue_size: 500
streaming:
  token_interval_ms: 25
database:
  dsn: postgres://u:p@localhost:5432/nightman
`

// minimalYAML omits every optional field; defaults must fill the gaps.
const minimalYAML = `
listeners:
  - port: 11434
    services: [ollama]
database:
  dsn: postgres://u:p@localhost:5432/nightman
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "nightman.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return p
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		env     map[string]string
		wantErr string // substring; "" means expect success
		check   func(t *testing.T, c *Config)
	}{
		{
			name: "valid full config",
			yaml: validYAML,
			check: func(t *testing.T, c *Config) {
				if c.LogLevel != "debug" {
					t.Errorf("LogLevel = %q, want debug", c.LogLevel)
				}
				if len(c.Listeners) != 2 {
					t.Fatalf("Listeners = %d, want 2", len(c.Listeners))
				}
				if c.Listeners[1].Port != 8080 || len(c.Listeners[1].Services) != 3 {
					t.Errorf("Listeners[1] = %+v, want port 8080 with 3 services", c.Listeners[1])
				}
				if c.RateLimit.RequestsPerSecond != 2 || c.RateLimit.Burst != 10 {
					t.Errorf("RateLimit = %+v", c.RateLimit)
				}
				if c.Capture.QueueSize != 500 {
					t.Errorf("Capture.QueueSize = %d, want 500", c.Capture.QueueSize)
				}
				if c.Streaming.TokenInterval().Milliseconds() != 25 {
					t.Errorf("TokenInterval = %s, want 25ms", c.Streaming.TokenInterval())
				}
			},
		},
		{
			name: "defaults fill optional fields",
			yaml: minimalYAML,
			check: func(t *testing.T, c *Config) {
				if c.LogLevel != "info" {
					t.Errorf("LogLevel = %q, want info", c.LogLevel)
				}
				if c.RateLimit.RequestsPerSecond != 1 || c.RateLimit.Burst != 5 {
					t.Errorf("RateLimit = %+v, want {1 5}", c.RateLimit)
				}
				if c.Capture.QueueSize != 1000 {
					t.Errorf("Capture.QueueSize = %d, want 1000", c.Capture.QueueSize)
				}
				if c.Streaming.TokenIntervalMS != 40 {
					t.Errorf("TokenIntervalMS = %d, want 40", c.Streaming.TokenIntervalMS)
				}
			},
		},
		{
			name: "env overrides file DSN",
			yaml: validYAML,
			env:  map[string]string{"NIGHTMAN_DB_DSN": "postgres://env:secret@db:5432/nightman"},
			check: func(t *testing.T, c *Config) {
				if c.Database.DSN != "postgres://env:secret@db:5432/nightman" {
					t.Errorf("DSN = %q, want the env value", c.Database.DSN)
				}
			},
		},
		{
			name: "env supplies DSN when file omits it",
			yaml: `
listeners:
  - port: 11434
    services: [ollama]
`,
			env: map[string]string{"NIGHTMAN_DB_DSN": "postgres://env@db/nightman"},
			check: func(t *testing.T, c *Config) {
				if c.Database.DSN != "postgres://env@db/nightman" {
					t.Errorf("DSN = %q", c.Database.DSN)
				}
			},
		},
		{
			name: "env overrides log level",
			yaml: validYAML,
			env:  map[string]string{"NIGHTMAN_LOG_LEVEL": "warn"},
			check: func(t *testing.T, c *Config) {
				if c.LogLevel != "warn" {
					t.Errorf("LogLevel = %q, want warn", c.LogLevel)
				}
			},
		},
		{
			name:    "missing DSN",
			yaml:    minimalYAML[:strings.Index(minimalYAML, "database:")],
			wantErr: "database.dsn is empty",
		},
		{
			name:    "no listeners",
			yaml:    "database:\n  dsn: postgres://u:p@localhost/db\n",
			wantErr: "no listeners configured",
		},
		{
			name: "port out of range",
			yaml: `
listeners:
  - port: 70000
    services: [ollama]
database:
  dsn: postgres://u:p@localhost/db
`,
			wantErr: "out of range",
		},
		{
			name: "duplicate port",
			yaml: `
listeners:
  - port: 8080
    services: [openai]
  - port: 8080
    services: [vllm]
database:
  dsn: postgres://u:p@localhost/db
`,
			wantErr: "already used by listeners[0]",
		},
		{
			name: "unknown service",
			yaml: `
listeners:
  - port: 8080
    services: [openai, bogus]
database:
  dsn: postgres://u:p@localhost/db
`,
			wantErr: `unknown service "bogus"`,
		},
		{
			name: "service listed twice",
			yaml: `
listeners:
  - port: 8080
    services: [openai, openai]
database:
  dsn: postgres://u:p@localhost/db
`,
			wantErr: `service "openai" listed twice`,
		},
		{
			name: "empty services list",
			yaml: `
listeners:
  - port: 8080
    services: []
database:
  dsn: postgres://u:p@localhost/db
`,
			wantErr: "no services listed",
		},
		{
			name: "negative rate limit",
			yaml: `
listeners:
  - port: 11434
    services: [ollama]
rate_limit:
  requests_per_second: -1
database:
  dsn: postgres://u:p@localhost/db
`,
			wantErr: "requests_per_second must be > 0",
		},
		{
			name: "invalid log level",
			yaml: `
log_level: verbose
listeners:
  - port: 11434
    services: [ollama]
database:
  dsn: postgres://u:p@localhost/db
`,
			wantErr: `log_level "verbose" is not one of`,
		},
		{
			name:    "unknown key is rejected",
			yaml:    validYAML + "bogus_key: 1\n",
			wantErr: "parsing",
		},
		{
			name:    "malformed yaml",
			yaml:    "listeners: [unclosed\n",
			wantErr: "parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			path := writeConfig(t, tt.yaml)

			got, err := Load(path)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil || !strings.Contains(err.Error(), "reading") {
		t.Fatalf("Load() error = %v, want a 'reading' error", err)
	}
}

func TestLoadEmptyPath(t *testing.T) {
	if _, err := Load(""); err == nil {
		t.Fatal("Load(\"\") error = nil, want error")
	}
}
