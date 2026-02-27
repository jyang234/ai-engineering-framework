# Task: Retry with Exponential Backoff

Implement a retry mechanism with exponential backoff and jitter for unreliable operations.

## Requirements

Implement the following in `retry.go`:

1. **`RetryConfig` struct** with fields:
   - `MaxRetries int` — maximum number of retry attempts (default: 3)
   - `InitialDelay time.Duration` — base delay before first retry (default: 100ms)
   - `MaxDelay time.Duration` — cap on delay between retries (default: 10s)
   - `Multiplier float64` — backoff multiplier (default: 2.0)

2. **`Retry(ctx context.Context, config RetryConfig, fn func() error) error`** function that:
   - Calls `fn()` up to `MaxRetries + 1` times (1 initial + retries)
   - On failure, waits with exponential backoff: `delay = InitialDelay * Multiplier^attempt`
   - Adds random jitter to each delay (±25% of the calculated delay)
   - Caps the delay at `MaxDelay`
   - Respects context cancellation — returns `ctx.Err()` if context is cancelled during wait
   - Returns `nil` on success, or the last error after all retries are exhausted

3. **`DefaultRetryConfig() RetryConfig`** that returns sensible defaults.

## Constraints

- Use only the Go standard library
- Must be safe for concurrent use (multiple goroutines can call Retry simultaneously)
- Jitter must be truly random (not deterministic) to prevent thundering herd
- The function must respect context cancellation at every wait point

## Example Usage

```go
config := DefaultRetryConfig()
err := Retry(ctx, config, func() error {
    return callExternalService()
})
```
