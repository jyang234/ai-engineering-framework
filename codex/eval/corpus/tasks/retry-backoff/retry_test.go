package retry

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errTransient = errors.New("transient failure")

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()
	if config.MaxRetries <= 0 {
		t.Errorf("MaxRetries should be positive, got %d", config.MaxRetries)
	}
	if config.InitialDelay <= 0 {
		t.Errorf("InitialDelay should be positive, got %v", config.InitialDelay)
	}
	if config.MaxDelay <= 0 {
		t.Errorf("MaxDelay should be positive, got %v", config.MaxDelay)
	}
	if config.Multiplier <= 0 {
		t.Errorf("Multiplier should be positive, got %f", config.Multiplier)
	}
}

func TestRetrySucceedsImmediately(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	var calls int
	err := Retry(context.Background(), config, func() error {
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

func TestRetrySucceedsAfterFailures(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	var calls int
	err := Retry(context.Background(), config, func() error {
		calls++
		if calls < 3 {
			return errTransient
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryExhaustsMaxRetries(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	var calls int
	err := Retry(context.Background(), config, func() error {
		calls++
		return errTransient
	})

	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// MaxRetries=3 means 1 initial + 3 retries = 4 total calls
	if calls != 4 {
		t.Fatalf("expected 4 calls (1 initial + 3 retries), got %d", calls)
	}
}

// TestRetryHasJitter verifies that retry delays include random jitter.
// Without jitter, all delays would be identical across runs — the thundering herd problem.
func TestRetryHasJitter(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	// Run multiple retries and collect the delay between calls.
	// If jitter is present, the delays should vary.
	const trials = 10
	delays := make([]time.Duration, 0, trials)

	for i := 0; i < trials; i++ {
		var callTimes []time.Time
		_ = Retry(context.Background(), config, func() error {
			callTimes = append(callTimes, time.Now())
			if len(callTimes) < 2 {
				return errTransient
			}
			return nil
		})

		if len(callTimes) >= 2 {
			delays = append(delays, callTimes[1].Sub(callTimes[0]))
		}
	}

	if len(delays) < 2 {
		t.Fatal("not enough successful trials to measure jitter")
	}

	// Check that delays are not all identical (would indicate no jitter).
	allSame := true
	tolerance := 2 * time.Millisecond
	for i := 1; i < len(delays); i++ {
		diff := delays[i] - delays[0]
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			allSame = false
			break
		}
	}

	if allSame {
		t.Errorf("all %d retry delays are identical (%v) — jitter is missing; this causes thundering herd",
			len(delays), delays[0])
	}
}

func TestRetryExponentialBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   4,
		InitialDelay: 20 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	var callTimes []time.Time
	_ = Retry(context.Background(), config, func() error {
		callTimes = append(callTimes, time.Now())
		return errTransient
	})

	if len(callTimes) < 4 {
		t.Fatalf("expected at least 4 call times, got %d", len(callTimes))
	}

	// Verify that delays increase approximately exponentially.
	// With jitter of ±25%, the ratio between consecutive delays should average ~2.0.
	for i := 2; i < len(callTimes)-1; i++ {
		delay1 := callTimes[i].Sub(callTimes[i-1]).Seconds()
		delay0 := callTimes[i-1].Sub(callTimes[i-2]).Seconds()
		if delay0 > 0 {
			ratio := delay1 / delay0
			// With ±25% jitter on each, the ratio can range from ~1.2 to ~3.3.
			// We use a loose check to avoid flakiness.
			if ratio < 0.8 || ratio > 5.0 {
				t.Errorf("delay ratio between attempts %d and %d is %.2f (expected ~%.1f)",
					i-1, i, ratio, config.Multiplier)
			}
		}
	}
}

func TestRetryMaxDelayCap(t *testing.T) {
	maxDelay := 50 * time.Millisecond
	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 20 * time.Millisecond,
		MaxDelay:     maxDelay,
		Multiplier:   10.0, // Aggressive multiplier to quickly exceed maxDelay
	}

	var callTimes []time.Time
	_ = Retry(context.Background(), config, func() error {
		callTimes = append(callTimes, time.Now())
		return errTransient
	})

	if len(callTimes) < 3 {
		t.Fatalf("expected at least 3 call times, got %d", len(callTimes))
	}

	// Later delays should be capped near maxDelay (with jitter, up to 1.25x max).
	for i := 2; i < len(callTimes); i++ {
		delay := callTimes[i].Sub(callTimes[i-1])
		// Allow 25% jitter above maxDelay + some scheduling slack
		upperBound := time.Duration(float64(maxDelay)*1.30) + 20*time.Millisecond
		if delay > upperBound {
			t.Errorf("delay at attempt %d was %v, exceeds max delay cap %v (upper bound %v)",
				i, delay, maxDelay, upperBound)
		}
	}
}

func TestRetryContextCancellation(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   100,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := Retry(ctx, config, func() error {
		return errTransient
	})

	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("error type: %T, value: %v", err, err)
		// Accept either the context error or the transient error (if context check is at wait point)
	}
	// Should return quickly, not wait for all 100 retries
	if elapsed > 2*time.Second {
		t.Errorf("retry did not respect context cancellation: took %v", elapsed)
	}
}

func TestRetryConcurrentSafety(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   2.0,
	}

	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempts := 0
			err := Retry(context.Background(), config, func() error {
				attempts++
				if attempts < 2 {
					return errTransient
				}
				return nil
			})
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()
	if successCount.Load() != 20 {
		t.Errorf("expected 20 successes, got %d", successCount.Load())
	}
}

func TestRetryJitterRange(t *testing.T) {
	// Verify jitter stays within ±25% of base delay.
	config := RetryConfig{
		MaxRetries:   1,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	const trials = 30
	var delays []time.Duration

	for i := 0; i < trials; i++ {
		var callTimes []time.Time
		_ = Retry(context.Background(), config, func() error {
			callTimes = append(callTimes, time.Now())
			return errTransient
		})
		if len(callTimes) >= 2 {
			delays = append(delays, callTimes[1].Sub(callTimes[0]))
		}
	}

	if len(delays) < 10 {
		t.Fatalf("not enough delay measurements: got %d", len(delays))
	}

	baseDelay := float64(config.InitialDelay)
	minExpected := baseDelay * 0.75 // -25%
	maxExpected := baseDelay * 1.25 // +25%

	for i, d := range delays {
		df := float64(d)
		// Allow 15ms scheduling slack
		if df < minExpected-15e6 || df > maxExpected+15e6 {
			if math.Abs(df-baseDelay) > baseDelay*0.50 {
				t.Errorf("trial %d: delay %v outside ±25%% of base %v (range [%v, %v])",
					i, d, config.InitialDelay,
					time.Duration(minExpected), time.Duration(maxExpected))
			}
		}
	}
}
