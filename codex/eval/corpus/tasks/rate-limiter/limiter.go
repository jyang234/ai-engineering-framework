// Package limiter implements a token bucket rate limiter.
package limiter

import (
	"context"
)

// Limiter controls the rate of operations using a token bucket algorithm.
type Limiter struct {
	rate  float64
	burst int
}

// NewLimiter creates a rate limiter that allows `rate` tokens per second
// with a maximum burst size of `burst`.
func NewLimiter(rate float64, burst int) *Limiter {
	// TODO: implement
	return &Limiter{rate: rate, burst: burst}
}

// Allow reports whether one token can be consumed.
// It is shorthand for AllowN(1).
func (l *Limiter) Allow() bool {
	// TODO: implement
	return false
}

// AllowN reports whether n tokens can be consumed atomically.
// It returns false without consuming any tokens if fewer than n are available.
func (l *Limiter) AllowN(n int) bool {
	// TODO: implement
	return false
}

// Wait blocks until a token is available or the context is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	// TODO: implement
	return ctx.Err()
}

// Tokens returns the current number of available tokens.
func (l *Limiter) Tokens() float64 {
	// TODO: implement
	return 0
}
