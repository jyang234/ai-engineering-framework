// Package aggregator implements a time-windowed stream aggregator.
package aggregator

import (
	"time"
)

// Event represents a single data point in the stream.
type Event struct {
	Key       string
	Value     float64
	Timestamp time.Time
}

// WindowResult holds the aggregated results for a single key within a time window.
type WindowResult struct {
	Key         string
	Sum         float64
	Count       int
	Min         float64
	Max         float64
	WindowStart time.Time
	WindowEnd   time.Time
}

// Aggregator collects events and emits aggregate results per time window.
type Aggregator struct {
	windowSize time.Duration
	emitFn     func([]WindowResult)
}

// NewAggregator creates an aggregator with the given window size and emit function.
func NewAggregator(windowSize time.Duration, emitFn func([]WindowResult)) *Aggregator {
	// TODO: implement
	return &Aggregator{
		windowSize: windowSize,
		emitFn:     emitFn,
	}
}

// Start begins the background window ticker.
func (a *Aggregator) Start() {
	// TODO: implement
}

// Emit adds an event to the current window. Safe for concurrent use.
func (a *Aggregator) Emit(event Event) {
	// TODO: implement
}

// Flush manually closes the current window and emits results.
func (a *Aggregator) Flush() {
	// TODO: implement
}

// Stop stops the ticker and flushes any partial window. Must not leak goroutines.
func (a *Aggregator) Stop() {
	// TODO: implement
}
