package bus

import (
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublishToSubscriber(t *testing.T) {
	b := NewBus()
	var received interface{}
	b.Subscribe("test", func(msg interface{}) {
		received = msg
	})
	b.Publish("test", "hello")
	if received != "hello" {
		t.Errorf("got %v, want hello", received)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := NewBus()
	var count atomic.Int32
	b.Subscribe("test", func(msg interface{}) { count.Add(1) })
	b.Subscribe("test", func(msg interface{}) { count.Add(1) })
	b.Subscribe("test", func(msg interface{}) { count.Add(1) })
	b.Publish("test", nil)
	if count.Load() != 3 {
		t.Errorf("count = %d; want 3", count.Load())
	}
}

func TestUnsubscribe(t *testing.T) {
	b := NewBus()
	var count atomic.Int32
	unsub := b.Subscribe("test", func(msg interface{}) { count.Add(1) })
	b.Publish("test", nil)
	unsub()
	b.Publish("test", nil)
	if count.Load() != 1 {
		t.Errorf("count = %d; want 1 (should not receive after unsubscribe)", count.Load())
	}
}

func TestPublishToWrongTopic(t *testing.T) {
	b := NewBus()
	called := false
	b.Subscribe("topic-a", func(msg interface{}) { called = true })
	b.Publish("topic-b", nil)
	if called {
		t.Error("subscriber should not be called for different topic")
	}
}

func TestPublishRecoversPanic(t *testing.T) {
	b := NewBus()
	var secondCalled atomic.Bool

	b.Subscribe("test", func(msg interface{}) {
		panic("handler panic!")
	})
	b.Subscribe("test", func(msg interface{}) {
		secondCalled.Store(true)
	})

	// Should not panic
	b.Publish("test", nil)

	if !secondCalled.Load() {
		t.Error("second handler should still be called after first panics")
	}
}

func TestPublishAsync(t *testing.T) {
	b := NewBus()
	var count atomic.Int32
	done := make(chan struct{})

	b.Subscribe("test", func(msg interface{}) {
		count.Add(1)
		if count.Load() == 3 {
			close(done)
		}
	})
	b.Subscribe("test", func(msg interface{}) {
		count.Add(1)
		if count.Load() == 3 {
			close(done)
		}
	})
	b.Subscribe("test", func(msg interface{}) {
		count.Add(1)
		if count.Load() == 3 {
			close(done)
		}
	})

	b.PublishAsync("test", nil)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout: only %d/3 async handlers completed", count.Load())
	}
}

func TestSubscriberCount(t *testing.T) {
	b := NewBus()
	if b.SubscriberCount("test") != 0 {
		t.Error("empty topic should have 0 subscribers")
	}
	unsub1 := b.Subscribe("test", func(interface{}) {})
	unsub2 := b.Subscribe("test", func(interface{}) {})
	if b.SubscriberCount("test") != 2 {
		t.Errorf("count = %d; want 2", b.SubscriberCount("test"))
	}
	unsub1()
	if b.SubscriberCount("test") != 1 {
		t.Errorf("count after unsub = %d; want 1", b.SubscriberCount("test"))
	}
	unsub2()
	if b.SubscriberCount("test") != 0 {
		t.Errorf("count after all unsub = %d; want 0", b.SubscriberCount("test"))
	}
}

func TestTopics(t *testing.T) {
	b := NewBus()
	b.Subscribe("beta", func(interface{}) {})
	b.Subscribe("alpha", func(interface{}) {})
	b.Subscribe("gamma", func(interface{}) {})

	topics := b.Topics()
	if !sort.StringsAreSorted(topics) {
		t.Errorf("Topics() not sorted: %v", topics)
	}
	if len(topics) != 3 {
		t.Errorf("Topics() len = %d; want 3", len(topics))
	}
}

func TestClose(t *testing.T) {
	b := NewBus()
	var count atomic.Int32
	b.Subscribe("test", func(interface{}) { count.Add(1) })
	b.Close()
	b.Publish("test", nil) // should be no-op
	if count.Load() != 0 {
		t.Error("Publish after Close should be no-op")
	}
}

func TestCloseIdempotent(t *testing.T) {
	b := NewBus()
	b.Close()
	b.Close() // should not panic
}

func TestSubscribeAfterClose(t *testing.T) {
	b := NewBus()
	b.Close()
	unsub := b.Subscribe("test", func(interface{}) {})
	unsub() // should not panic
}

// TestConcurrentOperations detects races with -race.
func TestConcurrentOperations(t *testing.T) {
	b := NewBus()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			topic := string(rune('a' + n%3))
			unsub := b.Subscribe(topic, func(interface{}) {})
			b.Publish(topic, n)
			b.PublishAsync(topic, n)
			b.SubscriberCount(topic)
			b.Topics()
			if n%2 == 0 {
				unsub()
			}
		}(i)
	}

	wg.Wait()
	b.Close()
}

// TestUnsubscribeDuringPublish ensures no panic when unsubscribing while publishing.
func TestUnsubscribeDuringPublish(t *testing.T) {
	b := NewBus()
	var unsub func()

	unsub = b.Subscribe("test", func(interface{}) {
		unsub() // unsubscribe self during publish
	})

	// Should not panic or deadlock
	b.Publish("test", nil)
}
