package cache

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGetMiss(t *testing.T) {
	c := NewCache(10)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("Get on empty cache should return false")
	}
}

func TestPutAndGet(t *testing.T) {
	c := NewCache(10)
	c.Put("a", 1)
	val, ok := c.Get("a")
	if !ok || val != 1 {
		t.Errorf("Get(a) = %v, %v; want 1, true", val, ok)
	}
}

func TestEvictsLRU(t *testing.T) {
	c := NewCache(3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Put("d", 4) // should evict "a"

	if _, ok := c.Get("a"); ok {
		t.Error("'a' should have been evicted")
	}
	if v, ok := c.Get("d"); !ok || v != 4 {
		t.Error("'d' should exist")
	}
}

func TestGetUpdatesRecency(t *testing.T) {
	c := NewCache(3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Get("a") // "a" is now most recent
	c.Put("d", 4) // should evict "b" (not "a")

	if _, ok := c.Get("a"); !ok {
		t.Error("'a' should still exist (was accessed recently)")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("'b' should have been evicted (least recently used)")
	}
}

func TestPutUpdatesExisting(t *testing.T) {
	c := NewCache(10)
	c.Put("a", 1)
	c.Put("a", 2)
	val, ok := c.Get("a")
	if !ok || val != 2 {
		t.Errorf("Get(a) after update = %v, %v; want 2, true", val, ok)
	}
	if c.Len() != 1 {
		t.Errorf("Len() = %d; want 1 (update should not increase size)", c.Len())
	}
}

func TestDelete(t *testing.T) {
	c := NewCache(10)
	c.Put("a", 1)
	if !c.Delete("a") {
		t.Error("Delete existing key should return true")
	}
	if c.Delete("a") {
		t.Error("Delete missing key should return false")
	}
	if c.Len() != 0 {
		t.Errorf("Len() = %d after delete; want 0", c.Len())
	}
}

func TestLen(t *testing.T) {
	c := NewCache(10)
	if c.Len() != 0 {
		t.Errorf("empty cache Len() = %d; want 0", c.Len())
	}
	c.Put("a", 1)
	c.Put("b", 2)
	if c.Len() != 2 {
		t.Errorf("Len() = %d; want 2", c.Len())
	}
}

func TestKeysOrder(t *testing.T) {
	c := NewCache(5)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Get("a") // "a" is now most recent

	keys := c.Keys()
	if len(keys) != 3 {
		t.Fatalf("Keys() len = %d; want 3", len(keys))
	}
	if keys[0] != "a" {
		t.Errorf("Keys()[0] = %s; want 'a' (most recent)", keys[0])
	}
	if keys[len(keys)-1] != "b" {
		t.Errorf("Keys()[last] = %s; want 'b' (least recent)", keys[len(keys)-1])
	}
}

func TestZeroCapacityDisabled(t *testing.T) {
	c := NewCache(0)
	c.Put("a", 1)
	if _, ok := c.Get("a"); ok {
		t.Error("zero capacity cache should always miss")
	}
	if c.Len() != 0 {
		t.Errorf("zero capacity Len() = %d; want 0", c.Len())
	}
}

// TestConcurrentAccess detects data races with -race flag.
func TestConcurrentAccess(t *testing.T) {
	c := NewCache(100)
	var wg sync.WaitGroup
	var hits atomic.Int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%26))
			c.Put(key, n)
			if _, ok := c.Get(key); ok {
				hits.Add(1)
			}
			c.Keys()
			c.Len()
		}(i)
	}

	wg.Wait()
	t.Logf("concurrent hits: %d/50", hits.Load())
}

func TestEvictionUnderConcurrency(t *testing.T) {
	c := NewCache(10)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('A' + n%26))
			c.Put(key, n)
		}(i)
	}

	wg.Wait()

	if c.Len() > 10 {
		t.Errorf("Len() = %d exceeds capacity 10 — eviction failed under concurrency", c.Len())
	}
}
