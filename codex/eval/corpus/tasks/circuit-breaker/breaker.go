// Package breaker implements the circuit breaker pattern.
package breaker

import (
	"errors"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota
	StateOpen
	StateHalfOpen
)

// Sentinel errors.
var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// Config holds circuit breaker configuration.
type Config struct {
	MaxFailures         int
	ResetTimeout        time.Duration
	HalfOpenMaxRequests int
}

// CircuitBreaker protects calls to external services.
type CircuitBreaker struct {
	config Config
}

// NewCircuitBreaker creates a circuit breaker with the given config.
func NewCircuitBreaker(config Config) *CircuitBreaker {
	// TODO: implement
	return &CircuitBreaker{config: config}
}

// Do executes the given function through the circuit breaker.
func (cb *CircuitBreaker) Do(fn func() error) error {
	// TODO: implement
	return fn()
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	// TODO: implement
	return StateClosed
}

// Reset manually resets the circuit breaker to Closed state.
func (cb *CircuitBreaker) Reset() {
	// TODO: implement
}
