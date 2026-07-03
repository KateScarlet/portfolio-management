package scheduler

import (
	"log/slog"
	"sync"
)

const subscriberBufferSize = 64

type EventBus struct {
	mu   sync.RWMutex
	subs map[string][]chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{
		subs: make(map[string][]chan Event),
	}
}

func (eb *EventBus) Subscribe(userID string) (events <-chan Event, unsubscribe func()) {
	ch := make(chan Event, subscriberBufferSize)

	eb.mu.Lock()
	eb.subs[userID] = append(eb.subs[userID], ch)
	eb.mu.Unlock()

	unsubscribe = func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		subs := eb.subs[userID]
		for i, sub := range subs {
			if sub == ch {
				eb.subs[userID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
		if len(eb.subs[userID]) == 0 {
			delete(eb.subs, userID)
		}
	}

	return ch, unsubscribe
}

func (eb *EventBus) Publish(userID string, event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, ch := range eb.subs[userID] {
		select {
		case ch <- event:
		default:
			slog.Warn("sse subscriber buffer full, dropping event", "userId", userID, "eventType", event.Type)
		}
	}
}
