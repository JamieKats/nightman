// Command nightman runs the Nightman honeypot: a single Go binary that
// exposes fake, internet-facing LLM-service endpoints (Ollama, OpenAI-
// compatible, vLLM, Anthropic-style) and logs every probe it receives.
//
// No real inference happens anywhere in this process. Every response is
// static or templated. See docs/PROJECT_BRIEF.md for the full design.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JamieKats/nightman/internal/logging"
)

func main() {
	// TODO: level comes from config.Config.LogLevel once config loading is
	// wired in here (Phase 2, step 8). "info" is the config default too.
	logger := logging.New(os.Stdout, "info")

	if err := run(logger); err != nil {
		logger.Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}

// run wires up the honeypot and blocks until the process is signalled to
// shut down. It is kept separate from main so it can return an error and
// so tests can drive it.
func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// TODO: load config, build the store, construct the router with each
	// mocked service registered, and start listeners on the impersonated
	// ports. This stub just serves a placeholder so the binary runs.
	srv := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not implemented", http.StatusNotImplemented)
		}),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("nightman listening", slog.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
