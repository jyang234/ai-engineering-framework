// Package coalescer implements request deduplication/coalescing for concurrent callers.
package coalescer

// Result holds the return value, error, and sharing status of a coalesced call.
type Result struct {
	Value  interface{}
	Err    error
	Shared bool
}

// Group manages in-flight function calls, deduplicating concurrent calls for the same key.
type Group struct {
	// TODO: add fields (e.g., mutex, in-flight map)
}

// NewGroup creates a new coalescing group.
func NewGroup() *Group {
	// TODO: implement
	return &Group{}
}

// Do executes fn for the given key, deduplicating concurrent calls.
// If a call is already in-flight for key, it blocks until that call completes
// and returns the same result with Shared set to true.
func (g *Group) Do(key string, fn func() (interface{}, error)) Result {
	// TODO: implement
	val, err := fn()
	return Result{Value: val, Err: err, Shared: false}
}

// DoChan is like Do but returns a channel that receives the result.
// The call is non-blocking; the returned channel will receive exactly one Result.
func (g *Group) DoChan(key string, fn func() (interface{}, error)) <-chan Result {
	// TODO: implement
	ch := make(chan Result, 1)
	go func() {
		val, err := fn()
		ch <- Result{Value: val, Err: err, Shared: false}
	}()
	return ch
}

// Forget removes the in-flight entry for the given key, allowing the next call
// to execute fn fresh. It does not cancel or interrupt any in-progress execution.
func (g *Group) Forget(key string) {
	// TODO: implement
}
