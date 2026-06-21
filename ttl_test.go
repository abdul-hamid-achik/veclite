package veclite

import (
	"testing"
	"time"
)

func TestRecordTTL(t *testing.T) {
	t.Run("no TTL by default", func(t *testing.T) {
		r := &Record{
			ID:        1,
			CreatedAt: time.Now(),
		}
		if r.HasTTL() {
			t.Error("expected HasTTL() to return false for record without TTL")
		}
		if r.IsExpired() {
			t.Error("expected IsExpired() to return false for record without TTL")
		}
		if r.TTL() != 0 {
			t.Errorf("expected TTL() to return 0, got %v", r.TTL())
		}
	})

	t.Run("TTL methods work correctly", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		r := &Record{
			ID:        1,
			ExpiresAt: future,
		}
		if !r.HasTTL() {
			t.Error("expected HasTTL() to return true")
		}
		if r.IsExpired() {
			t.Error("expected IsExpired() to return false for future expiration")
		}
		ttl := r.TTL()
		if ttl < 59*time.Minute || ttl > time.Hour {
			t.Errorf("expected TTL() to return ~1 hour, got %v", ttl)
		}
	})

	t.Run("expired record", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		r := &Record{
			ID:        1,
			ExpiresAt: past,
		}
		if !r.HasTTL() {
			t.Error("expected HasTTL() to return true")
		}
		if !r.IsExpired() {
			t.Error("expected IsExpired() to return true for past expiration")
		}
		if r.TTL() != 0 {
			t.Errorf("expected TTL() to return 0 for expired record, got %v", r.TTL())
		}
	})
}

func TestInsertWithOptions(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("test")

	t.Run("insert with TTL", func(t *testing.T) {
		vec := []float32{0.1, 0.2, 0.3}
		id, err := coll.InsertWithOptions(vec, nil, WithTTL(time.Hour))
		if err != nil {
			t.Fatalf("InsertWithOptions failed: %v", err)
		}

		record, err := coll.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if !record.HasTTL() {
			t.Error("expected record to have TTL")
		}
		if record.IsExpired() {
			t.Error("record should not be expired yet")
		}
	})

	t.Run("insert with explicit ExpiresAt", func(t *testing.T) {
		vec := []float32{0.1, 0.2, 0.3}
		expiresAt := time.Now().Add(2 * time.Hour)
		id, err := coll.InsertWithOptions(vec, nil, WithExpiresAt(expiresAt))
		if err != nil {
			t.Fatalf("InsertWithOptions failed: %v", err)
		}

		record, err := coll.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if !record.ExpiresAt.Equal(expiresAt) {
			t.Errorf("expected ExpiresAt to be %v, got %v", expiresAt, record.ExpiresAt)
		}
	})

	t.Run("insert with importance", func(t *testing.T) {
		vec := []float32{0.1, 0.2, 0.3}
		id, err := coll.InsertWithOptions(vec, nil, WithImportance(0.9))
		if err != nil {
			t.Fatalf("InsertWithOptions failed: %v", err)
		}

		record, err := coll.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if record.Importance != 0.9 {
			t.Errorf("expected Importance to be 0.9, got %v", record.Importance)
		}
	})

	t.Run("importance clamping", func(t *testing.T) {
		vec := []float32{0.1, 0.2, 0.3}

		// Test value > 1
		id1, _ := coll.InsertWithOptions(vec, nil, WithImportance(1.5))
		record1, _ := coll.Get(id1)
		if record1.Importance != 1.0 {
			t.Errorf("expected Importance to be clamped to 1.0, got %v", record1.Importance)
		}

		// Test value < 0
		id2, _ := coll.InsertWithOptions(vec, nil, WithImportance(-0.5))
		record2, _ := coll.Get(id2)
		if record2.Importance != 0.0 {
			t.Errorf("expected Importance to be clamped to 0.0, got %v", record2.Importance)
		}
	})

	t.Run("insert with content option", func(t *testing.T) {
		vec := []float32{0.1, 0.2, 0.3}
		id, err := coll.InsertWithOptions(vec, nil, WithContentOption("test content"))
		if err != nil {
			t.Fatalf("InsertWithOptions failed: %v", err)
		}

		record, err := coll.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if record.Content != "test content" {
			t.Errorf("expected Content to be 'test content', got %q", record.Content)
		}
	})

	t.Run("combined options", func(t *testing.T) {
		vec := []float32{0.1, 0.2, 0.3}
		id, err := coll.InsertWithOptions(vec, map[string]any{"key": "value"},
			WithTTL(time.Hour),
			WithImportance(0.8),
			WithContentOption("combined"),
		)
		if err != nil {
			t.Fatalf("InsertWithOptions failed: %v", err)
		}

		record, err := coll.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if !record.HasTTL() {
			t.Error("expected record to have TTL")
		}
		if record.Importance != 0.8 {
			t.Errorf("expected Importance 0.8, got %v", record.Importance)
		}
		if record.Content != "combined" {
			t.Errorf("expected Content 'combined', got %q", record.Content)
		}
		if record.Payload["key"] != "value" {
			t.Error("expected payload to be preserved")
		}
	})
}

func TestCleanupExpired(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("test")

	// Insert some records with very short TTL
	vec := []float32{0.1, 0.2, 0.3}

	// Record that expires immediately (in the past)
	_, err = coll.InsertWithOptions(vec, nil, WithExpiresAt(time.Now().Add(-time.Second)))
	if err != nil {
		t.Fatalf("InsertWithOptions failed: %v", err)
	}

	// Another expired record
	_, err = coll.InsertWithOptions(vec, nil, WithExpiresAt(time.Now().Add(-time.Minute)))
	if err != nil {
		t.Fatalf("InsertWithOptions failed: %v", err)
	}

	// Record that hasn't expired
	_, err = coll.InsertWithOptions(vec, nil, WithTTL(time.Hour))
	if err != nil {
		t.Fatalf("InsertWithOptions failed: %v", err)
	}

	// Record without TTL
	_, err = coll.Insert(vec, nil)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Should have 4 records
	if coll.Count() != 4 {
		t.Fatalf("expected 4 records, got %d", coll.Count())
	}

	// Count expired should be 2
	expiredCount := coll.CountExpired()
	if expiredCount != 2 {
		t.Errorf("expected 2 expired records, got %d", expiredCount)
	}

	// Cleanup
	deleted, err := coll.CleanupExpired()
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}

	if deleted != 2 {
		t.Errorf("expected 2 deleted records, got %d", deleted)
	}

	// Should have 2 records remaining
	if coll.Count() != 2 {
		t.Errorf("expected 2 remaining records, got %d", coll.Count())
	}
}

func TestTTLPersistence(t *testing.T) {
	// Create a temporary file for persistence test
	tmpFile := t.TempDir() + "/test.veclite"

	// First, create and insert with TTL
	func() {
		db, err := Open(tmpFile)
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		defer func() { _ = db.Close() }()

		coll := db.Collection("test")
		vec := []float32{0.1, 0.2, 0.3}

		_, err = coll.InsertWithOptions(vec, nil,
			WithTTL(time.Hour),
			WithImportance(0.75),
		)
		if err != nil {
			t.Fatalf("InsertWithOptions failed: %v", err)
		}
	}()

	// Reopen and verify TTL is preserved
	db, err := Open(tmpFile)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll, err := db.GetCollection("test")
	if err != nil {
		t.Fatalf("GetCollection failed: %v", err)
	}

	record, err := coll.Get(1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !record.HasTTL() {
		t.Error("expected record to have TTL after reload")
	}

	if record.Importance != 0.75 {
		t.Errorf("expected Importance 0.75 after reload, got %v", record.Importance)
	}
}
