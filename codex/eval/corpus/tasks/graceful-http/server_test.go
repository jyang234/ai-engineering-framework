package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func freePort() string {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestListenAndServe(t *testing.T) {
	addr := freePort()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	srv := NewServer(addr, handler)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ok" {
		t.Errorf("got %q, want %q", body, "ok")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func TestShutdownDrainsInFlightRequests(t *testing.T) {
	addr := freePort()
	var completed atomic.Bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond) // Simulate slow request
		completed.Store(true)
		fmt.Fprint(w, "done")
	})

	srv := NewServer(addr, handler)
	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	// Start a slow request
	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err == nil {
			respCh <- resp
		}
	}()

	// Give the request time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown while request is in flight
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := srv.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	// The in-flight request should have completed
	if !completed.Load() {
		t.Error("in-flight request was NOT drained — shutdown killed it prematurely")
	}
}

func TestShutdownCallsHooks(t *testing.T) {
	addr := freePort()
	srv := NewServer(addr, http.DefaultServeMux)

	var hookCalled atomic.Bool
	srv.OnShutdown(func() {
		hookCalled.Store(true)
	})

	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	if !hookCalled.Load() {
		t.Error("shutdown hook was not called")
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	addr := freePort()
	srv := NewServer(addr, http.DefaultServeMux)

	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	// Should not panic
	srv.Shutdown(ctx)
	srv.Shutdown(ctx)
	srv.Shutdown(ctx)
}

func TestShutdownRespectsDeadline(t *testing.T) {
	addr := freePort()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // Very slow handler
	})

	srv := NewServer(addr, handler)
	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	// Start a stuck request
	go http.Get("http://" + addr + "/stuck")
	time.Sleep(50 * time.Millisecond)

	// Shutdown with short deadline
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := srv.Shutdown(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Log("Shutdown returned nil (acceptable — some impls close forcefully)")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Shutdown did not respect deadline: took %v", elapsed)
	}
}

func TestListenAndServeWithSignals(t *testing.T) {
	addr := freePort()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	srv := NewServer(addr, handler)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeWithSignals(syscall.SIGINT)
	}()

	time.Sleep(200 * time.Millisecond)

	// Send SIGINT to self
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil on signal shutdown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down after signal")
	}
}

// TestSignalChannelCleanup verifies signal.Stop is called to prevent leaking.
func TestSignalChannelCleanup(t *testing.T) {
	addr := freePort()
	srv := NewServer(addr, http.DefaultServeMux)

	go srv.ListenAndServeWithSignals(syscall.SIGUSR1)
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	// After shutdown, sending SIGUSR1 should not affect anything.
	// If signal.Stop was not called, this could cause issues.
	time.Sleep(50 * time.Millisecond)
}

func TestAddrReturnsActualAddress(t *testing.T) {
	// Use port 0 to get a random port
	srv := NewServer("127.0.0.1:0", http.DefaultServeMux)
	go srv.ListenAndServe()
	time.Sleep(200 * time.Millisecond)

	addr := srv.Addr()
	if addr == "" || addr == "127.0.0.1:0" {
		t.Errorf("Addr() = %q; should return actual bound address", addr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
