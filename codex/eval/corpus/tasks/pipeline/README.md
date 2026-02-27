# Task: Fan-Out/Fan-In Data Processing Pipeline

Implement a fan-out/fan-in data processing pipeline. The pipeline takes input from a source channel, fans out to N worker goroutines that run items through a sequence of stages, and fans results back in to a single output channel.

## Requirements

Implement the following in `pipeline.go`:

1. **`Stage`** type:
   ```go
   type Stage func(ctx context.Context, in interface{}) (interface{}, error)
   ```
   A Stage transforms an input value into an output value, or returns an error.

2. **`Result` struct** with:
   - `Value interface{}` — the final output after all stages
   - `Err error` — the first error encountered (if any)
   - `StageIndex int` — index of the stage that produced the error (-1 if no error)

3. **`Pipeline` struct** created via:
   **`NewPipeline(workers int, stages ...Stage) *Pipeline`** where `workers` is the fan-out concurrency and `stages` is the ordered sequence of processing stages.

4. **`(*Pipeline) Process(ctx context.Context, input <-chan interface{}) (<-chan Result, error)`** that:
   - Starts processing items from the input channel
   - Fans out to `workers` goroutines, each reading from the input channel
   - Each worker runs an item through all stages in order
   - If a stage returns an error, the worker emits a Result with the error and stage index, then moves to the next input item
   - Fans results back in to a single output channel
   - Returns the output channel and nil error on success
   - Returns error if called with nil input channel

5. **Context cancellation**:
   - Pipeline must propagate context cancellation to all stages
   - Workers must exit promptly when the context is cancelled
   - Workers must not block on sending to the output channel when cancelled

6. **Goroutine lifecycle**:
   - All workers must exit cleanly when the input channel is closed
   - The output channel must be closed after ALL workers finish
   - No goroutines may leak on error, cancellation, or normal completion

## Constraints

- Use only the Go standard library
- Must not leak goroutines — all workers must exit after Process completes
- Must not panic from sending on a closed channel
- Workers must handle context cancellation in their send path
- The output channel must be closed exactly once, after all workers are done

## Example Usage

```go
double := func(ctx context.Context, in interface{}) (interface{}, error) {
    return in.(int) * 2, nil
}
addOne := func(ctx context.Context, in interface{}) (interface{}, error) {
    return in.(int) + 1, nil
}

p := NewPipeline(4, double, addOne)

input := make(chan interface{})
go func() {
    for i := 0; i < 100; i++ {
        input <- i
    }
    close(input)
}()

ctx := context.Background()
output, err := p.Process(ctx, input)
if err != nil {
    log.Fatal(err)
}

for result := range output {
    if result.Err != nil {
        fmt.Printf("error at stage %d: %v\n", result.StageIndex, result.Err)
    } else {
        fmt.Println(result.Value)
    }
}
```
