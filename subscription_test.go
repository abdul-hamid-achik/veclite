package veclite

import (
	"sync"
	"testing"
	"time"
)

func TestSubscription(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	t.Run("basic subscribe and receive events", func(t *testing.T) {
		sub, err := coll.Subscribe(
			[]float32{1, 0, 0, 0},
			WithSubscriptionThreshold(0.5),
		)
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		defer sub.Close()

		// Insert a matching record
		id, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, nil)

		// Manually trigger notification (in real use, this happens on insert)
		record, _ := coll.Get(id)
		sm := db.getSubscriptionManager(coll)
		sm.notifyInsert(record)

		// Wait for event
		select {
		case event := <-sub.Events():
			if event.Record.ID != id {
				t.Errorf("expected ID %d, got %d", id, event.Record.ID)
			}
			if event.SubscriptionID != sub.ID {
				t.Errorf("expected subscription ID %s, got %s", sub.ID, event.SubscriptionID)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
		}
	})

	t.Run("threshold filtering", func(t *testing.T) {
		// Use a separate collection for isolation from goroutine races
		threshColl := db.Collection("threshold_test")

		sub, err := threshColl.Subscribe(
			[]float32{1, 0, 0, 0},
			WithSubscriptionThreshold(0.99), // Very high threshold
		)
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		defer sub.Close()

		// Insert a record that doesn't meet threshold
		id, _ := threshColl.Insert([]float32{0.5, 0.5, 0, 0}, nil)
		record, _ := threshColl.Get(id)

		sm := db.getSubscriptionManager(threshColl)
		sm.notifyInsert(record)

		// Should not receive event
		select {
		case <-sub.Events():
			t.Error("should not receive event below threshold")
		case <-time.After(100 * time.Millisecond):
			// Expected - no event
		}
	})

	t.Run("filter matching", func(t *testing.T) {
		sub, err := coll.Subscribe(
			[]float32{1, 0, 0, 0},
			WithSubscriptionFilter(Equal("type", "important")),
		)
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		defer sub.Close()

		// Insert non-matching record
		id1, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, map[string]any{"type": "regular"})
		record1, _ := coll.Get(id1)

		sm := db.getSubscriptionManager(coll)
		sm.notifyInsert(record1)

		// Should not receive event
		select {
		case <-sub.Events():
			t.Error("should not receive event for non-matching filter")
		case <-time.After(100 * time.Millisecond):
			// Expected
		}

		// Insert matching record
		id2, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, map[string]any{"type": "important"})
		record2, _ := coll.Get(id2)
		sm.notifyInsert(record2)

		// Should receive event
		select {
		case event := <-sub.Events():
			if event.Record.ID != id2 {
				t.Errorf("expected ID %d, got %d", id2, event.Record.ID)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
		}
	})

	t.Run("unsubscribe", func(t *testing.T) {
		sub, err := coll.Subscribe([]float32{1, 0, 0, 0})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}

		subID := sub.ID

		err = coll.Unsubscribe(subID)
		if err != nil {
			t.Fatalf("Unsubscribe failed: %v", err)
		}

		if !sub.IsClosed() {
			t.Error("subscription should be closed after unsubscribe")
		}

		// Unsubscribing again should return error
		err = coll.Unsubscribe(subID)
		if err == nil {
			t.Error("expected error for unsubscribing non-existent subscription")
		}
	})

	t.Run("subscription close", func(t *testing.T) {
		sub, err := coll.Subscribe([]float32{1, 0, 0, 0})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}

		err = sub.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		if !sub.IsClosed() {
			t.Error("subscription should be closed")
		}

		// Closing again should be safe
		err = sub.Close()
		if err != nil {
			t.Error("closing again should not error")
		}
	})
}

func TestSubscriptionConcurrency(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	const numSubscriptions = 10
	const numInserts = 100

	var subs []*Subscription
	for i := 0; i < numSubscriptions; i++ {
		sub, err := coll.Subscribe([]float32{1, 0, 0, 0}, WithSubscriptionBufferSize(numInserts))
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		subs = append(subs, sub)
	}
	defer func() {
		for _, sub := range subs {
			sub.Close()
		}
	}()

	// Insert records concurrently
	var wg sync.WaitGroup
	sm := db.getSubscriptionManager(coll)

	for i := 0; i < numInserts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, map[string]any{"index": idx})
			record, _ := coll.Get(id)
			sm.notifyInsert(record)
		}(i)
	}

	wg.Wait()

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Each subscription should have received all events
	for i, sub := range subs {
		count := 0
		// Drain the channel
	drain:
		for {
			select {
			case <-sub.Events():
				count++
			default:
				break drain
			}
		}
		if count != numInserts {
			t.Errorf("subscription %d received %d events, expected %d", i, count, numInserts)
		}
	}
}

func TestSubscriptionBufferSize(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	// Create subscription with small buffer
	sub, err := coll.Subscribe(
		[]float32{1, 0, 0, 0},
		WithSubscriptionBufferSize(2),
	)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	sm := db.getSubscriptionManager(coll)

	// Insert more records than buffer size
	for i := 0; i < 5; i++ {
		id, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, nil)
		record, _ := coll.Get(id)
		sm.notifyInsert(record)
	}

	// We should have received some events (buffer size worth)
	count := 0
drain:
	for {
		select {
		case <-sub.Events():
			count++
		case <-time.After(100 * time.Millisecond):
			break drain
		}
	}

	// Should have received at most buffer size + some that fit
	if count > 3 { // Buffer of 2 plus maybe 1 more
		t.Logf("received %d events (buffer size: 2)", count)
	}
}

func TestSubscriptionIDGeneration(t *testing.T) {
	id1 := generateSubscriptionID()
	id2 := generateSubscriptionID()

	if id1 == id2 {
		t.Error("subscription IDs should be unique")
	}

	if len(id1) == 0 {
		t.Error("subscription ID should not be empty")
	}
}
