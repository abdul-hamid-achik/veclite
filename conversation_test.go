package veclite

import (
	"testing"
	"time"
)

func TestInsertTurn(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("conversations")

	t.Run("basic turn insertion", func(t *testing.T) {
		id, err := coll.InsertTurn(ConversationTurn{
			SessionID:  "session-1",
			TurnNumber: 1,
			Role:       "user",
			Content:    "Hello",
			Vector:     []float32{1, 0, 0, 0},
		})
		if err != nil {
			t.Fatalf("InsertTurn failed: %v", err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}

		record, err := coll.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if record.Payload[PayloadKeySessionID] != "session-1" {
			t.Errorf("expected session-1, got %v", record.Payload[PayloadKeySessionID])
		}
		if record.Payload[PayloadKeyRole] != "user" {
			t.Errorf("expected user, got %v", record.Payload[PayloadKeyRole])
		}
	})

	t.Run("turn with importance and TTL", func(t *testing.T) {
		id, err := coll.InsertTurn(ConversationTurn{
			SessionID:  "session-1",
			TurnNumber: 2,
			Role:       "assistant",
			Vector:     []float32{0, 1, 0, 0},
			Importance: 0.8,
			TTL:        time.Hour,
		})
		if err != nil {
			t.Fatalf("InsertTurn failed: %v", err)
		}

		record, err := coll.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if record.Importance != 0.8 {
			t.Errorf("expected importance 0.8, got %v", record.Importance)
		}
		if record.ExpiresAt.IsZero() {
			t.Error("expected ExpiresAt to be set")
		}
	})

	t.Run("requires session ID", func(t *testing.T) {
		_, err := coll.InsertTurn(ConversationTurn{
			Vector: []float32{0, 0, 1, 0},
		})
		if err == nil {
			t.Error("expected error for missing session ID")
		}
	})
}

func TestGetSession(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("conversations")

	// Insert turns for two sessions
	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 1, Vector: []float32{1, 0, 0, 0}})
	time.Sleep(time.Millisecond)
	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 2, Vector: []float32{0, 1, 0, 0}})
	time.Sleep(time.Millisecond)
	coll.InsertTurn(ConversationTurn{SessionID: "session-2", TurnNumber: 1, Vector: []float32{0, 0, 1, 0}})
	time.Sleep(time.Millisecond)
	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 3, Vector: []float32{0, 0, 0, 1}})

	records, err := coll.GetSession("session-1")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}

	// Check sorted by turn number
	for i := 0; i < len(records); i++ {
		expectedTurn := i + 1
		actualTurn := getTurnNumber(records[i].Payload)
		if actualTurn != expectedTurn {
			t.Errorf("expected turn %d, got %d", expectedTurn, actualTurn)
		}
	}
}

func TestGetThread(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("conversations")

	// Create a thread: root -> reply1 -> reply2
	rootID, _ := coll.InsertTurn(ConversationTurn{
		SessionID:  "session-1",
		TurnNumber: 1,
		Vector:     []float32{1, 0, 0, 0},
	})
	time.Sleep(time.Millisecond)

	reply1ID, _ := coll.InsertTurn(ConversationTurn{
		SessionID:     "session-1",
		TurnNumber:    2,
		Vector:        []float32{0, 1, 0, 0},
		ParentChunkID: rootID,
	})
	time.Sleep(time.Millisecond)

	_, _ = coll.InsertTurn(ConversationTurn{
		SessionID:     "session-1",
		TurnNumber:    3,
		Vector:        []float32{0, 0, 1, 0},
		ParentChunkID: reply1ID,
	})

	// Get thread starting from root
	thread, err := coll.GetThread(rootID)
	if err != nil {
		t.Fatalf("GetThread failed: %v", err)
	}

	if len(thread) != 3 {
		t.Errorf("expected 3 records in thread, got %d", len(thread))
	}

	// Check sorted by creation time
	for i := 1; i < len(thread); i++ {
		if thread[i].CreatedAt.Before(thread[i-1].CreatedAt) {
			t.Error("thread not sorted by creation time")
		}
	}
}

func TestSearchInSession(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("conversations")

	// Insert turns in different sessions
	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 1, Vector: []float32{1, 0, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 2, Vector: []float32{0.9, 0.1, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "session-2", TurnNumber: 1, Vector: []float32{0.95, 0.05, 0, 0}})

	// Search in session-1 only
	results, err := coll.SearchInSession("session-1", []float32{1, 0, 0, 0}, WithLimit(10))
	if err != nil {
		t.Fatalf("SearchInSession failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results from session-1, got %d", len(results))
	}

	for _, r := range results {
		if r.Record.Payload[PayloadKeySessionID] != "session-1" {
			t.Error("found result from wrong session")
		}
	}
}

func TestListSessions(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("conversations")

	coll.InsertTurn(ConversationTurn{SessionID: "alpha", TurnNumber: 1, Vector: []float32{1, 0, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "beta", TurnNumber: 1, Vector: []float32{0, 1, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "alpha", TurnNumber: 2, Vector: []float32{0, 0, 1, 0}})

	sessions := coll.ListSessions()

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// Should be sorted
	if sessions[0] != "alpha" || sessions[1] != "beta" {
		t.Errorf("unexpected session order: %v", sessions)
	}
}

func TestGetSessionStats(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("conversations")

	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 1, Role: "user", Vector: []float32{1, 0, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 2, Role: "assistant", Vector: []float32{0, 1, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "session-1", TurnNumber: 3, Role: "user", Vector: []float32{0, 0, 1, 0}})

	stats, err := coll.GetSessionStats("session-1")
	if err != nil {
		t.Fatalf("GetSessionStats failed: %v", err)
	}

	if stats.TurnCount != 3 {
		t.Errorf("expected 3 turns, got %d", stats.TurnCount)
	}
	if stats.Roles["user"] != 2 {
		t.Errorf("expected 2 user turns, got %d", stats.Roles["user"])
	}
	if stats.Roles["assistant"] != 1 {
		t.Errorf("expected 1 assistant turn, got %d", stats.Roles["assistant"])
	}
}
