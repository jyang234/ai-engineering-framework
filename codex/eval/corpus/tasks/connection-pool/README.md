# Task: Generic Connection Pool

Implement a generic connection pool that manages reusable connections with health checking and idle timeout.

## Requirements

Implement the following in `pool.go`:

1. **`Conn` interface**:
   ```go
   type Conn interface {
       Close() error
       IsAlive() bool
   }
   ```

2. **`PoolConfig` struct** with:
   - `MaxSize int` — maximum pool size
   - `MinIdle int` — minimum idle connections to maintain
   - `MaxIdleTime time.Duration` — close idle connections after this duration
   - `Factory func() (Conn, error)` — creates new connections

3. **`Pool` struct** created via:
   **`NewPool(config PoolConfig) *Pool`**

4. **`(*Pool) Get(ctx context.Context) (Conn, error)`** that:
   - Returns an idle connection if available (checking IsAlive first)
   - Creates a new connection if pool is not at max size
   - Blocks until a connection is available or context is cancelled
   - Must be safe for concurrent use

5. **`(*Pool) Put(conn Conn)`** that:
   - Returns a connection to the pool for reuse
   - Discards the connection (calls Close) if pool is at capacity or conn is not alive
   - Must be safe for concurrent use

6. **`(*Pool) Close() error`** that:
   - Closes all idle connections
   - Waits for checked-out connections to be returned, then closes them
   - Returns the first close error encountered

7. **`(*Pool) Stats() PoolStats`** where PoolStats has:
   - `Idle int` — connections waiting in pool
   - `InUse int` — connections currently checked out
   - `Total int` — Idle + InUse

## Constraints

- Use only the Go standard library
- Must be safe for concurrent Get/Put from multiple goroutines
- Put must ALWAYS be called for connections obtained from Get (even on error paths)
- Dead connections (IsAlive() == false) must be discarded, not returned to pool
- Get must respect context cancellation
