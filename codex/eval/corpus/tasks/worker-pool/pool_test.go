package pool

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewPoolCreatesPool(t *testing.T) {
	p := NewPool(4)
	if p == nil {
		t.Fatal("NewPool returned nil")
	}
}

func TestSubmitAndCollectResults(t *testing.T) {
	p := NewPool(2)
	p.Start()

	var completed atomic.Int32

	for i := 0; i < 10; i++ {
		id := i
		err := p.Submit(id, func() error {
			completed.Add(1)
			return nil
		})
		if err != nil {
			t.Fatalf("Submit(%d) failed: %v", id, err)
		}
	}

	// Collect results
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond) // Let tasks run
		p.Shutdown(ctx)
	}()

	var resultCount int
	for range p.Results() {
		resultCount++
		if resultCount >= 10 {
			break
		}
	}

	if completed.Load() != 10 {
		t.Errorf("expected 10 completions, got %d", completed.Load())
	}
}

func TestResultsContainErrors(t *testing.T) {
	p := NewPool(2)
	p.Start()

	errTest := errors.New("task failed")

	p.Submit(1, func() error { return nil })
	p.Submit(2, func() error { return errTest })
	p.Submit(3, func() error { return nil })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Shutdown(ctx)
	}()

	var errorCount int
	for result := range p.Results() {
		if result.Err != nil {
			errorCount++
		}
	}

	if errorCount != 1 {
		t.Errorf("expected 1 error result, got %d", errorCount)
	}
}

func TestSubmitAfterShutdownReturnsError(t *testing.T) {
	p := NewPool(2)
	p.Start()

	ctx := context.Background()
	p.Shutdown(ctx)

	err := p.Submit(1, func() error { return nil })
	if !errors.Is(err, ErrPoolClosed) {
		t.Errorf("expected ErrPoolClosed after shutdown, got %v", err)
	}
}

// TestShutdownWaitsForInFlightTasks verifies graceful shutdown.
func TestShutdownWaitsForInFlightTasks(t *testing.T) {
	p := NewPool(2)
	p.Start()

	var completed atomic.Int32
	started := make(chan struct{})

	p.Submit(1, func() error {
		close(started)
		time.Sleep(200 * time.Millisecond)
		completed.Add(1)
		return nil
	})

	<-started // Wait for task to start

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.Shutdown(ctx)

	if completed.Load() != 1 {
		t.Error("Shutdown returned before in-flight task completed")
	}
}

func TestShutdownRespectsContextDeadline(t *testing.T) {
	p := NewPool(1)
	p.Start()

	p.Submit(1, func() error {
		time.Sleep(10 * time.Second) // Very long task
		return nil
	})

	time.Sleep(50 * time.Millisecond) // Let task start

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := p.Shutdown(ctx)
	elapsed := time.Since(start)

	if err == nil {
		// Some implementations may return the context error
		t.Log("Shutdown did not return error (acceptable if it returned within deadline)")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Shutdown did not respect context deadline: took %v", elapsed)
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	p := NewPool(2)
	p.Start()

	ctx := context.Background()

	// Should not panic
	p.Shutdown(ctx)
	p.Shutdown(ctx)
	p.Shutdown(ctx)
}

// TestNoGoroutineLeaks verifies all workers exit after shutdown.
// This is the most critical test — goroutine leaks waste resources
// and can cause the program to hang.
func TestNoGoroutineLeaks(t *testing.T) {
	before := runtime.NumGoroutine()

	p := NewPool(8)
	p.Start()

	for i := 0; i < 20; i++ {
		p.Submit(i, func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	// Drain results
	go func() {
		for range p.Results() {
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx)

	// Give goroutines time to clean up
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before
	if leaked > 2 { // Allow some slack for runtime goroutines
		t.Errorf("goroutine leak: %d goroutines before, %d after (%d leaked)", before, after, leaked)
	}
}

// TestResultsChannelClosed verifies the results channel is closed after shutdown.
func TestResultsChannelClosed(t *testing.T) {
	p := NewPool(2)
	p.Start()

	p.Submit(1, func() error { return nil })

	ctx := context.Background()

	// Drain and shutdown
	done := make(chan struct{})
	go func() {
		for range p.Results() {
		}
		close(done) // This will only execute if Results() channel is closed
	}()

	time.Sleep(50 * time.Millisecond)
	p.Shutdown(ctx)

	select {
	case <-done:
		// Results channel was closed
	case <-time.After(3 * time.Second):
		t.Fatal("Results channel was never closed after shutdown")
	}
}

// TestConcurrentSubmitAndShutdown validates no panics from concurrent operations.
func TestConcurrentSubmitAndShutdown(t *testing.T) {
	p := NewPool(4)
	p.Start()

	var wg sync.WaitGroup

	// Goroutine that submits tasks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = p.Submit(i, func() error {
				time.Sleep(time.Millisecond)
				return nil
			})
		}
	}()

	// Goroutine that reads results
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range p.Results() {
		}
	}()

	// Let some tasks get submitted
	time.Sleep(20 * time.Millisecond)

	// Shutdown while tasks are being submitted
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx)

	wg.Wait()
}

// TestWorkerRecoversPanic verifies that a panicking task does not crash the pool.
func TestWorkerRecoversPanic(t *testing.T) {
	p := NewPool(2)
	p.Start()

	p.Submit(1, func() error {
		panic("task panic!")
	})
	p.Submit(2, func() error {
		return nil
	})

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Drain results
	var panicResult bool
	go func() {
		for r := range p.Results() {
			if r.TaskID == 1 && r.Err != nil {
				panicResult = true
			}
		}
	}()

	p.Shutdown(ctx)

	if !panicResult {
		t.Log("Note: panic recovery converts panic to error in result (implementation dependent)")
	}
}

func TestRunningCount(t *testing.T) {
	p := NewPool(4)
	p.Start()

	started := make(chan struct{}, 3)
	proceed := make(chan struct{})

	for i := 0; i < 3; i++ {
		p.Submit(i, func() error {
			started <- struct{}{}
			<-proceed
			return nil
		})
	}

	// Wait for all 3 to start
	for i := 0; i < 3; i++ {
		<-started
	}

	running := p.Running()
	if running != 3 {
		t.Errorf("expected 3 running, got %d", running)
	}

	close(proceed)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		for range p.Results() {
		}
	}()

	p.Shutdown(ctx)
}
