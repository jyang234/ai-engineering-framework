// Package retry provides a retry mechanism with exponential backoff and jitter.
package retry

import (
	"context"
	"time"
)

// RetryConfig holds configuration for the retry mechanism.
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	// TODO: implement
	return RetryConfig{}
}

// Retry calls fn with exponential backoff and jitter on failure.
// It respects context cancellation and returns the last error after exhausting retries.
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	// TODO: implement
	return fn()
}
