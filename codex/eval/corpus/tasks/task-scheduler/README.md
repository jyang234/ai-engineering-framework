# Task: Periodic Task Scheduler

Implement a periodic task scheduler that runs functions at specified intervals, similar to a simple cron.

## Requirements

Implement the following in `scheduler.go`:

1. **`Job` struct** with:
   - `Name string` — unique identifier for the job
   - `Interval time.Duration` — how often the job runs
   - `Fn func(ctx context.Context) error` — the function to execute each tick

2. **`Scheduler` struct** created via:
   **`NewScheduler() *Scheduler`**

3. **`(*Scheduler) Add(job Job) error`** that:
   - Registers a job with the scheduler
   - Returns an error if a job with the same `Name` is already registered
   - May be called before or after `Start()`

4. **`(*Scheduler) Remove(name string) bool`** that:
   - Removes a registered job by name
   - Returns `true` if the job was found and removed, `false` otherwise
   - If the scheduler is running, the job's goroutine must be stopped

5. **`(*Scheduler) Start()`** that:
   - Begins executing all registered jobs at their configured intervals
   - Each job runs in its own goroutine with a `time.Ticker`
   - Jobs added after `Start()` must also begin executing immediately

6. **`(*Scheduler) Stop(ctx context.Context) error`** that:
   - Signals all job goroutines to stop
   - Waits for any in-flight job executions to finish
   - Respects context deadline — returns `ctx.Err()` if deadline exceeded
   - Must not leak goroutines — all job goroutines must exit after Stop returns
   - Is safe to call multiple times (idempotent)

## Overlap Prevention

Jobs must NOT overlap: if a job's function is still executing when the next interval tick fires, that tick must be skipped. This prevents resource exhaustion from slow jobs stacking up.

## Concurrency Safety

All methods (`Add`, `Remove`, `Start`, `Stop`) must be safe for concurrent use from multiple goroutines. The scheduler must not panic or corrupt state under concurrent access.

## Constraints

- Use only the Go standard library
- Each job goroutine must use `time.Ticker` for scheduling
- Stop must signal each job goroutine to exit (e.g., via a done channel) and stop the ticker
- No goroutine leaks — `runtime.NumGoroutine()` must return to baseline after Stop

## Example Usage

```go
s := NewScheduler()

s.Add(Job{
    Name:     "health-check",
    Interval: 5 * time.Second,
    Fn: func(ctx context.Context) error {
        return pingHealthEndpoint(ctx)
    },
})

s.Add(Job{
    Name:     "cleanup",
    Interval: 1 * time.Minute,
    Fn: func(ctx context.Context) error {
        return cleanupTempFiles(ctx)
    },
})

s.Start()

// Later...
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
s.Stop(ctx)
```
