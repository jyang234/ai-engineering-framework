# Task: Time-Windowed Stream Aggregator

Implement a time-windowed stream aggregator that collects events and emits aggregate results per time window, grouped by key.

## Requirements

Implement the following in `aggregator.go`:

1. **`Event` struct** with:
   - `Key string` — grouping key for aggregation
   - `Value float64` — numeric value to aggregate
   - `Timestamp time.Time` — when the event occurred

2. **`WindowResult` struct** with:
   - `Key string` — the grouping key
   - `Sum float64` — sum of all values for this key in the window
   - `Count int` — number of events for this key in the window
   - `Min float64` — minimum value seen for this key in the window
   - `Max float64` — maximum value seen for this key in the window
   - `WindowStart time.Time` — start of the time window
   - `WindowEnd time.Time` — end of the time window

3. **`Aggregator` struct** created via:
   **`NewAggregator(windowSize time.Duration, emitFn func([]WindowResult)) *Aggregator`**
   where `windowSize` is how long each window lasts and `emitFn` is called with aggregated results when a window closes.

4. **`(*Aggregator) Start()`** — starts the background window ticker that closes and emits windows at each `windowSize` interval.

5. **`(*Aggregator) Emit(event Event)`** that:
   - Adds an event to the current window's aggregation
   - Events are grouped by `Key` — each unique key has its own running Sum, Count, Min, Max
   - Must be safe for concurrent use from multiple goroutines

6. **`(*Aggregator) Flush()`** — manually closes the current window and calls `emitFn` with the aggregated results. Resets the window for fresh accumulation.

7. **`(*Aggregator) Stop()`** that:
   - Stops the background ticker
   - Flushes any partial window (events accumulated since last emit)
   - Must not leak goroutines — the background ticker goroutine must exit

## Constraints

- Use only the Go standard library
- Must be safe for concurrent Emit calls from multiple goroutines
- The background ticker and Emit must not race on the window data (use proper synchronization)
- Stop must flush remaining events — do not lose data from the partial window
- Empty windows (no events received) should not call emitFn
- Each window tracks WindowStart (when the window opened) and WindowEnd (when it closed)
