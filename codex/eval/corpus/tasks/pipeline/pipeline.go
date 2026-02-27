// Package pipeline implements a fan-out/fan-in data processing pipeline.
package pipeline

import (
	"context"
	"errors"
)

// ErrNilInput is returned when Process is called with a nil input channel.
var ErrNilInput = errors.New("input channel must not be nil")

// Stage is a processing function that transforms an input value.
// It receives a context for cancellation and returns the transformed value or an error.
type Stage func(ctx context.Context, in interface{}) (interface{}, error)

// Result holds the outcome of processing an item through the pipeline.
type Result struct {
	Value      interface{} // Final output after all stages (nil if error)
	Err        error       // First error encountered (nil if success)
	StageIndex int         // Index of the failing stage (-1 if no error)
}

// Pipeline fans out input items to N workers, each running items through
// an ordered sequence of stages, then fans results back in.
type Pipeline struct {
	workers int
	stages  []Stage
}

// NewPipeline creates a pipeline with the given fan-out concurrency and
// ordered processing stages.
func NewPipeline(workers int, stages ...Stage) *Pipeline {
	// TODO: implement
	return &Pipeline{workers: workers, stages: stages}
}

// Process starts processing items from the input channel.
// It returns a channel of Results and an error if the input is invalid.
// The output channel is closed after all workers finish.
func (p *Pipeline) Process(ctx context.Context, input <-chan interface{}) (<-chan Result, error) {
	// TODO: implement
	// - Validate input is not nil
	// - Create output channel
	// - Launch p.workers goroutines, each reading from input
	// - Each worker runs items through p.stages in order
	// - On stage error: emit Result with Err and StageIndex, continue to next item
	// - On context cancellation: workers must exit without blocking on output send
	// - Use sync.WaitGroup to track workers; close output channel when all done
	return nil, ErrNilInput
}
