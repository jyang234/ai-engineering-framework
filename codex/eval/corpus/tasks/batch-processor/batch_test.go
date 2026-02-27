package batch

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlushOnMaxSize(t *testing.T) {
	var flushed [][]interface{}
	var mu sync.Mutex

	p := NewProcessor(Config{
		MaxSize: 3,
		MaxWait: 10 * time.Second,
		FlushFunc: func(items []interface{}) error {
			mu.Lock()
			flushed = append(flushed, items)
			mu.Unlock()
			return nil
		},
	})
	p.Start()

	p.Add("a")
	p.Add("b")
	p.Add("c") // should trigger flush

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 flush at MaxSize, got %d", count)
	}

	ctx := context.Background()
	p.Shutdown(ctx)
}

func TestFlushOnTimer(t *testing.T) {
	var flushed atomic.Int32

	p := NewProcessor(Config{
		MaxSize: 100,
		MaxWait: 100 * time.Millisecond,
		FlushFunc: func(items []interface{}) error {
			flushed.Add(1)
			return nil
		},
	})
	p.Start()

	p.Add("a")
	p.Add("b")

	// Wait for timer flush
	time.Sleep(250 * time.Millisecond)

	if flushed.Load() < 1 {
		t.Error("timer should have triggered flush")
	}

	ctx := context.Background()
	p.Shutdown(ctx)
}

func TestManualFlush(t *testing.T) {
	var flushedItems []interface{}
	var mu sync.Mutex

	p := NewProcessor(Config{
		MaxSize: 100,
		MaxWait: 10 * time.Second,
		FlushFunc: func(items []interface{}) error {
			mu.Lock()
			flushedItems = append(flushedItems, items...)
			mu.Unlock()
			return nil
		},
	})
	p.Start()

	p.Add("x")
	p.Add("y")
	p.Flush()

	mu.Lock()
	count := len(flushedItems)
	mu.Unlock()

	if count != 2 {
		t.Errorf("manual flush should have flushed 2 items, got %d", count)
	}

	ctx := context.Background()
	p.Shutdown(ctx)
}

// TestShutdownFlushesRemaining is critical: shutdown must not lose data.
func TestShutdownFlushesRemaining(t *testing.T) {
	var flushedItems []interface{}
	var mu sync.Mutex

	p := NewProcessor(Config{
		MaxSize: 100,
		MaxWait: 10 * time.Second,
		FlushFunc: func(items []interface{}) error {
			mu.Lock()
			flushedItems = append(flushedItems, items...)
			mu.Unlock()
			return nil
		},
	})
	p.Start()

	p.Add("a")
	p.Add("b")
	p.Add("c")

	// Shutdown should flush the 3 pending items
	ctx := context.Background()
	p.Shutdown(ctx)

	mu.Lock()
	count := len(flushedItems)
	mu.Unlock()

	if count != 3 {
		t.Errorf("shutdown should have flushed 3 remaining items, got %d — DATA LOSS", count)
	}
}

func TestAddAfterShutdown(t *testing.T) {
	p := NewProcessor(Config{
		MaxSize:   10,
		MaxWait:   time.Second,
		FlushFunc: func(items []interface{}) error { return nil },
	})
	p.Start()
	p.Shutdown(context.Background())

	err := p.Add("late")
	if !errors.Is(err, ErrProcessorClosed) {
		t.Errorf("Add after Shutdown should return ErrProcessorClosed, got %v", err)
	}
}

func TestShutdownIdempotent(t *testing.T) {
	p := NewProcessor(Config{
		MaxSize:   10,
		MaxWait:   time.Second,
		FlushFunc: func(items []interface{}) error { return nil },
	})
	p.Start()
	p.Shutdown(context.Background())
	p.Shutdown(context.Background()) // should not panic
}

func TestPending(t *testing.T) {
	p := NewProcessor(Config{
		MaxSize:   100,
		MaxWait:   10 * time.Second,
		FlushFunc: func(items []interface{}) error { return nil },
	})
	p.Start()

	if p.Pending() != 0 {
		t.Errorf("Pending() = %d; want 0", p.Pending())
	}
	p.Add("a")
	p.Add("b")
	if p.Pending() != 2 {
		t.Errorf("Pending() = %d; want 2", p.Pending())
	}
	p.Flush()
	if p.Pending() != 0 {
		t.Errorf("Pending() after flush = %d; want 0", p.Pending())
	}

	p.Shutdown(context.Background())
}

func TestFlushFuncError(t *testing.T) {
	errFlush := errors.New("flush failed")
	p := NewProcessor(Config{
		MaxSize:   100,
		MaxWait:   10 * time.Second,
		FlushFunc: func(items []interface{}) error { return errFlush },
	})
	p.Start()

	p.Add("a")
	err := p.Flush()
	if !errors.Is(err, errFlush) {
		t.Errorf("Flush() error = %v; want %v", err, errFlush)
	}

	p.Shutdown(context.Background())
}

// TestConcurrentAdd detects races with -race.
func TestConcurrentAdd(t *testing.T) {
	var totalFlushed atomic.Int32

	p := NewProcessor(Config{
		MaxSize: 10,
		MaxWait: 50 * time.Millisecond,
		FlushFunc: func(items []interface{}) error {
			totalFlushed.Add(int32(len(items)))
			return nil
		},
	})
	p.Start()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			p.Add(n)
		}(i)
	}
	wg.Wait()

	p.Shutdown(context.Background())

	if totalFlushed.Load() != 100 {
		t.Errorf("total flushed = %d; want 100 — items were lost", totalFlushed.Load())
	}
}

func TestEmptyFlushIsNoop(t *testing.T) {
	flushCalled := false
	p := NewProcessor(Config{
		MaxSize: 10,
		MaxWait: 10 * time.Second,
		FlushFunc: func(items []interface{}) error {
			flushCalled = true
			return nil
		},
	})
	p.Start()
	p.Flush() // nothing to flush

	if flushCalled {
		t.Error("FlushFunc should not be called when batch is empty")
	}

	p.Shutdown(context.Background())
}
