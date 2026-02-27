// Package batch implements a batch processor with timer-based flushing.
package batch

import (
	"context"
	"errors"
	"time"
)

// ErrProcessorClosed is returned when adding to a shut-down processor.
var ErrProcessorClosed = errors.New("processor is closed")

// Config holds batch processor configuration.
type Config struct {
	MaxSize   int
	MaxWait   time.Duration
	FlushFunc func(items []interface{}) error
}

// Processor collects items and flushes them in batches.
type Processor struct {
	config Config
}

// NewProcessor creates a batch processor.
func NewProcessor(config Config) *Processor {
	// TODO: implement
	return &Processor{config: config}
}

// Start begins the background flush timer.
func (p *Processor) Start() {
	// TODO: implement
}

// Add adds an item to the current batch.
func (p *Processor) Add(item interface{}) error {
	// TODO: implement
	return ErrProcessorClosed
}

// Flush manually triggers a flush of the current batch.
func (p *Processor) Flush() error {
	// TODO: implement
	return nil
}

// Shutdown stops the processor and flushes remaining items.
func (p *Processor) Shutdown(ctx context.Context) error {
	// TODO: implement
	return nil
}

// Pending returns the number of items waiting to be flushed.
func (p *Processor) Pending() int {
	// TODO: implement
	return 0
}
