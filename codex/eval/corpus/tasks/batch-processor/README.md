# Task: Batch Processor with Timer Flush

Implement a batch processor that collects items and flushes them when a count threshold or time interval is reached.

## Requirements

Implement the following in `batch.go`:

1. **`Config` struct** with:
   - `MaxSize int` — flush when batch reaches this size
   - `MaxWait time.Duration` — flush after this interval even if batch isn't full
   - `FlushFunc func(items []interface{}) error` — called on flush with accumulated items

2. **`Processor` struct** created via:
   **`NewProcessor(config Config) *Processor`**

3. **`(*Processor) Start()`** — starts the background flush timer. Must be called before Add.

4. **`(*Processor) Add(item interface{}) error`** that:
   - Adds an item to the current batch
   - If the batch reaches MaxSize, triggers an immediate flush
   - Returns `ErrProcessorClosed` if called after Shutdown
   - Must be safe for concurrent use

5. **`(*Processor) Flush() error`** — manually triggers a flush of the current batch.

6. **`(*Processor) Shutdown(ctx context.Context) error`** that:
   - Stops the flush timer
   - Flushes any remaining items
   - Returns error if the final flush fails
   - Respects context deadline
   - Is safe to call multiple times

7. **`(*Processor) Pending() int`** — returns the number of items waiting to be flushed.

## Constraints

- Use only the Go standard library
- Must be safe for concurrent Add calls from multiple goroutines
- Timer flush and size-triggered flush must not race (use proper synchronization)
- Shutdown must flush remaining items — do not lose data
- FlushFunc errors should be returned to the caller (Add or Flush)
