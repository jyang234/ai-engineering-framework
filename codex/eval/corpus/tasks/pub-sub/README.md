# Task: In-Process Publish/Subscribe Event Bus

Implement a typed, in-process publish/subscribe event bus for decoupling components.

## Requirements

Implement the following in `bus.go`:

1. **`Bus` struct** created via **`NewBus() *Bus`**

2. **`(*Bus) Subscribe(topic string, handler func(interface{})) (unsubscribe func())`** that:
   - Registers a handler for the given topic
   - Returns an unsubscribe function that removes the handler
   - Multiple handlers can subscribe to the same topic
   - Must be safe for concurrent use

3. **`(*Bus) Publish(topic string, msg interface{})`** that:
   - Delivers msg to all current subscribers of the topic
   - Delivery is synchronous (Publish returns after all handlers have been called)
   - Must not block if a subscriber panics (recover and continue to next handler)
   - Must be safe for concurrent use

4. **`(*Bus) PublishAsync(topic string, msg interface{})`** that:
   - Delivers msg to all current subscribers in separate goroutines
   - Returns immediately (fire-and-forget)
   - Each handler gets its own goroutine
   - Must recover from panics in handlers

5. **`(*Bus) SubscriberCount(topic string) int`** returns the number of active subscribers for a topic.

6. **`(*Bus) Topics() []string`** returns all topics that have at least one subscriber, sorted.

7. **`(*Bus) Close()`** that:
   - Removes all subscribers
   - Subsequent Publish calls become no-ops
   - Subsequent Subscribe calls return a no-op unsubscribe function
   - Is safe to call multiple times

## Constraints

- Use only the Go standard library
- Must be safe for concurrent Subscribe, Publish, and Unsubscribe from multiple goroutines
- Unsubscribe must not cause panics if called during Publish
- Handlers must not block the bus — panics in handlers must be recovered
- Close must not leak goroutines from pending PublishAsync calls
