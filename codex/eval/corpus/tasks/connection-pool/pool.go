// Package pool implements a generic connection pool.
package pool

import (
	"context"
	"errors"
	"time"
)

// ErrPoolClosed is returned when operating on a closed pool.
var ErrPoolClosed = errors.New("pool is closed")

// Conn is the interface for pooled connections.
type Conn interface {
	Close() error
	IsAlive() bool
}

// PoolConfig configures the connection pool.
type PoolConfig struct {
	MaxSize     int
	MinIdle     int
	MaxIdleTime time.Duration
	Factory     func() (Conn, error)
}

// PoolStats holds pool statistics.
type PoolStats struct {
	Idle  int
	InUse int
	Total int
}

// Pool manages reusable connections.
type Pool struct {
	config PoolConfig
}

// NewPool creates a connection pool.
func NewPool(config PoolConfig) *Pool {
	// TODO: implement
	return &Pool{config: config}
}

// Get returns a connection from the pool, creating one if necessary.
func (p *Pool) Get(ctx context.Context) (Conn, error) {
	// TODO: implement
	return nil, ErrPoolClosed
}

// Put returns a connection to the pool.
func (p *Pool) Put(conn Conn) {
	// TODO: implement
}

// Close closes all connections in the pool.
func (p *Pool) Close() error {
	// TODO: implement
	return nil
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	// TODO: implement
	return PoolStats{}
}
