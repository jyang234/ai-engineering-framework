package counter

import (
	"sort"
	"sync"
	"testing"
)

func TestNewCounter(t *testing.T) {
	c := NewCounter(42)
	if c.Value() != 42 {
		t.Errorf("Value() = %d; want 42", c.Value())
	}
}

func TestIncDec(t *testing.T) {
	c := NewCounter(0)
	if v := c.Inc(); v != 1 {
		t.Errorf("Inc() = %d; want 1", v)
	}
	if v := c.Inc(); v != 2 {
		t.Errorf("Inc() = %d; want 2", v)
	}
	if v := c.Dec(); v != 1 {
		t.Errorf("Dec() = %d; want 1", v)
	}
}

func TestAdd(t *testing.T) {
	c := NewCounter(10)
	if v := c.Add(5); v != 15 {
		t.Errorf("Add(5) = %d; want 15", v)
	}
	if v := c.Add(-20); v != -5 {
		t.Errorf("Add(-20) = %d; want -5", v)
	}
}

func TestReset(t *testing.T) {
	c := NewCounter(42)
	old := c.Reset()
	if old != 42 {
		t.Errorf("Reset() returned %d; want 42", old)
	}
	if c.Value() != 0 {
		t.Errorf("Value() after reset = %d; want 0", c.Value())
	}
}

func TestCompareAndSwap(t *testing.T) {
	c := NewCounter(10)
	if !c.CompareAndSwap(10, 20) {
		t.Error("CAS(10, 20) should succeed")
	}
	if c.Value() != 20 {
		t.Errorf("Value() = %d; want 20", c.Value())
	}
	if c.CompareAndSwap(10, 30) {
		t.Error("CAS(10, 30) should fail (current is 20)")
	}
}

// TestConcurrentInc verifies atomicity with -race.
func TestConcurrentInc(t *testing.T) {
	c := NewCounter(0)
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}

	wg.Wait()
	if c.Value() != 1000 {
		t.Errorf("after 1000 concurrent Inc: Value() = %d; want 1000", c.Value())
	}
}

// TestConcurrentAdd verifies no lost updates under contention.
func TestConcurrentAdd(t *testing.T) {
	c := NewCounter(0)
	var wg sync.WaitGroup

	for i := 0; i < 500; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); c.Add(2) }()
		go func() { defer wg.Done(); c.Add(-1) }()
	}

	wg.Wait()
	// 500 * 2 + 500 * (-1) = 500
	if c.Value() != 500 {
		t.Errorf("after concurrent Add: Value() = %d; want 500", c.Value())
	}
}

func TestCounterGroupGet(t *testing.T) {
	g := NewCounterGroup()
	c := g.Get("requests")
	if c == nil {
		t.Fatal("Get should never return nil")
	}
	c.Inc()
	if g.Get("requests").Value() != 1 {
		t.Error("same name should return same counter")
	}
}

func TestCounterGroupGetCreatesNew(t *testing.T) {
	g := NewCounterGroup()
	c := g.Get("new_counter")
	if c == nil {
		t.Fatal("Get should create counter if missing")
	}
	if c.Value() != 0 {
		t.Errorf("new counter Value() = %d; want 0", c.Value())
	}
}

func TestCounterGroupSnapshot(t *testing.T) {
	g := NewCounterGroup()
	g.Get("a").Add(10)
	g.Get("b").Add(20)

	snap := g.Snapshot()
	if snap["a"] != 10 || snap["b"] != 20 {
		t.Errorf("Snapshot() = %v; want {a:10, b:20}", snap)
	}

	// Modifying snapshot should not affect group
	snap["a"] = 999
	if g.Get("a").Value() != 10 {
		t.Error("Snapshot returned a reference, not a copy")
	}
}

func TestCounterGroupNames(t *testing.T) {
	g := NewCounterGroup()
	g.Get("charlie")
	g.Get("alpha")
	g.Get("bravo")

	names := g.Names()
	if !sort.StringsAreSorted(names) {
		t.Errorf("Names() = %v; should be sorted", names)
	}
	if len(names) != 3 {
		t.Errorf("Names() len = %d; want 3", len(names))
	}
}

// TestCounterGroupConcurrent runs with -race to detect data races.
func TestCounterGroupConcurrent(t *testing.T) {
	g := NewCounterGroup()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := string(rune('a' + n%5))
			g.Get(name).Inc()
			g.Snapshot()
			g.Names()
		}(i)
	}

	wg.Wait()

	snap := g.Snapshot()
	var total int64
	for _, v := range snap {
		total += v
	}
	if total != 100 {
		t.Errorf("total across all counters = %d; want 100", total)
	}
}
