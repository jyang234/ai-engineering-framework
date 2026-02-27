package pipeline

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewPipelineCreates(t *testing.T) {
	s := func(ctx context.Context, in interface{}) (interface{}, error) { return in, nil }
	p := NewPipeline(4, s)
	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
}

func TestBasicProcessingThroughStages(t *testing.T) {
	double := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in.(int) * 2, nil
	}
	addOne := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in.(int) + 1, nil
	}

	p := NewPipeline(2, double, addOne)

	input := make(chan interface{})
	go func() {
		for i := 0; i < 10; i++ {
			input <- i
		}
		close(input)
	}()

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	results := make(map[int]bool)
	for r := range output {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		results[r.Value.(int)] = true
	}

	// Each input i should produce i*2 + 1
	for i := 0; i < 10; i++ {
		expected := i*2 + 1
		if !results[expected] {
			t.Errorf("missing result for input %d: expected %d", i, expected)
		}
	}
}

func TestErrorPropagationFromFailingStage(t *testing.T) {
	errBoom := errors.New("stage failed")

	stage0 := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in, nil
	}
	stage1 := func(ctx context.Context, in interface{}) (interface{}, error) {
		if in.(int) == 3 {
			return nil, errBoom
		}
		return in, nil
	}

	p := NewPipeline(2, stage0, stage1)

	input := make(chan interface{})
	go func() {
		for i := 0; i < 5; i++ {
			input <- i
		}
		close(input)
	}()

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var errorCount int
	var successCount int
	for r := range output {
		if r.Err != nil {
			errorCount++
			if r.StageIndex != 1 {
				t.Errorf("expected error at stage 1, got stage %d", r.StageIndex)
			}
			if !errors.Is(r.Err, errBoom) {
				t.Errorf("expected errBoom, got %v", r.Err)
			}
		} else {
			successCount++
		}
	}

	if errorCount != 1 {
		t.Errorf("expected 1 error result, got %d", errorCount)
	}
	if successCount != 4 {
		t.Errorf("expected 4 success results, got %d", successCount)
	}
}

func TestContextCancellationStopsWorkers(t *testing.T) {
	var processed atomic.Int32

	slowStage := func(ctx context.Context, in interface{}) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			processed.Add(1)
			return in, nil
		}
	}

	p := NewPipeline(4, slowStage)

	input := make(chan interface{})
	go func() {
		for i := 0; ; i++ {
			select {
			case input <- i:
			case <-time.After(3 * time.Second):
				close(input)
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	// Let some items get picked up
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Drain the output channel — it must close promptly
	done := make(chan struct{})
	go func() {
		for range output {
		}
		close(done)
	}()

	select {
	case <-done:
		// Output channel closed after cancellation
	case <-time.After(3 * time.Second):
		t.Fatal("output channel not closed after context cancellation — workers may be stuck")
	}
}

func TestNoGoroutineLeakAfterProcessCompletes(t *testing.T) {
	// Stabilize goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	passthrough := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in, nil
	}

	p := NewPipeline(8, passthrough)

	input := make(chan interface{})
	go func() {
		for i := 0; i < 50; i++ {
			input <- i
		}
		close(input)
	}()

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	// Drain all results
	for range output {
	}

	// Give goroutines time to clean up
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before
	if leaked > 2 { // Allow slack for runtime goroutines
		t.Errorf("goroutine leak: %d goroutines before, %d after (%d leaked)", before, after, leaked)
	}
}

func TestNoGoroutineLeakOnCancellation(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	slowStage := func(ctx context.Context, in interface{}) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return in, nil
		}
	}

	p := NewPipeline(4, slowStage)

	input := make(chan interface{})
	go func() {
		defer close(input)
		for i := 0; i < 100; i++ {
			input <- i
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	// Cancel immediately
	cancel()

	// Drain output
	for range output {
	}

	// Give goroutines time to clean up
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before
	if leaked > 2 {
		t.Errorf("goroutine leak on cancellation: %d before, %d after (%d leaked)", before, after, leaked)
	}
}

func TestConcurrentProcessingFanOut(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	slowStage := func(ctx context.Context, in interface{}) (interface{}, error) {
		cur := concurrent.Add(1)
		// Track max concurrency
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		concurrent.Add(-1)
		return in, nil
	}

	workers := 4
	p := NewPipeline(workers, slowStage)

	input := make(chan interface{})
	go func() {
		for i := 0; i < 20; i++ {
			input <- i
		}
		close(input)
	}()

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var count int
	for range output {
		count++
	}

	if count != 20 {
		t.Errorf("expected 20 results, got %d", count)
	}

	mc := maxConcurrent.Load()
	if mc < 2 {
		t.Errorf("expected fan-out concurrency >= 2, but max concurrent was %d — fan-out may not be working", mc)
	}
}

func TestOutputChannelClosedAfterAllInputConsumed(t *testing.T) {
	passthrough := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in, nil
	}

	p := NewPipeline(2, passthrough)

	input := make(chan interface{})
	go func() {
		for i := 0; i < 5; i++ {
			input <- i
		}
		close(input)
	}()

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	// Drain results
	done := make(chan struct{})
	go func() {
		for range output {
		}
		// If we reach here, output was closed
		close(done)
	}()

	select {
	case <-done:
		// Output channel closed properly
	case <-time.After(5 * time.Second):
		t.Fatal("output channel was never closed after all input consumed")
	}
}

func TestEmptyInputProducesEmptyOutput(t *testing.T) {
	passthrough := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in, nil
	}

	p := NewPipeline(4, passthrough)

	input := make(chan interface{})
	close(input) // Immediately closed — no items

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var count int
	done := make(chan struct{})
	go func() {
		for range output {
			count++
		}
		close(done)
	}()

	select {
	case <-done:
		if count != 0 {
			t.Errorf("expected 0 results for empty input, got %d", count)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("output channel was never closed for empty input")
	}
}

func TestNilInputReturnsError(t *testing.T) {
	passthrough := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in, nil
	}

	p := NewPipeline(2, passthrough)

	_, err := p.Process(context.Background(), nil)
	if !errors.Is(err, ErrNilInput) {
		t.Errorf("expected ErrNilInput for nil input channel, got %v", err)
	}
}

func TestMultipleStagesAppliedInOrder(t *testing.T) {
	// Build a trace string to verify stage ordering
	stage0 := func(ctx context.Context, in interface{}) (interface{}, error) {
		return fmt.Sprintf("%v->s0", in), nil
	}
	stage1 := func(ctx context.Context, in interface{}) (interface{}, error) {
		return fmt.Sprintf("%v->s1", in), nil
	}
	stage2 := func(ctx context.Context, in interface{}) (interface{}, error) {
		return fmt.Sprintf("%v->s2", in), nil
	}

	p := NewPipeline(1, stage0, stage1, stage2)

	input := make(chan interface{}, 1)
	input <- "start"
	close(input)

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	r := <-output
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}

	expected := "start->s0->s1->s2"
	if r.Value != expected {
		t.Errorf("stages not applied in order: got %q, want %q", r.Value, expected)
	}
}

func TestErrorAtEarlyStageSkipsLaterStages(t *testing.T) {
	errEarly := errors.New("early failure")
	var stage2Called atomic.Int32

	stage0 := func(ctx context.Context, in interface{}) (interface{}, error) {
		return nil, errEarly
	}
	stage1 := func(ctx context.Context, in interface{}) (interface{}, error) {
		stage2Called.Add(1)
		return in, nil
	}

	p := NewPipeline(1, stage0, stage1)

	input := make(chan interface{}, 1)
	input <- "x"
	close(input)

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	r := <-output
	if r.StageIndex != 0 {
		t.Errorf("expected error at stage 0, got stage %d", r.StageIndex)
	}
	if !errors.Is(r.Err, errEarly) {
		t.Errorf("expected errEarly, got %v", r.Err)
	}
	if stage2Called.Load() != 0 {
		t.Error("stage 1 should not be called after stage 0 fails")
	}
}

func TestLargeVolumeProcessing(t *testing.T) {
	increment := func(ctx context.Context, in interface{}) (interface{}, error) {
		return in.(int) + 1, nil
	}

	p := NewPipeline(8, increment, increment, increment) // +3

	const n = 1000
	input := make(chan interface{})
	go func() {
		for i := 0; i < n; i++ {
			input <- i
		}
		close(input)
	}()

	ctx := context.Background()
	output, err := p.Process(ctx, input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var mu sync.Mutex
	results := make(map[int]bool)
	for r := range output {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		mu.Lock()
		results[r.Value.(int)] = true
		mu.Unlock()
	}

	for i := 0; i < n; i++ {
		expected := i + 3
		if !results[expected] {
			t.Errorf("missing result for input %d: expected %d", i, expected)
		}
	}
}
