// Package config loads and validates Nightman's runtime configuration:
// which mocked services listen on which ports, per-IP rate limits, the
// PostgreSQL connection string for the capture store, the capture queue
// size, and SSE token-drip timing.
//
// Configuration comes from a YAML file. Secrets are overridden from the
// environment: the DB DSN is read from NIGHTMAN_DB_DSN when set, and the
// log level from NIGHTMAN_LOG_LEVEL. Load applies defaults, then env
// overrides, then validates — an invalid config is a startup error.
package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/goccy/go-yaml"
)

// knownServices is the set of mocked-service names a listener may enable.
// Keep in sync with the sub-packages under internal/services.
var knownServices = []string{"ollama", "openai", "vllm", "anthropic"}

// validLogLevels are the accepted values for Config.LogLevel.
var validLogLevels = []string{"debug", "info", "warn", "error"}

// Config is the fully resolved runtime configuration.
type Config struct {
	LogLevel  string     `yaml:"log_level"`
	Listeners []Listener `yaml:"listeners"`
	RateLimit RateLimit  `yaml:"rate_limit"`
	Capture   Capture    `yaml:"capture"`
	Streaming Streaming  `yaml:"streaming"`
	Database  Database   `yaml:"database"`
}

// Listener binds one TCP port and serves one or more mocked services on
// it (services sharing a port are routed by path).
type Listener struct {
	Port     int      `yaml:"port"`
	Services []string `yaml:"services"`
}

// RateLimit is the per-source-IP token-bucket limit applied to every
// endpoint.
type RateLimit struct {
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
}

// Capture tunes the asynchronous request-capture pipeline.
type Capture struct {
	// QueueSize bounds the in-memory buffer of pending records. When it
	// fills, records are dropped (and counted) rather than blocking the
	// request path.
	QueueSize int `yaml:"queue_size"`
}

// Streaming tunes the fake token-by-token SSE responses.
type Streaming struct {
	// TokenIntervalMS is the delay between emitted tokens, in
	// milliseconds. Zero means emit as fast as possible.
	TokenIntervalMS int `yaml:"token_interval_ms"`
}

// TokenInterval returns TokenIntervalMS as a time.Duration.
func (s Streaming) TokenInterval() time.Duration {
	return time.Duration(s.TokenIntervalMS) * time.Millisecond
}

// Database holds the capture-store connection settings.
type Database struct {
	// DSN is the PostgreSQL connection string for the write/ingest role.
	// It is a secret: prefer setting NIGHTMAN_DB_DSN over writing it here.
	DSN string `yaml:"dsn"`
}

// Load reads, resolves and validates the config file at path.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config: no path provided")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.UnmarshalWithOptions(data, &cfg, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("config: parsing %s: %w", path, err)
	}

	cfg.applyEnvOverrides()
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return &cfg, nil
}

// applyEnvOverrides lets NIGHTMAN_* environment variables win over the
// file, so secrets stay out of version control.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("NIGHTMAN_DB_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("NIGHTMAN_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
}

// applyDefaults fills zero-valued fields with sensible defaults. It never
// overrides a value the operator set explicitly (including an invalid
// one — that is Validate's job to reject).
func (c *Config) applyDefaults() {
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.RateLimit.RequestsPerSecond == 0 {
		c.RateLimit.RequestsPerSecond = 1
	}
	if c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = 5
	}
	if c.Capture.QueueSize == 0 {
		c.Capture.QueueSize = 1000
	}
	if c.Streaming.TokenIntervalMS == 0 {
		c.Streaming.TokenIntervalMS = 40
	}
}

// Validate reports the first problem that would stop Nightman starting
// cleanly. Errors are phrased to point at the offending config key.
func (c *Config) Validate() error {
	if !slices.Contains(validLogLevels, c.LogLevel) {
		return fmt.Errorf("log_level %q is not one of %v", c.LogLevel, validLogLevels)
	}

	if len(c.Listeners) == 0 {
		return errors.New("no listeners configured")
	}
	usedPorts := make(map[int]int, len(c.Listeners))
	for i, l := range c.Listeners {
		if l.Port < 1 || l.Port > 65535 {
			return fmt.Errorf("listeners[%d]: port %d out of range 1-65535", i, l.Port)
		}
		if prev, ok := usedPorts[l.Port]; ok {
			return fmt.Errorf("listeners[%d]: port %d already used by listeners[%d]", i, l.Port, prev)
		}
		usedPorts[l.Port] = i

		if len(l.Services) == 0 {
			return fmt.Errorf("listeners[%d] (port %d): no services listed", i, l.Port)
		}
		seen := make(map[string]bool, len(l.Services))
		for _, name := range l.Services {
			if !slices.Contains(knownServices, name) {
				return fmt.Errorf("listeners[%d] (port %d): unknown service %q (known: %v)", i, l.Port, name, knownServices)
			}
			if seen[name] {
				return fmt.Errorf("listeners[%d] (port %d): service %q listed twice", i, l.Port, name)
			}
			seen[name] = true
		}
	}

	if c.RateLimit.RequestsPerSecond <= 0 {
		return fmt.Errorf("rate_limit.requests_per_second must be > 0, got %g", c.RateLimit.RequestsPerSecond)
	}
	if c.RateLimit.Burst < 1 {
		return fmt.Errorf("rate_limit.burst must be >= 1, got %d", c.RateLimit.Burst)
	}
	if c.Capture.QueueSize < 1 {
		return fmt.Errorf("capture.queue_size must be >= 1, got %d", c.Capture.QueueSize)
	}
	if c.Streaming.TokenIntervalMS < 0 {
		return fmt.Errorf("streaming.token_interval_ms must be >= 0, got %d", c.Streaming.TokenIntervalMS)
	}
	if c.Database.DSN == "" {
		return errors.New("database.dsn is empty (set it in the file or via NIGHTMAN_DB_DSN)")
	}
	return nil
}
