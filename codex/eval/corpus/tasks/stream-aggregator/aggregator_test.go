package aggregator

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAggregatesSumCountMinMax(t *testing.T) {
	var results []WindowResult
	var mu sync.Mutex

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		mu.Lock()
		results = append(results, wr...)
		mu.Unlock()
	})
	a.Start()

	now := time.Now()
	a.Emit(Event{Key: "cpu", Value: 10.0, Timestamp: now})
	a.Emit(Event{Key: "cpu", Value: 20.0, Timestamp: now})
	a.Emit(Event{Key: "cpu", Value: 5.0, Timestamp: now})

	a.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Key != "cpu" {
		t.Errorf("Key = %q; want %q", r.Key, "cpu")
	}
	if r.Sum != 35.0 {
		t.Errorf("Sum = %f; want 35.0", r.Sum)
	}
	if r.Count != 3 {
		t.Errorf("Count = %d; want 3", r.Count)
	}
	if r.Min != 5.0 {
		t.Errorf("Min = %f; want 5.0", r.Min)
	}
	if r.Max != 20.0 {
		t.Errorf("Max = %f; want 20.0", r.Max)
	}

	a.Stop()
}

func TestWindowEmitsOnTimer(t *testing.T) {
	var emitCount atomic.Int32

	a := NewAggregator(100*time.Millisecond, func(wr []WindowResult) {
		emitCount.Add(1)
	})
	a.Start()

	a.Emit(Event{Key: "mem", Value: 42.0, Timestamp: time.Now()})

	// Wait for at least one timer-driven emit
	time.Sleep(250 * time.Millisecond)

	if emitCount.Load() < 1 {
		t.Error("expected at least 1 timer-driven emit, got 0")
	}

	a.Stop()
}

func TestManualFlush(t *testing.T) {
	var results []WindowResult
	var mu sync.Mutex

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		mu.Lock()
		results = append(results, wr...)
		mu.Unlock()
	})
	a.Start()

	a.Emit(Event{Key: "disk", Value: 100.0, Timestamp: time.Now()})
	a.Emit(Event{Key: "disk", Value: 200.0, Timestamp: time.Now()})
	a.Flush()

	mu.Lock()
	count := len(results)
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 result from manual flush, got %d", count)
	}

	// Flush again with no new events should not emit
	a.Flush()

	mu.Lock()
	count = len(results)
	mu.Unlock()

	if count != 1 {
		t.Errorf("empty flush should not emit, total results = %d; want 1", count)
	}

	a.Stop()
}

// TestStopFlushesPartialWindow is critical: Stop must not lose data.
func TestStopFlushesPartialWindow(t *testing.T) {
	var results []WindowResult
	var mu sync.Mutex

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		mu.Lock()
		results = append(results, wr...)
		mu.Unlock()
	})
	a.Start()

	a.Emit(Event{Key: "net", Value: 1.0, Timestamp: time.Now()})
	a.Emit(Event{Key: "net", Value: 2.0, Timestamp: time.Now()})
	a.Emit(Event{Key: "net", Value: 3.0, Timestamp: time.Now()})

	// Stop should flush the partial window
	a.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 1 {
		t.Fatalf("Stop should flush partial window, got %d results — DATA LOSS", len(results))
	}
	if results[0].Sum != 6.0 {
		t.Errorf("Sum = %f; want 6.0 — DATA LOSS", results[0].Sum)
	}
	if results[0].Count != 3 {
		t.Errorf("Count = %d; want 3 — DATA LOSS", results[0].Count)
	}
}

// TestConcurrentEmit detects races with -race flag.
func TestConcurrentEmit(t *testing.T) {
	var totalCount atomic.Int32

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		for _, r := range wr {
			totalCount.Add(int32(r.Count))
		}
	})
	a.Start()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v float64) {
			defer wg.Done()
			a.Emit(Event{Key: "concurrent", Value: v, Timestamp: time.Now()})
		}(float64(i))
	}
	wg.Wait()

	a.Stop()

	if totalCount.Load() != 100 {
		t.Errorf("total event count = %d; want 100 — events were lost", totalCount.Load())
	}
}

func TestMultipleKeysAggregateIndependently(t *testing.T) {
	var results []WindowResult
	var mu sync.Mutex

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		mu.Lock()
		results = append(results, wr...)
		mu.Unlock()
	})
	a.Start()

	now := time.Now()
	a.Emit(Event{Key: "cpu", Value: 10.0, Timestamp: now})
	a.Emit(Event{Key: "mem", Value: 100.0, Timestamp: now})
	a.Emit(Event{Key: "cpu", Value: 20.0, Timestamp: now})
	a.Emit(Event{Key: "mem", Value: 200.0, Timestamp: now})
	a.Emit(Event{Key: "disk", Value: 50.0, Timestamp: now})

	a.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 3 {
		t.Fatalf("expected 3 results (one per key), got %d", len(results))
	}

	byKey := make(map[string]WindowResult)
	for _, r := range results {
		byKey[r.Key] = r
	}

	// Check cpu
	cpu, ok := byKey["cpu"]
	if !ok {
		t.Fatal("missing result for key 'cpu'")
	}
	if cpu.Sum != 30.0 {
		t.Errorf("cpu Sum = %f; want 30.0", cpu.Sum)
	}
	if cpu.Count != 2 {
		t.Errorf("cpu Count = %d; want 2", cpu.Count)
	}
	if cpu.Min != 10.0 {
		t.Errorf("cpu Min = %f; want 10.0", cpu.Min)
	}
	if cpu.Max != 20.0 {
		t.Errorf("cpu Max = %f; want 20.0", cpu.Max)
	}

	// Check mem
	mem, ok := byKey["mem"]
	if !ok {
		t.Fatal("missing result for key 'mem'")
	}
	if mem.Sum != 300.0 {
		t.Errorf("mem Sum = %f; want 300.0", mem.Sum)
	}
	if mem.Count != 2 {
		t.Errorf("mem Count = %d; want 2", mem.Count)
	}

	// Check disk
	disk, ok := byKey["disk"]
	if !ok {
		t.Fatal("missing result for key 'disk'")
	}
	if disk.Sum != 50.0 {
		t.Errorf("disk Sum = %f; want 50.0", disk.Sum)
	}
	if disk.Count != 1 {
		t.Errorf("disk Count = %d; want 1", disk.Count)
	}
}

func TestEmptyWindowDoesNotEmit(t *testing.T) {
	emitCalled := false

	a := NewAggregator(100*time.Millisecond, func(wr []WindowResult) {
		emitCalled = true
	})
	a.Start()

	// Let a few windows pass with no events
	time.Sleep(350 * time.Millisecond)

	a.Stop()

	if emitCalled {
		t.Error("emitFn should not be called for empty windows")
	}
}

func TestWindowTimeBounds(t *testing.T) {
	var results []WindowResult
	var mu sync.Mutex

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		mu.Lock()
		results = append(results, wr...)
		mu.Unlock()
	})
	a.Start()

	a.Emit(Event{Key: "x", Value: 1.0, Timestamp: time.Now()})
	a.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.WindowStart.IsZero() {
		t.Error("WindowStart should not be zero")
	}
	if r.WindowEnd.IsZero() {
		t.Error("WindowEnd should not be zero")
	}
	if !r.WindowEnd.After(r.WindowStart) && !r.WindowEnd.Equal(r.WindowStart) {
		t.Errorf("WindowEnd (%v) should be >= WindowStart (%v)", r.WindowEnd, r.WindowStart)
	}
}

func TestSingleEventMinMaxEqual(t *testing.T) {
	var results []WindowResult
	var mu sync.Mutex

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		mu.Lock()
		results = append(results, wr...)
		mu.Unlock()
	})
	a.Start()

	a.Emit(Event{Key: "single", Value: 42.0, Timestamp: time.Now()})
	a.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Min != 42.0 || r.Max != 42.0 {
		t.Errorf("single event: Min=%f Max=%f; want both 42.0", r.Min, r.Max)
	}
}

func TestNegativeValues(t *testing.T) {
	var results []WindowResult
	var mu sync.Mutex

	a := NewAggregator(10*time.Second, func(wr []WindowResult) {
		mu.Lock()
		results = append(results, wr...)
		mu.Unlock()
	})
	a.Start()

	a.Emit(Event{Key: "temp", Value: -10.0, Timestamp: time.Now()})
	a.Emit(Event{Key: "temp", Value: -5.0, Timestamp: time.Now()})
	a.Emit(Event{Key: "temp", Value: -20.0, Timestamp: time.Now()})
	a.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if math.Abs(r.Sum-(-35.0)) > 1e-9 {
		t.Errorf("Sum = %f; want -35.0", r.Sum)
	}
	if r.Min != -20.0 {
		t.Errorf("Min = %f; want -20.0", r.Min)
	}
	if r.Max != -5.0 {
		t.Errorf("Max = %f; want -5.0", r.Max)
	}

	a.Stop()
}
