# Task: Request Coalescer

Implement request deduplication/coalescing (like Go's `singleflight`). When multiple goroutines request the same key simultaneously, only one executes the function and the result is shared with all callers.

## Requirements

Implement the following in `coalescer.go`:

1. **`Result` struct** with:
   - `Value interface{}` — the value returned by the function
   - `Err error` — the error returned by the function
   - `Shared bool` — true if this result was shared from another caller's in-flight execution

2. **`Group` struct** created via:
   **`NewGroup() *Group`** — creates a new coalescing group.

3. **`(*Group) Do(key string, fn func() (interface{}, error)) Result`** that:
   - If no call is in-flight for `key`, executes `fn` and returns its result with `Shared: false`
   - If a call is already in-flight for `key`, blocks until it completes and returns the same result with `Shared: true`
   - `fn` must only be called **once** per key for concurrent callers
   - Removes the in-flight entry after `fn` completes so subsequent calls execute fresh

4. **`(*Group) DoChan(key string, fn func() (interface{}, error)) <-chan Result`** that:
   - Like `Do` but returns a channel immediately (non-blocking)
   - The channel receives exactly one `Result` when `fn` completes
   - Follows the same deduplication rules as `Do`

5. **`(*Group) Forget(key string)`** that:
   - Removes the in-flight entry for `key` without affecting any in-progress execution
   - The next call to `Do` or `DoChan` for that key will execute `fn` fresh
   - Safe to call even if no entry exists for `key`

## Constraints

- Use only the Go standard library
- Must be safe for concurrent use from multiple goroutines
- `fn` must be called exactly once per key when multiple goroutines call `Do` concurrently with the same key
- Must properly clean up in-flight entries after `fn` completes — even if `fn` panics
- `Forget` must not cancel or interrupt an in-flight `fn` execution

## Example Usage

```go
g := NewGroup()

// Multiple goroutines requesting the same key — only one fetch executes
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        result := g.Do("user:42", func() (interface{}, error) {
            return fetchUser(42) // called only once
        })
        fmt.Println(result.Value, result.Shared)
    }()
}
wg.Wait()

// Non-blocking variant
ch := g.DoChan("config", func() (interface{}, error) {
    return loadConfig()
})
// ... do other work ...
result := <-ch

// Force re-execution on next call
g.Forget("user:42")
```
