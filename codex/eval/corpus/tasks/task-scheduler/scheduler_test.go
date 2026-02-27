package scheduler

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestJobExecutesAtIntervals verifies that a registered job fires repeatedly
// at roughly the configured interval.
func TestJobExecutesAtIntervals(t *testing.T) {
	s := NewScheduler()

	var count atomic.Int32
	err := s.Add(Job{
		Name:     "tick",
		Interval: 50 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			count.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	s.Start()

	// Wait enough time for several ticks
	time.Sleep(275 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Stop(ctx)

	got := count.Load()
	// In ~275ms with 50ms interval we expect 4-6 ticks (first at 50ms, then 100, 150, 200, 250)
	if got < 3 || got > 8 {
		t.Errorf("expected 3-8 executions in 275ms at 50ms interval, got %d", got)
	}
}

// TestDuplicateNameReturnsError verifies that adding two jobs with the same
// name returns ErrDuplicateJob.
func TestDuplicateNameReturnsError(t *testing.T) {
	s := NewScheduler()

	fn := func(ctx context.Context) error { return nil }

	err := s.Add(Job{Name: "dup", Interval: time.Second, Fn: fn})
	if err != nil {
		t.Fatalf("first Add failed: %v", err)
	}

	err = s.Add(Job{Name: "dup", Interval: time.Second, Fn: fn})
	if !errors.Is(err, ErrDuplicateJob) {
		t.Errorf("expected ErrDuplicateJob, got %v", err)
	}
}

// TestRemoveStopsJob verifies that Remove returns true for a known job,
// false for an unknown one, and that the removed job stops executing.
func TestRemoveStopsJob(t *testing.T) {
	s := NewScheduler()

	var count atomic.Int32
	s.Add(Job{
		Name:     "removable",
		Interval: 30 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			count.Add(1)
			return nil
		},
	})

	s.Start()

	// Let it tick a few times
	time.Sleep(100 * time.Millisecond)

	removed := s.Remove("removable")
	if !removed {
		t.Error("Remove returned false for existing job")
	}

	// Remove unknown job
	if s.Remove("nonexistent") {
		t.Error("Remove returned true for unknown job")
	}

	afterRemove := count.Load()

	// Wait and verify no more ticks happen
	time.Sleep(150 * time.Millisecond)

	afterWait := count.Load()
	if afterWait > afterRemove+1 {
		// Allow at most 1 extra tick that was in-flight during Remove
		t.Errorf("job continued after Remove: count went from %d to %d", afterRemove, afterWait)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Stop(ctx)
}

// TestOverlapPrevention verifies that if a job is still running when the next
// tick fires, that tick is skipped rather than launching a concurrent execution.
func TestOverlapPrevention(t *testing.T) {
	s := NewScheduler()

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var totalRuns atomic.Int32

	s.Add(Job{
		Name:     "slow",
		Interval: 20 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			cur := concurrent.Add(1)
			totalRuns.Add(1)
			// Track max concurrent executions
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			// Simulate a slow job that takes longer than the interval
			time.Sleep(80 * time.Millisecond)
			concurrent.Add(-1)
			return nil
		},
	})

	s.Start()

	// Let it run for several intervals
	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Stop(ctx)

	if maxConcurrent.Load() > 1 {
		t.Errorf("overlap detected: max concurrent executions = %d (want 1)", maxConcurrent.Load())
	}

	runs := totalRuns.Load()
	// With 80ms execution and 20ms interval over 300ms, we expect ~3-4 runs (not 15)
	if runs > 6 {
		t.Errorf("too many runs (%d) — overlap prevention may not be working", runs)
	}
}

// TestStopWaitsForInFlight verifies that Stop blocks until a currently-executing
// job function finishes.
func TestStopWaitsForInFlight(t *testing.T) {
	s := NewScheduler()

	var completed atomic.Bool
	started := make(chan struct{})

	s.Add(Job{
		Name:     "inflight",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			select {
			case <-started:
				// Already signaled
			default:
				close(started)
			}
			time.Sleep(200 * time.Millisecond)
			completed.Store(true)
			return nil
		},
	})

	s.Start()

	// Wait for the job to start executing
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if !completed.Load() {
		t.Error("Stop returned before in-flight job completed")
	}
}

// TestStopIsIdempotent verifies that calling Stop multiple times does not panic.
func TestStopIsIdempotent(t *testing.T) {
	s := NewScheduler()

	s.Add(Job{
		Name:     "idempotent",
		Interval: 50 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			return nil
		},
	})

	s.Start()
	time.Sleep(80 * time.Millisecond)

	ctx := context.Background()

	// Should not panic on multiple calls
	s.Stop(ctx)
	s.Stop(ctx)
	s.Stop(ctx)
}

// TestNoGoroutineLeakAfterStop verifies that all goroutines spawned by the
// scheduler are cleaned up after Stop returns.
func TestNoGoroutineLeakAfterStop(t *testing.T) {
	before := runtime.NumGoroutine()

	s := NewScheduler()

	for i := 0; i < 10; i++ {
		name := "job-" + string(rune('a'+i))
		s.Add(Job{
			Name:     name,
			Interval: 20 * time.Millisecond,
			Fn: func(ctx context.Context) error {
				time.Sleep(5 * time.Millisecond)
				return nil
			},
		})
	}

	s.Start()
	time.Sleep(100 * time.Millisecond) // Let jobs run

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(ctx)

	// Give goroutines time to fully exit
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before
	if leaked > 2 { // Allow small slack for runtime goroutines
		t.Errorf("goroutine leak: %d before, %d after (%d leaked)", before, after, leaked)
	}
}

// TestConcurrentAddRemoveSafety validates that concurrent Add and Remove
// operations do not cause panics or data races (run with -race).
func TestConcurrentAddRemoveSafety(t *testing.T) {
	s := NewScheduler()
	s.Start()

	var wg sync.WaitGroup
	fn := func(ctx context.Context) error {
		time.Sleep(time.Millisecond)
		return nil
	}

	// Concurrently add jobs
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := "concurrent-" + string(rune('A'+id%26)) + string(rune('0'+id/26))
			_ = s.Add(Job{Name: name, Interval: 50 * time.Millisecond, Fn: fn})
		}(i)
	}

	// Concurrently remove jobs while adds are happening
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond) // Slight delay so some adds complete
			name := "concurrent-" + string(rune('A'+id%26)) + string(rune('0'+id/26))
			s.Remove(name)
		}(i)
	}

	wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(ctx)
}

// TestAddAfterStart verifies that a job added after Start begins executing.
func TestAddAfterStart(t *testing.T) {
	s := NewScheduler()
	s.Start()

	var count atomic.Int32
	err := s.Add(Job{
		Name:     "late-add",
		Interval: 30 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			count.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Add after Start failed: %v", err)
	}

	time.Sleep(120 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Stop(ctx)

	if count.Load() < 2 {
		t.Errorf("job added after Start did not execute enough: got %d ticks", count.Load())
	}
}

// TestStopRespectsContextDeadline verifies that Stop returns promptly when the
// context deadline is exceeded, rather than blocking forever on a stuck job.
func TestStopRespectsContextDeadline(t *testing.T) {
	s := NewScheduler()

	started := make(chan struct{})
	s.Add(Job{
		Name:     "stuck",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			select {
			case <-started:
			default:
				close(started)
			}
			time.Sleep(10 * time.Second) // Simulate a stuck job
			return nil
		},
	})

	s.Start()
	<-started // Wait for the job to start

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := s.Stop(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Log("Stop returned nil (acceptable if implementation force-stops)")
	}

	if elapsed > 2*time.Second {
		t.Errorf("Stop did not respect context deadline: took %v", elapsed)
	}
}

// TestJobFnReceivesContext verifies that the function passed to a job receives
// a non-nil context.
func TestJobFnReceivesContext(t *testing.T) {
	s := NewScheduler()

	ctxReceived := make(chan struct{})
	s.Add(Job{
		Name:     "ctx-check",
		Interval: 20 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			if ctx != nil {
				select {
				case <-ctxReceived:
				default:
					close(ctxReceived)
				}
			}
			return nil
		},
	})

	s.Start()

	select {
	case <-ctxReceived:
		// Job received a non-nil context
	case <-time.After(2 * time.Second):
		t.Fatal("job function never received a non-nil context")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Stop(ctx)
}
