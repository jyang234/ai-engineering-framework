// Package bus provides an in-process publish/subscribe event bus.
package bus

// Bus is a concurrent-safe publish/subscribe event bus.
type Bus struct{}

// NewBus creates a new event bus.
func NewBus() *Bus {
	// TODO: implement
	return &Bus{}
}

// Subscribe registers a handler for a topic.
// Returns a function to unsubscribe the handler.
func (b *Bus) Subscribe(topic string, handler func(interface{})) func() {
	// TODO: implement
	return func() {}
}

// Publish sends a message to all subscribers of a topic synchronously.
func (b *Bus) Publish(topic string, msg interface{}) {
	// TODO: implement
}

// PublishAsync sends a message to all subscribers asynchronously.
func (b *Bus) PublishAsync(topic string, msg interface{}) {
	// TODO: implement
}

// SubscriberCount returns the number of subscribers for a topic.
func (b *Bus) SubscriberCount(topic string) int {
	// TODO: implement
	return 0
}

// Topics returns all topics with subscribers, sorted.
func (b *Bus) Topics() []string {
	// TODO: implement
	return nil
}

// Close removes all subscribers and makes the bus inert.
func (b *Bus) Close() {
	// TODO: implement
}
