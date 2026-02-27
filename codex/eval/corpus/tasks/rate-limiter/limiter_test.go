package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLimiterBurstAvailable(t *testing.T) {
	lim := NewLimiter(10.0, 5)
	tokens := lim.Tokens()
	if tokens < 4.5 || tokens > 5.5 {
		t.Errorf("new limiter should have ~burst tokens, got %.1f", tokens)
	}
}

func TestAllowConsumesToken(t *testing.T) {
	lim := NewLimiter(10.0, 5)

	// Should be able to consume burst tokens
	for i := 0; i < 5; i++ {
		if !lim.Allow() {
			t.Errorf("Allow() returned false at call %d (burst=5)", i+1)
		}
	}

	// Next should fail (no tokens left)
	if lim.Allow() {
		t.Error("Allow() should return false when no tokens available")
	}
}

func TestAllowNAtomicConsumption(t *testing.T) {
	lim := NewLimiter(10.0, 10)

	// Should consume 7 tokens atomically
	if !lim.AllowN(7) {
		t.Error("AllowN(7) should succeed with 10 tokens available")
	}

	// Only 3 left, asking for 5 should fail without consuming
	if lim.AllowN(5) {
		t.Error("AllowN(5) should fail with only ~3 tokens left")
	}

	// The 3 tokens should still be there
	remaining := lim.Tokens()
	if remaining < 2.5 || remaining > 3.5 {
		t.Errorf("expected ~3 tokens remaining, got %.1f", remaining)
	}
}

func TestTokenRefill(t *testing.T) {
	lim := NewLimiter(100.0, 100) // 100 tokens/sec

	// Drain all tokens
	for lim.Allow() {
	}

	// Wait 50ms — should get ~5 tokens
	time.Sleep(50 * time.Millisecond)

	tokens := lim.Tokens()
	if tokens < 3.0 || tokens > 8.0 {
		t.Errorf("expected ~5 tokens after 50ms at 100/s, got %.1f", tokens)
	}
}

// TestTokenNeverExceedsBurst verifies tokens are capped at burst capacity.
// This catches the bug where refill logic doesn't cap at burst, allowing
// unlimited token accumulation.
func TestTokenNeverExceedsBurst(t *testing.T) {
	lim := NewLimiter(1000.0, 10) // High rate, low burst

	// Wait longer than it takes to fill the bucket many times over
	time.Sleep(100 * time.Millisecond)

	tokens := lim.Tokens()
	if tokens > 10.5 {
		t.Errorf("tokens (%.1f) exceed burst capacity (10) — refill must cap at burst", tokens)
	}
}

func TestWaitBlocksUntilToken(t *testing.T) {
	lim := NewLimiter(100.0, 1) // 1 token burst, 100/sec refill

	// Consume the burst
	lim.Allow()

	// Wait should block briefly then succeed
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := lim.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if elapsed < 5*time.Millisecond {
		t.Error("Wait returned too quickly — should have blocked for token refill")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}
}

func TestWaitRespectsContextCancellation(t *testing.T) {
	lim := NewLimiter(0.001, 1) // Very slow refill

	// Consume the burst
	lim.Allow()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := lim.Wait(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Wait should have returned context error")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Wait did not respect context cancellation: took %v", elapsed)
	}
}

func TestZeroRateNeverRefills(t *testing.T) {
	lim := NewLimiter(0, 5)

	// Consume all tokens
	for i := 0; i < 5; i++ {
		lim.Allow()
	}

	time.Sleep(50 * time.Millisecond)

	if lim.Allow() {
		t.Error("zero rate should never refill tokens")
	}
}

func TestNegativeRateNeverRefills(t *testing.T) {
	lim := NewLimiter(-1.0, 5)

	// Consume all tokens
	for i := 0; i < 5; i++ {
		lim.Allow()
	}

	time.Sleep(50 * time.Millisecond)

	if lim.Allow() {
		t.Error("negative rate should never refill tokens")
	}
}

// TestConcurrentAccess runs with -race to detect data races.
// This catches the common bug of not synchronizing token access.
func TestConcurrentAccess(t *testing.T) {
	lim := NewLimiter(1000.0, 100)

	var wg sync.WaitGroup
	var allowed atomic.Int32
	var denied atomic.Int32

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if lim.Allow() {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}

	wg.Wait()

	total := allowed.Load() + denied.Load()
	if total != 200 {
		t.Errorf("expected 200 total, got %d", total)
	}
	if allowed.Load() > 100 {
		t.Errorf("allowed %d > burst capacity 100", allowed.Load())
	}

	t.Logf("allowed=%d, denied=%d", allowed.Load(), denied.Load())
}

func TestConcurrentAllowN(t *testing.T) {
	lim := NewLimiter(1000.0, 50)

	var wg sync.WaitGroup
	var totalConsumed atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if lim.AllowN(5) {
				totalConsumed.Add(5)
			}
		}()
	}

	wg.Wait()

	// At most 50 tokens could be consumed (burst capacity)
	if totalConsumed.Load() > 50 {
		t.Errorf("consumed %d tokens exceeds burst capacity 50", totalConsumed.Load())
	}
}
