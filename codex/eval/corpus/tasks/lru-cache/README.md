# Task: LRU Cache

Implement a least-recently-used (LRU) cache with a fixed capacity.

## Requirements

Implement the following in `cache.go`:

1. **`Cache` struct** created via:
   **`NewCache(capacity int) *Cache`** where capacity is the maximum number of entries.

2. **`(*Cache) Get(key string) (interface{}, bool)`** that:
   - Returns the value and true if the key exists
   - Returns nil, false if the key does not exist
   - Marks the accessed key as most-recently-used
   - Must be safe for concurrent use

3. **`(*Cache) Put(key string, value interface{})`** that:
   - Inserts or updates a key-value pair
   - Marks the key as most-recently-used
   - Evicts the least-recently-used entry if the cache is at capacity
   - Must be safe for concurrent use

4. **`(*Cache) Delete(key string) bool`** that:
   - Removes the key and returns true if it existed
   - Returns false if the key did not exist

5. **`(*Cache) Len() int`** returns the current number of entries.

6. **`(*Cache) Keys() []string`** returns all keys in order from most-recently-used to least-recently-used.

## Constraints

- Use only the Go standard library
- Must be safe for concurrent use from multiple goroutines
- Eviction must be O(1) — use a doubly-linked list + map
- Capacity of 0 means cache is disabled (all Gets miss, Puts are no-ops)

## Example Usage

```go
c := NewCache(3)
c.Put("a", 1)
c.Put("b", 2)
c.Put("c", 3)
c.Get("a")       // 1, true — "a" is now most recent
c.Put("d", 4)    // evicts "b" (least recently used)
c.Get("b")       // nil, false — evicted
```
