// Package cache implements a least-recently-used cache with fixed capacity.
package cache

// Cache is a concurrent-safe LRU cache.
type Cache struct {
	capacity int
}

// NewCache creates an LRU cache with the given capacity.
func NewCache(capacity int) *Cache {
	// TODO: implement
	return &Cache{capacity: capacity}
}

// Get retrieves a value by key, marking it as most-recently-used.
func (c *Cache) Get(key string) (interface{}, bool) {
	// TODO: implement
	return nil, false
}

// Put inserts or updates a key-value pair, evicting the LRU entry if at capacity.
func (c *Cache) Put(key string, value interface{}) {
	// TODO: implement
}

// Delete removes a key from the cache.
func (c *Cache) Delete(key string) bool {
	// TODO: implement
	return false
}

// Len returns the number of entries in the cache.
func (c *Cache) Len() int {
	// TODO: implement
	return 0
}

// Keys returns all keys from most-recently-used to least-recently-used.
func (c *Cache) Keys() []string {
	// TODO: implement
	return nil
}
