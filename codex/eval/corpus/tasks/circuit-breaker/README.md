# Task: Circuit Breaker

Implement the circuit breaker pattern for protecting against cascading failures in distributed systems.

## Requirements

Implement the following in `breaker.go`:

1. **`State` type** — the circuit breaker has three states:
   - `StateClosed` — normal operation, requests pass through
   - `StateOpen` — circuit is tripped, requests fail immediately
   - `StateHalfOpen` — testing recovery, limited requests allowed

2. **`CircuitBreaker` struct** created via:
   **`NewCircuitBreaker(config Config) *CircuitBreaker`** where `Config` has:
   - `MaxFailures int` — failures before tripping to Open (default: 5)
   - `ResetTimeout time.Duration` — time in Open before transitioning to HalfOpen (default: 30s)
   - `HalfOpenMaxRequests int` — max concurrent requests in HalfOpen state (default: 1)

3. **`(*CircuitBreaker) Do(fn func() error) error`** that:
   - In **Closed**: executes `fn`, counts consecutive failures, trips to Open at `MaxFailures`
   - In **Open**: returns `ErrCircuitOpen` immediately without calling `fn`; transitions to HalfOpen after `ResetTimeout`
   - In **HalfOpen**: allows up to `HalfOpenMaxRequests` concurrent calls; success resets to Closed, failure returns to Open

4. **`(*CircuitBreaker) State() State`** returns current state.

5. **`(*CircuitBreaker) Reset()`** manually resets to Closed.

6. Exported errors: `ErrCircuitOpen` and `ErrTooManyRequests` (for HalfOpen overflow).

## Constraints

- Use only the Go standard library
- Must be safe for concurrent use from multiple goroutines
- State transitions must be atomic — no race conditions
- A single success in HalfOpen must reset the breaker to Closed
- A single failure in HalfOpen must trip back to Open
- In HalfOpen, requests beyond `HalfOpenMaxRequests` must return `ErrTooManyRequests`

## Example Usage

```go
cb := NewCircuitBreaker(Config{
    MaxFailures:         5,
    ResetTimeout:        30 * time.Second,
    HalfOpenMaxRequests: 1,
})

err := cb.Do(func() error {
    return callDownstream()
})
```
