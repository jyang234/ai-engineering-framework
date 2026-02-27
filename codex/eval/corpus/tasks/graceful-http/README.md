# Task: HTTP Server with Graceful Shutdown

Implement an HTTP server wrapper that handles OS signals and shuts down gracefully, draining in-flight requests.

## Requirements

Implement the following in `server.go`:

1. **`Server` struct** created via:
   **`NewServer(addr string, handler http.Handler) *Server`**

2. **`(*Server) ListenAndServe() error`** that:
   - Starts the HTTP server on the configured address
   - Blocks until the server is shut down
   - Returns `http.ErrServerClosed` on clean shutdown (not treated as an error)

3. **`(*Server) ListenAndServeWithSignals(signals ...os.Signal) error`** that:
   - Starts the HTTP server
   - Listens for the specified OS signals (default: SIGINT, SIGTERM)
   - On receiving a signal, initiates graceful shutdown
   - Returns nil on clean signal-triggered shutdown

4. **`(*Server) Shutdown(ctx context.Context) error`** that:
   - Stops accepting new connections
   - Waits for in-flight requests to complete
   - Respects context deadline — forcefully closes after deadline
   - Is safe to call multiple times
   - Calls registered shutdown hooks before closing

5. **`(*Server) OnShutdown(fn func()) `** registers a function to call during shutdown.

6. **`(*Server) Addr() string`** returns the actual listening address (useful when port 0 is used).

## Constraints

- Use only the Go standard library
- Must properly register signal handlers and clean them up
- In-flight requests must be allowed to complete (drain)
- The server must not leak goroutines after shutdown
- signal.Notify must be matched with signal.Stop to avoid leaking the signal channel

## Example Usage

```go
mux := http.NewServeMux()
mux.HandleFunc("/", handler)

srv := NewServer(":8080", mux)
srv.OnShutdown(func() { db.Close() })

if err := srv.ListenAndServeWithSignals(syscall.SIGINT, syscall.SIGTERM); err != nil {
    log.Fatal(err)
}
```
