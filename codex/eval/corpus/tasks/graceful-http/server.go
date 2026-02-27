// Package server provides an HTTP server with graceful shutdown support.
package server

import (
	"context"
	"net/http"
	"os"
)

// Server wraps http.Server with graceful shutdown and signal handling.
type Server struct {
	addr    string
	handler http.Handler
}

// NewServer creates a new server.
func NewServer(addr string, handler http.Handler) *Server {
	// TODO: implement
	return &Server{addr: addr, handler: handler}
}

// ListenAndServe starts the server and blocks until shutdown.
func (s *Server) ListenAndServe() error {
	// TODO: implement
	return nil
}

// ListenAndServeWithSignals starts the server and shuts down on OS signals.
func (s *Server) ListenAndServeWithSignals(signals ...os.Signal) error {
	// TODO: implement
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO: implement
	return nil
}

// OnShutdown registers a function to call during shutdown.
func (s *Server) OnShutdown(fn func()) {
	// TODO: implement
}

// Addr returns the listening address.
func (s *Server) Addr() string {
	// TODO: implement
	return s.addr
}
