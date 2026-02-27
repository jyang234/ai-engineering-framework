# Task: Worker Pool with Graceful Shutdown

Implement a worker pool that processes tasks concurrently with support for graceful shutdown.

## Requirements

Implement the following in `pool.go`:

1. **`Task`** type: `type Task func() error`

2. **`Result` struct** with:
   - `Value interface{}` — the result value (set by task if needed)
   - `Err error` — error from task execution
   - `TaskID int` — identifier for the task

3. **`Pool` struct** created via:
   **`NewPool(workers int) *Pool`** where `workers` is the number of concurrent workers.

4. **`(*Pool) Start()`** that launches worker goroutines. Workers must:
   - Pull tasks from an internal channel
   - Execute tasks and collect results
   - Stop cleanly when the pool is shut down

5. **`(*Pool) Submit(id int, fn Task) error`** that:
   - Submits a task for execution
   - Returns `ErrPoolClosed` if the pool has been shut down
   - Does NOT block indefinitely — use a buffered channel or return error if full

6. **`(*Pool) Results() <-chan Result`** returns a read-only channel of results.

7. **`(*Pool) Shutdown(ctx context.Context) error`** that:
   - Signals no more tasks will be submitted (closes task channel)
   - Waits for all in-flight tasks to complete
   - Respects context deadline — returns `ctx.Err()` if deadline exceeded
   - Closes the results channel AFTER all workers finish
   - Is safe to call multiple times (idempotent)

8. **`(*Pool) Running() int`** returns the number of currently executing tasks.

## Constraints

- Use only the Go standard library
- Must not leak goroutines — all workers must exit after Shutdown completes
- Must not panic from sending on a closed channel
- Must handle concurrent Submit and Shutdown safely
- Workers should recover from panics in task functions (convert to error in Result)
- The results channel must be closed exactly once, after all workers are done

## Example Usage

```go
pool := NewPool(4)
pool.Start()

for i := 0; i < 100; i++ {
    id := i
    pool.Submit(id, func() error {
        // do work
        return nil
    })
}

go func() {
    for result := range pool.Results() {
        fmt.Printf("Task %d: err=%v\n", result.TaskID, result.Err)
    }
}()

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
pool.Shutdown(ctx)
```
