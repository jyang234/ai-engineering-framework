// Package pool implements a worker pool with graceful shutdown.
package pool

import (
	"context"
	"errors"
)

// ErrPoolClosed is returned when submitting to a shut-down pool.
var ErrPoolClosed = errors.New("pool is closed")

// Task is a unit of work to be executed by the pool.
type Task func() error

// Result holds the outcome of a task execution.
type Result struct {
	Value  interface{}
	Err    error
	TaskID int
}

// Pool manages a set of worker goroutines.
type Pool struct {
	workers int
}

// NewPool creates a worker pool with the given number of workers.
func NewPool(workers int) *Pool {
	// TODO: implement
	return &Pool{workers: workers}
}

// Start launches the worker goroutines.
func (p *Pool) Start() {
	// TODO: implement
}

// Submit adds a task to the pool for execution.
// Returns ErrPoolClosed if the pool has been shut down.
func (p *Pool) Submit(id int, fn Task) error {
	// TODO: implement
	return ErrPoolClosed
}

// Results returns a read-only channel of task results.
func (p *Pool) Results() <-chan Result {
	// TODO: implement
	return nil
}

// Shutdown signals the pool to stop and waits for all workers to finish.
// It respects the context deadline.
func (p *Pool) Shutdown(ctx context.Context) error {
	// TODO: implement
	return nil
}

// Running returns the number of currently executing tasks.
func (p *Pool) Running() int {
	// TODO: implement
	return 0
}
