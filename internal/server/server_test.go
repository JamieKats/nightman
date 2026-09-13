package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// freePort reserves a free TCP port by briefly binding to it, then
// releases it so a test's real New() call can bind it instead.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("releasing reserved port: %v", err)
	}
	return port
}

// get issues a GET with a short timeout so a broken server fails the
// test promptly instead of hanging it.
func get(t *testing.T, url string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building request for %s: %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func TestNewRejectsEmptyHandlers(t *testing.T) {
	if _, err := New(map[int]http.Handler{}, discardLogger()); err == nil {
		t.Fatal("New(empty map) error = nil, want error")
	}
}

func TestNewRejectsPortAlreadyInUse(t *testing.T) {
	port := freePort(t)
	blocker, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("reserving port %d: %v", port, err)
	}
	defer func() { _ = blocker.Close() }()

	if _, err := New(map[int]http.Handler{port: http.NewServeMux()}, discardLogger()); err == nil {
		t.Fatal("New() error = nil, want a bind error")
	}
}

func TestNewCleansUpOnPartialFailure(t *testing.T) {
	goodPort := freePort(t)
	badPort := freePort(t)

	blocker, err := net.Listen("tcp", fmt.Sprintf(":%d", badPort))
	if err != nil {
		t.Fatalf("reserving port %d: %v", badPort, err)
	}
	defer func() { _ = blocker.Close() }()

	handlers := map[int]http.Handler{
		goodPort: http.NewServeMux(),
		badPort:  http.NewServeMux(),
	}
	if _, err := New(handlers, discardLogger()); err == nil {
		t.Fatal("New() error = nil, want a bind error")
	}

	// goodPort must have been released by the cleanup path, not leaked.
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", goodPort))
	if err != nil {
		t.Fatalf("port %d still held after New() failed — listener leaked: %v", goodPort, err)
	}
	_ = ln.Close()
}

func TestServerServesAndShutsDownGracefully(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/known", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv, err := New(map[int]http.Handler{0: mux}, discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	addrs := srv.Addrs()
	if len(addrs) != 1 {
		t.Fatalf("Addrs() = %v, want exactly 1 address", addrs)
	}
	base := "http://" + addrs[0]

	ctx, cancel := context.WithCancel(context.Background())
	runErrCh := make(chan error, 1)
	go func() { runErrCh <- srv.Run(ctx, 2*time.Second) }()

	resp := get(t, base+"/known")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /known = %d, want 200", resp.StatusCode)
	}

	resp = get(t, base+"/unknown")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /unknown = %d, want 404", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-runErrCh:
		if err != nil {
			t.Errorf("Run() error = %v, want nil after graceful shutdown", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return after ctx was cancelled")
	}
}

func TestServerMultipleListeners(t *testing.T) {
	portA, portB := freePort(t), freePort(t)

	muxA := http.NewServeMux()
	muxA.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("a")) })
	muxB := http.NewServeMux()
	muxB.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("b")) })

	srv, err := New(map[int]http.Handler{portA: muxA, portB: muxB}, discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErrCh := make(chan error, 1)
	go func() { runErrCh <- srv.Run(ctx, 2*time.Second) }()

	checkBody := func(port int, want string) {
		resp := get(t, fmt.Sprintf("http://127.0.0.1:%d/", port))
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading body from :%d: %v", port, err)
		}
		if string(body) != want {
			t.Errorf("GET :%d body = %q, want %q", port, body, want)
		}
	}
	checkBody(portA, "a")
	checkBody(portB, "b")

	cancel()
	select {
	case err := <-runErrCh:
		if err != nil {
			t.Errorf("Run() error = %v, want nil after graceful shutdown", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return after ctx was cancelled")
	}
}
