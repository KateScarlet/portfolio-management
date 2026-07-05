package scheduler

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	user1 = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	user2 = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

func TestEventBus_Subscribe(t *testing.T) {
	eb := NewEventBus()
	ch, unsub := eb.Subscribe(user1)
	defer unsub()

	if ch == nil {
		t.Error("expected channel to be non-nil")
	}
}

func TestEventBus_Publish(t *testing.T) {
	eb := NewEventBus()
	ch, unsub := eb.Subscribe(user1)
	defer unsub()

	event := Event{
		Type:        EventSyncStarted,
		PortfolioID: "portfolio1",
		Data:        SyncStartedData{},
		Timestamp:   time.Now(),
	}

	eb.Publish(user1, event)

	select {
	case received := <-ch:
		if received.Type != EventSyncStarted {
			t.Errorf("expected event type %s, got %s", EventSyncStarted, received.Type)
		}
		if received.PortfolioID != "portfolio1" {
			t.Errorf("expected portfolio ID portfolio1, got %s", received.PortfolioID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive event, but timed out")
	}
}

func TestEventBus_PublishToMultipleSubscribers(t *testing.T) {
	eb := NewEventBus()
	ch1, unsub1 := eb.Subscribe(user1)
	ch2, unsub2 := eb.Subscribe(user1)
	defer unsub1()
	defer unsub2()

	event := Event{
		Type:        EventSyncCompleted,
		PortfolioID: "portfolio1",
		Data:        SyncCompletedData{},
		Timestamp:   time.Now(),
	}

	eb.Publish(user1, event)

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Type != EventSyncCompleted {
				t.Errorf("subscriber %d: expected event type %s, got %s", i, EventSyncCompleted, received.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d: expected to receive event, but timed out", i)
		}
	}
}

func TestEventBus_PublishOnlyToUser(t *testing.T) {
	eb := NewEventBus()
	ch1, unsub1 := eb.Subscribe(user1)
	ch2, unsub2 := eb.Subscribe(user2)
	defer unsub1()
	defer unsub2()

	event := Event{
		Type:        EventSyncStarted,
		PortfolioID: "portfolio1",
		Data:        SyncStartedData{},
		Timestamp:   time.Now(),
	}

	eb.Publish(user1, event)

	select {
	case <-ch1:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("user1: expected to receive event")
	}

	select {
	case <-ch2:
		t.Error("user2: should not receive event for user1")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	eb := NewEventBus()
	ch, unsub := eb.Subscribe(user1)

	unsub()

	event := Event{
		Type:        EventSyncStarted,
		PortfolioID: "portfolio1",
		Data:        SyncStartedData{},
		Timestamp:   time.Now(),
	}

	eb.Publish(user1, event)

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected closed channel, but timed out")
	}
}

func TestEventBus_BufferFull_DropsEvent(t *testing.T) {
	eb := NewEventBus()
	ch, unsub := eb.Subscribe(user1)
	defer unsub()

	event := Event{
		Type:        EventSyncStarted,
		PortfolioID: "portfolio1",
		Data:        SyncStartedData{},
		Timestamp:   time.Now(),
	}

	// Fill the buffer
	for range subscriberBufferSize {
		eb.Publish(user1, event)
	}

	// This should be dropped
	eb.Publish(user1, event)

	// Drain the buffer
	for range subscriberBufferSize {
		<-ch
	}

	// Channel should be empty now
	select {
	case <-ch:
		t.Error("expected no more events in channel")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestEventBus_ConcurrentPublish(t *testing.T) {
	eb := NewEventBus()
	ch, unsub := eb.Subscribe(user1)
	defer unsub()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			eb.Publish(user1, Event{
				Type:        EventSyncStarted,
				PortfolioID: "portfolio1",
				Data:        SyncStartedData{},
				Timestamp:   time.Now(),
			})
		}()
	}

	wg.Wait()

	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			// Some events may be dropped due to buffer size
			if received == 0 {
				t.Error("expected at least some events")
			}
			return
		}
	}
}

func TestEventBus_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	eb := NewEventBus()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			ch, unsub := eb.Subscribe(user1)
			// Immediately unsubscribe
			unsub()
			// Channel should be closed
			_, ok := <-ch
			if ok {
				t.Error("expected closed channel after unsubscribe")
			}
		}()
	}

	wg.Wait()
}

func TestEventBus_SubscribeUserIsolation(t *testing.T) {
	eb := NewEventBus()
	ch1, unsub1 := eb.Subscribe(user1)
	ch2, unsub2 := eb.Subscribe(user2)
	defer unsub1()
	defer unsub2()

	// Publish to user1
	eb.Publish(user1, Event{
		Type:        EventSyncStarted,
		PortfolioID: "p1",
		Data:        SyncStartedData{},
		Timestamp:   time.Now(),
	})

	// Publish to user2
	eb.Publish(user2, Event{
		Type:        EventSyncCompleted,
		PortfolioID: "p2",
		Data:        SyncCompletedData{},
		Timestamp:   time.Now(),
	})

	// user1 should get sync.started
	select {
	case e := <-ch1:
		if e.Type != EventSyncStarted {
			t.Errorf("expected sync.started, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("user1: expected event")
	}

	// user2 should get sync.completed
	select {
	case e := <-ch2:
		if e.Type != EventSyncCompleted {
			t.Errorf("expected sync.completed, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("user2: expected event")
	}
}
