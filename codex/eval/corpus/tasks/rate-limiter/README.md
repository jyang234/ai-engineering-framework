# Task: Token Bucket Rate Limiter

Implement a token bucket rate limiter for controlling the rate of operations.

## Requirements

Implement the following in `limiter.go`:

1. **`Limiter` struct** created via:
   **`NewLimiter(rate float64, burst int) *Limiter`** where:
   - `rate` is tokens added per second
   - `burst` is maximum tokens that can accumulate (bucket capacity)

2. **`(*Limiter) Allow() bool`** that:
   - Returns `true` and consumes one token if available
   - Returns `false` if no tokens are available
   - Must be safe for concurrent use

3. **`(*Limiter) Wait(ctx context.Context) error`** that:
   - Blocks until a token is available or context is cancelled
   - Returns `nil` when a token is consumed
   - Returns `ctx.Err()` if context is cancelled while waiting

4. **`(*Limiter) AllowN(n int) bool`** that:
   - Atomically consumes `n` tokens if available
   - Returns `false` without consuming any tokens if fewer than `n` are available

5. **`(*Limiter) Tokens() float64`** that returns the current number of available tokens.

## Token Refill Logic

- Tokens are refilled lazily: calculate elapsed time since last refill and add `rate * elapsed` tokens
- Tokens must NEVER exceed `burst` (the bucket capacity)
- Time tracking must use `time.Now()` — do not use background goroutines for refilling

## Constraints

- Use only the Go standard library
- Must be safe for concurrent use from multiple goroutines (use sync.Mutex)
- Token count must never exceed burst capacity after refill
- `Wait` must respect context cancellation and not busy-spin
- Zero or negative rate means no tokens are ever added

## Example Usage

```go
lim := NewLimiter(10.0, 100) // 10 tokens/sec, burst of 100

if lim.Allow() {
    handleRequest()
} else {
    rejectRequest()
}

// Or blocking:
if err := lim.Wait(ctx); err != nil {
    return err
}
handleRequest()
```
