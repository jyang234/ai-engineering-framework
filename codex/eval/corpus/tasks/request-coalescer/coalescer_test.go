package coalescer

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleCallExecutesFn(t *testing.T) {
	g := NewGroup()

	result := g.Do("key", func() (interface{}, error) {
		return 42, nil
	})

	if result.Value != 42 {
		t.Errorf("Value = %v; want 42", result.Value)
	}
	if result.Err != nil {
		t.Errorf("Err = %v; want nil", result.Err)
	}
	if result.Shared {
		t.Error("Shared = true; want false for single caller")
	}
}

func TestConcurrentSameKeyCallsFnOnce(t *testing.T) {
	g := NewGroup()
	var callCount atomic.Int32

	started := make(chan struct{})
	proceed := make(chan struct{})

	var wg sync.WaitGroup
	const numCallers = 10

	// First goroutine will start fn and block until we signal
	for i := 0; i < numCallers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.Do("key", func() (interface{}, error) {
				callCount.Add(1)
				close(started) // signal that fn has started
				<-proceed      // block until test tells us to continue
				return "result", nil
			})
		}()
		if i == 0 {
			<-started // wait for first goroutine to enter fn
		}
	}

	// Give other goroutines a moment to queue up on the in-flight call
	time.Sleep(50 * time.Millisecond)

	close(proceed) // let fn complete
	wg.Wait()

	if count := callCount.Load(); count != 1 {
		t.Errorf("fn called %d times; want 1", count)
	}
}

func TestDifferentKeysExecuteIndependently(t *testing.T) {
	g := NewGroup()
	var countA, countB atomic.Int32

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		g.Do("a", func() (interface{}, error) {
			countA.Add(1)
			return "A", nil
		})
	}()

	go func() {
		defer wg.Done()
		g.Do("b", func() (interface{}, error) {
			countB.Add(1)
			return "B", nil
		})
	}()

	wg.Wait()

	if countA.Load() != 1 {
		t.Errorf("fn for key 'a' called %d times; want 1", countA.Load())
	}
	if countB.Load() != 1 {
		t.Errorf("fn for key 'b' called %d times; want 1", countB.Load())
	}
}

func TestSharedFlagTrueForWaiters(t *testing.T) {
	g := NewGroup()

	started := make(chan struct{})
	proceed := make(chan struct{})

	var results [2]Result
	var wg sync.WaitGroup

	// First caller — the one that actually executes fn
	wg.Add(1)
	go func() {
		defer wg.Done()
		results[0] = g.Do("key", func() (interface{}, error) {
			close(started)
			<-proceed
			return "val", nil
		})
	}()

	<-started // fn is now in-flight

	// Second caller — should wait and get Shared=true
	wg.Add(1)
	go func() {
		defer wg.Done()
		results[1] = g.Do("key", func() (interface{}, error) {
			t.Error("fn should not be called for second caller")
			return "wrong", nil
		})
	}()

	time.Sleep(20 * time.Millisecond) // let second caller queue up
	close(proceed)
	wg.Wait()

	if results[0].Shared {
		t.Error("first caller: Shared = true; want false")
	}
	if !results[1].Shared {
		t.Error("second caller: Shared = false; want true")
	}
	if results[1].Value != "val" {
		t.Errorf("second caller: Value = %v; want 'val'", results[1].Value)
	}
}

func TestErrorPropagatedToAllWaiters(t *testing.T) {
	g := NewGroup()
	errFailed := errors.New("fetch failed")

	started := make(chan struct{})
	proceed := make(chan struct{})

	var wg sync.WaitGroup
	var results [3]Result

	for i := 0; i < 3; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = g.Do("key", func() (interface{}, error) {
				if idx == 0 {
					close(started)
					<-proceed
				}
				return nil, errFailed
			})
		}()
		if i == 0 {
			<-started
		}
	}

	time.Sleep(20 * time.Millisecond)
	close(proceed)
	wg.Wait()

	for i, r := range results {
		if !errors.Is(r.Err, errFailed) {
			t.Errorf("caller %d: Err = %v; want %v", i, r.Err, errFailed)
		}
	}
}

func TestForgetAllowsReExecution(t *testing.T) {
	g := NewGroup()
	var callCount atomic.Int32

	// First call
	g.Do("key", func() (interface{}, error) {
		callCount.Add(1)
		return "first", nil
	})

	// Forget the key
	g.Forget("key")

	// Second call should execute fn again
	result := g.Do("key", func() (interface{}, error) {
		callCount.Add(1)
		return "second", nil
	})

	if count := callCount.Load(); count != 2 {
		t.Errorf("fn called %d times; want 2 (once before Forget, once after)", count)
	}
	if result.Value != "second" {
		t.Errorf("Value = %v; want 'second'", result.Value)
	}
	if result.Shared {
		t.Error("Shared = true; want false for fresh execution after Forget")
	}
}

func TestForgetDuringInFlightAllowsNewExecution(t *testing.T) {
	g := NewGroup()
	var callCount atomic.Int32

	started := make(chan struct{})
	proceed := make(chan struct{})

	var wg sync.WaitGroup

	// Start an in-flight call
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.Do("key", func() (interface{}, error) {
			callCount.Add(1)
			close(started)
			<-proceed
			return "first", nil
		})
	}()

	<-started

	// Forget while in-flight
	g.Forget("key")

	// New call should start a fresh execution
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.Do("key", func() (interface{}, error) {
			callCount.Add(1)
			return "second", nil
		})
	}()

	close(proceed)
	wg.Wait()

	if count := callCount.Load(); count != 2 {
		t.Errorf("fn called %d times; want 2 (Forget should allow re-execution)", count)
	}
}

func TestForgetSafeForNonexistentKey(t *testing.T) {
	g := NewGroup()
	// Should not panic
	g.Forget("nonexistent")
}

func TestDoChanReturnsResultOnChannel(t *testing.T) {
	g := NewGroup()

	ch := g.DoChan("key", func() (interface{}, error) {
		return "hello", nil
	})

	select {
	case r := <-ch:
		if r.Value != "hello" {
			t.Errorf("Value = %v; want 'hello'", r.Value)
		}
		if r.Err != nil {
			t.Errorf("Err = %v; want nil", r.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DoChan result")
	}
}

func TestDoChanIsNonBlocking(t *testing.T) {
	g := NewGroup()

	proceed := make(chan struct{})

	ch := g.DoChan("key", func() (interface{}, error) {
		<-proceed
		return "done", nil
	})

	// This should return immediately — DoChan is non-blocking
	select {
	case <-ch:
		t.Fatal("DoChan channel should not have result yet")
	default:
		// Expected: channel has no result yet
	}

	close(proceed)

	select {
	case r := <-ch:
		if r.Value != "done" {
			t.Errorf("Value = %v; want 'done'", r.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DoChan result")
	}
}

func TestDoChanDeduplicatesConcurrentCalls(t *testing.T) {
	g := NewGroup()
	var callCount atomic.Int32

	started := make(chan struct{})
	proceed := make(chan struct{})

	ch1 := g.DoChan("key", func() (interface{}, error) {
		callCount.Add(1)
		close(started)
		<-proceed
		return "result", nil
	})

	<-started // fn is in-flight

	ch2 := g.DoChan("key", func() (interface{}, error) {
		callCount.Add(1)
		return "wrong", nil
	})

	close(proceed)

	r1 := <-ch1
	r2 := <-ch2

	if count := callCount.Load(); count != 1 {
		t.Errorf("fn called %d times; want 1", count)
	}
	if r1.Value != "result" || r2.Value != "result" {
		t.Errorf("results = (%v, %v); want ('result', 'result')", r1.Value, r2.Value)
	}
}

func TestDoChanErrorPropagated(t *testing.T) {
	g := NewGroup()
	errBad := errors.New("bad request")

	ch := g.DoChan("key", func() (interface{}, error) {
		return nil, errBad
	})

	select {
	case r := <-ch:
		if !errors.Is(r.Err, errBad) {
			t.Errorf("Err = %v; want %v", r.Err, errBad)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DoChan result")
	}
}

func TestCleanupAfterCompletion(t *testing.T) {
	g := NewGroup()

	// First call completes
	g.Do("key", func() (interface{}, error) {
		return "first", nil
	})

	// Second call for the same key should execute fn again (not share with first)
	var secondCalled bool
	result := g.Do("key", func() (interface{}, error) {
		secondCalled = true
		return "second", nil
	})

	if !secondCalled {
		t.Error("fn should have been called again after first call completed")
	}
	if result.Shared {
		t.Error("Shared = true; want false for call after previous completion")
	}
	if result.Value != "second" {
		t.Errorf("Value = %v; want 'second'", result.Value)
	}
}

// TestNoGoroutineLeak verifies that goroutines are properly cleaned up after use.
func TestNoGoroutineLeak(t *testing.T) {
	// Allow any background goroutines to settle
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	g := NewGroup()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			g.Do("key", func() (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return n, nil
			})
		}(i)
	}
	wg.Wait()

	// Use DoChan as well
	for i := 0; i < 10; i++ {
		ch := g.DoChan("chan-key", func() (interface{}, error) {
			return i, nil
		})
		<-ch
	}

	// Allow goroutines to exit
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Allow a small margin for test infrastructure goroutines
	if after > before+2 {
		t.Errorf("goroutine leak: before=%d, after=%d (delta=%d)", before, after, after-before)
	}
}

// TestConcurrentDoSafety uses -race to verify no data races.
func TestConcurrentDoSafety(t *testing.T) {
	g := NewGroup()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%5))
			r := g.Do(key, func() (interface{}, error) {
				time.Sleep(time.Millisecond)
				return n, nil
			})
			_ = r.Value
			_ = r.Err
			_ = r.Shared
		}(i)
	}

	// Also mix in Forget calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%5))
			time.Sleep(time.Duration(n) * time.Millisecond)
			g.Forget(key)
		}(i)
	}

	// And DoChan calls
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%5))
			ch := g.DoChan(key, func() (interface{}, error) {
				return n, nil
			})
			<-ch
		}(i)
	}

	wg.Wait()
}
