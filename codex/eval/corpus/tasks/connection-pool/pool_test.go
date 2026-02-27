package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockConn struct {
	alive  bool
	closed atomic.Bool
}

func (m *mockConn) Close() error { m.closed.Store(true); return nil }
func (m *mockConn) IsAlive() bool { return m.alive && !m.closed.Load() }

func newMockFactory() func() (Conn, error) {
	return func() (Conn, error) {
		return &mockConn{alive: true}, nil
	}
}

func TestGetCreatesConnection(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 5, Factory: newMockFactory()})
	conn, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if conn == nil {
		t.Fatal("Get returned nil connection")
	}
	p.Put(conn)
	p.Close()
}

func TestGetReusesConnection(t *testing.T) {
	var created atomic.Int32
	p := NewPool(PoolConfig{
		MaxSize: 5,
		Factory: func() (Conn, error) {
			created.Add(1)
			return &mockConn{alive: true}, nil
		},
	})

	conn1, _ := p.Get(context.Background())
	p.Put(conn1)
	conn2, _ := p.Get(context.Background())
	p.Put(conn2)

	if created.Load() != 1 {
		t.Errorf("created %d connections; want 1 (should reuse)", created.Load())
	}
	p.Close()
}

func TestGetDiscardsDeadConnections(t *testing.T) {
	var created atomic.Int32
	p := NewPool(PoolConfig{
		MaxSize: 5,
		Factory: func() (Conn, error) {
			created.Add(1)
			return &mockConn{alive: true}, nil
		},
	})

	conn, _ := p.Get(context.Background())
	conn.(*mockConn).alive = false // simulate dead connection
	p.Put(conn)

	conn2, _ := p.Get(context.Background())
	if conn2 == nil {
		t.Fatal("Get returned nil")
	}
	if created.Load() < 2 {
		t.Error("should have created a new connection since the old one was dead")
	}
	p.Put(conn2)
	p.Close()
}

func TestGetBlocksAtMaxSize(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 1, Factory: newMockFactory()})

	conn, _ := p.Get(context.Background())

	// Second Get should block since max is 1
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := p.Get(ctx)
	if err == nil {
		t.Error("Get should have failed with timeout (pool at max capacity)")
	}

	p.Put(conn)
	p.Close()
}

func TestGetRespectsContextCancellation(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 1, Factory: newMockFactory()})

	conn, _ := p.Get(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := p.Get(ctx)
	if err == nil {
		t.Error("Get should return error when context is cancelled")
	}

	p.Put(conn)
	p.Close()
}

// TestPutAlwaysReturnsConnection verifies Put handles the connection properly.
// Callers MUST call Put even when the connection errored.
func TestPutAfterConnectionError(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 5, Factory: newMockFactory()})

	conn, _ := p.Get(context.Background())
	conn.(*mockConn).alive = false

	// Should not panic
	p.Put(conn)

	stats := p.Stats()
	if stats.Idle != 0 {
		t.Errorf("dead connection should not be returned to idle pool; idle=%d", stats.Idle)
	}
	p.Close()
}

func TestCloseClosesAllConnections(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 5, Factory: newMockFactory()})

	conns := make([]Conn, 3)
	for i := range conns {
		conns[i], _ = p.Get(context.Background())
	}
	for _, c := range conns {
		p.Put(c)
	}

	p.Close()

	for i, c := range conns {
		if !c.(*mockConn).closed.Load() {
			t.Errorf("connection %d was not closed after pool Close", i)
		}
	}
}

func TestGetAfterClose(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 5, Factory: newMockFactory()})
	p.Close()

	_, err := p.Get(context.Background())
	if !errors.Is(err, ErrPoolClosed) {
		t.Errorf("Get after Close: %v; want ErrPoolClosed", err)
	}
}

func TestStats(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 5, Factory: newMockFactory()})

	s := p.Stats()
	if s.Total != 0 {
		t.Errorf("initial Total = %d; want 0", s.Total)
	}

	conn1, _ := p.Get(context.Background())
	conn2, _ := p.Get(context.Background())

	s = p.Stats()
	if s.InUse != 2 {
		t.Errorf("InUse = %d; want 2", s.InUse)
	}

	p.Put(conn1)
	s = p.Stats()
	if s.Idle != 1 || s.InUse != 1 {
		t.Errorf("Stats = %+v; want Idle=1, InUse=1", s)
	}

	p.Put(conn2)
	p.Close()
}

// TestConcurrentGetPut detects races with -race.
func TestConcurrentGetPut(t *testing.T) {
	p := NewPool(PoolConfig{MaxSize: 10, Factory: newMockFactory()})
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := p.Get(context.Background())
			if err != nil {
				return
			}
			time.Sleep(time.Millisecond)
			p.Put(conn)
		}()
	}

	wg.Wait()
	p.Close()
}
