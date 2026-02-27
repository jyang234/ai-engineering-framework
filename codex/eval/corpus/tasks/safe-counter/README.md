# Task: Thread-Safe Counter with Batch Operations

Implement a thread-safe counter that supports atomic increment, decrement, and batch operations.

## Requirements

Implement the following in `counter.go`:

1. **`Counter` struct** created via:
   **`NewCounter(initial int64) *Counter`**

2. **`(*Counter) Inc() int64`** — atomically increments by 1, returns new value.

3. **`(*Counter) Dec() int64`** — atomically decrements by 1, returns new value.

4. **`(*Counter) Add(delta int64) int64`** — atomically adds delta, returns new value.

5. **`(*Counter) Value() int64`** — returns current value.

6. **`(*Counter) Reset() int64`** — atomically sets to 0, returns previous value.

7. **`(*Counter) CompareAndSwap(old, new int64) bool`** — atomically sets value to new if current equals old.

8. **`CounterGroup` struct** created via:
   **`NewCounterGroup() *CounterGroup`** — manages named counters.

9. **`(*CounterGroup) Get(name string) *Counter`** — returns the counter for the given name, creating it (with initial value 0) if it doesn't exist.

10. **`(*CounterGroup) Snapshot() map[string]int64`** — returns a point-in-time copy of all counter values. Must not hold locks while reading individual counter values.

11. **`(*CounterGroup) Names() []string`** — returns sorted counter names.

## Constraints

- Use only the Go standard library
- Counter operations must be lock-free (use sync/atomic)
- CounterGroup must be safe for concurrent use
- Snapshot must return a consistent copy, not a reference to internal state
- CounterGroup.Get must not return nil for any name (always create if missing)
