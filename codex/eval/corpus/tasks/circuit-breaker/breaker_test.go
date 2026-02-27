package breaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errService = errors.New("service unavailable")

func TestNewCircuitBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         5,
		ResetTimeout:        30 * time.Second,
		HalfOpenMaxRequests: 1,
	})

	if cb.State() != StateClosed {
		t.Errorf("new circuit breaker should be Closed, got %v", cb.State())
	}
}

func TestClosedPassesThrough(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         5,
		ResetTimeout:        30 * time.Second,
		HalfOpenMaxRequests: 1,
	})

	var calls int
	err := cb.Do(func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestTripsToOpenAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         3,
		ResetTimeout:        1 * time.Second,
		HalfOpenMaxRequests: 1,
	})

	// Cause MaxFailures consecutive failures
	for i := 0; i < 3; i++ {
		_ = cb.Do(func() error {
			return errService
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected Open after %d failures, got %v", 3, cb.State())
	}

	// Next call should fail immediately without executing fn
	called := false
	err := cb.Do(func() error {
		called = true
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
	if called {
		t.Error("function should NOT be called when circuit is Open")
	}
}

func TestSuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         3,
		ResetTimeout:        1 * time.Second,
		HalfOpenMaxRequests: 1,
	})

	// 2 failures, then a success
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return nil })

	// 2 more failures — should NOT trip because count was reset
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return errService })

	if cb.State() != StateClosed {
		t.Error("success should have reset failure count; breaker should still be Closed")
	}
}

func TestTransitionsToHalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})

	// Trip the breaker
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return errService })

	if cb.State() != StateOpen {
		t.Fatal("expected Open state")
	}

	// Wait for reset timeout
	time.Sleep(80 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Errorf("expected HalfOpen after reset timeout, got %v", cb.State())
	}
}

func TestHalfOpenSuccessResetsToClosed(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})

	// Trip and wait
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return errService })
	time.Sleep(80 * time.Millisecond)

	// Success in HalfOpen
	err := cb.Do(func() error { return nil })
	if err != nil {
		t.Fatalf("expected nil in HalfOpen, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Errorf("expected Closed after HalfOpen success, got %v", cb.State())
	}
}

func TestHalfOpenFailureReturnsToOpen(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})

	// Trip and wait
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return errService })
	time.Sleep(80 * time.Millisecond)

	// Failure in HalfOpen
	_ = cb.Do(func() error { return errService })

	if cb.State() != StateOpen {
		t.Errorf("expected Open after HalfOpen failure, got %v", cb.State())
	}
}

func TestHalfOpenLimitsRequests(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         2,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})

	// Trip and wait
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return errService })
	time.Sleep(80 * time.Millisecond)

	// Block the first HalfOpen request
	started := make(chan struct{})
	proceed := make(chan struct{})

	go func() {
		_ = cb.Do(func() error {
			close(started)
			<-proceed
			return nil
		})
	}()

	<-started // First request is in-flight

	// Second request should be rejected
	err := cb.Do(func() error { return nil })
	if !errors.Is(err, ErrTooManyRequests) {
		t.Errorf("expected ErrTooManyRequests, got %v", err)
	}

	close(proceed) // Let first request complete
}

func TestManualReset(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         2,
		ResetTimeout:        10 * time.Minute,
		HalfOpenMaxRequests: 1,
	})

	// Trip the breaker
	_ = cb.Do(func() error { return errService })
	_ = cb.Do(func() error { return errService })
	if cb.State() != StateOpen {
		t.Fatal("expected Open state")
	}

	// Manual reset
	cb.Reset()
	if cb.State() != StateClosed {
		t.Errorf("expected Closed after Reset, got %v", cb.State())
	}
}

// TestConcurrentAccess uses -race to verify no data races.
func TestConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         10,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	})

	var wg sync.WaitGroup
	var openErrors atomic.Int32
	var successCount atomic.Int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := cb.Do(func() error {
				if n%3 == 0 {
					return errService
				}
				return nil
			})
			if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrTooManyRequests) {
				openErrors.Add(1)
			}
			if err == nil {
				successCount.Add(1)
			}
			_ = cb.State()
		}(i)
	}

	wg.Wait()
	t.Logf("successes=%d, open_errors=%d", successCount.Load(), openErrors.Load())
}

func TestOriginalErrorPassedThrough(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		MaxFailures:         5,
		ResetTimeout:        30 * time.Second,
		HalfOpenMaxRequests: 1,
	})

	customErr := errors.New("custom service error")
	err := cb.Do(func() error {
		return customErr
	})

	if !errors.Is(err, customErr) {
		t.Errorf("expected original error, got %v", err)
	}
}
