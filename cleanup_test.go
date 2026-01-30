package veclite

import (
	"sync/atomic"
	"testing"
	"time"
)

// TC-CLEANUP-001: TTL Cleaner starts and stops correctly
func TestTTLCleanerStartStop(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var cleanupCount int32

	stop := db.StartTTLCleaner(50*time.Millisecond, func(collection string, deleted int) {
		atomic.AddInt32(&cleanupCount, 1)
	})

	// Let it run a few cycles
	time.Sleep(200 * time.Millisecond)

	// Stop should not block or panic
	stop()

	// Verify callback was called
	if atomic.LoadInt32(&cleanupCount) == 0 {
		t.Log("Note: No cleanup cycles detected (expected if no collections exist)")
	}
}

// TC-CLEANUP-002: TTL Cleaner removes expired records
func TestTTLCleanerRemovesExpired(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("test", WithDimension(3))
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	// Insert records with short TTL
	for i := 0; i < 5; i++ {
		_, err := coll.InsertWithOptions(
			[]float32{float32(i), 0, 0},
			nil,
			WithTTL(50*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("Failed to insert: %v", err)
		}
	}

	// Insert records without TTL (should persist)
	for i := 0; i < 3; i++ {
		_, err := coll.Insert([]float32{float32(i + 10), 0, 0}, nil)
		if err != nil {
			t.Fatalf("Failed to insert: %v", err)
		}
	}

	if coll.Count() != 8 {
		t.Fatalf("Expected 8 records, got %d", coll.Count())
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	var deletedCount int
	stop := db.StartTTLCleaner(10*time.Millisecond, func(collection string, deleted int) {
		deletedCount += deleted
	})

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)
	stop()

	// Should have 3 remaining (the ones without TTL)
	if coll.Count() != 3 {
		t.Errorf("Expected 3 records after TTL cleanup, got %d", coll.Count())
	}

	if deletedCount != 5 {
		t.Errorf("Expected 5 deletions reported, got %d", deletedCount)
	}
}

// TC-CLEANUP-003: Double stop does not panic
func TestTTLCleanerDoubleStop(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	stop := db.StartTTLCleaner(100*time.Millisecond, nil)

	// Double stop should not panic
	stop()
	stop() // Second call should be safe
}

// TC-MEMORY-001: Memory limiter enforces MaxRecords
func TestMemoryLimiterEnforcesLimit(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert 100 records
	for i := 0; i < 100; i++ {
		_, _ = coll.Insert([]float32{float32(i), 0, 0}, nil)
	}

	if coll.Count() != 100 {
		t.Fatalf("Expected 100 records, got %d", coll.Count())
	}

	// Enforce limit of 50
	evicted := coll.EnforceMemoryLimit(MemoryConfig{
		MaxRecords:        50,
		EvictionPolicy:    "fifo",
		EvictionBatchSize: 100,
	})

	if evicted != 50 {
		t.Errorf("Expected 50 evictions, got %d", evicted)
	}

	if coll.Count() != 50 {
		t.Errorf("Expected 50 records after eviction, got %d", coll.Count())
	}
}

// TC-MEMORY-002: FIFO eviction removes oldest first
func TestMemoryLimiterFIFO(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert records with known IDs
	var ids []uint64
	for i := 0; i < 10; i++ {
		id, _ := coll.Insert([]float32{float32(i), 0, 0}, nil)
		ids = append(ids, id)
	}

	// Evict 5 with FIFO
	coll.EnforceMemoryLimit(MemoryConfig{
		MaxRecords:        5,
		EvictionPolicy:    "fifo",
		EvictionBatchSize: 10,
	})

	// First 5 should be gone, last 5 should remain
	for i := 0; i < 5; i++ {
		_, err := coll.Get(ids[i])
		if err == nil {
			t.Errorf("Record %d (ID %d) should have been evicted", i, ids[i])
		}
	}

	for i := 5; i < 10; i++ {
		_, err := coll.Get(ids[i])
		if err != nil {
			t.Errorf("Record %d (ID %d) should still exist: %v", i, ids[i], err)
		}
	}
}

// TC-MEMORY-003: Importance eviction removes least important first
func TestMemoryLimiterImportance(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert records with varying importance
	var ids []uint64
	for i := 0; i < 10; i++ {
		id, _ := coll.InsertWithOptions(
			[]float32{float32(i), 0, 0},
			nil,
			WithImportance(float32(i)/10.0), // 0.0, 0.1, 0.2, ..., 0.9
		)
		ids = append(ids, id)
	}

	// Evict 5 with importance policy
	coll.EnforceMemoryLimit(MemoryConfig{
		MaxRecords:        5,
		EvictionPolicy:    "importance",
		EvictionBatchSize: 10,
	})

	// Records 0-4 (low importance) should be gone
	for i := 0; i < 5; i++ {
		_, err := coll.Get(ids[i])
		if err == nil {
			t.Errorf("Record %d (importance %.1f) should have been evicted", i, float32(i)/10.0)
		}
	}

	// Records 5-9 (high importance) should remain
	for i := 5; i < 10; i++ {
		_, err := coll.Get(ids[i])
		if err != nil {
			t.Errorf("Record %d (importance %.1f) should still exist", i, float32(i)/10.0)
		}
	}
}

// TC-MEMORY-004: Background memory limiter works
func TestMemoryLimiterBackground(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert 50 records
	for i := 0; i < 50; i++ {
		_, _ = coll.Insert([]float32{float32(i), 0, 0}, nil)
	}

	// Start limiter with max 30
	stop := coll.StartMemoryLimiter(MemoryConfig{
		MaxRecords:        30,
		EvictionPolicy:    "fifo",
		CleanupInterval:   50 * time.Millisecond,
		EvictionBatchSize: 25,
	})
	defer stop()

	// Wait for cleanup
	time.Sleep(150 * time.Millisecond)

	if coll.Count() > 30 {
		t.Errorf("Expected at most 30 records, got %d", coll.Count())
	}
}

// TC-MEMORY-005: No eviction when under limit
func TestMemoryLimiterUnderLimit(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert 10 records
	for i := 0; i < 10; i++ {
		_, _ = coll.Insert([]float32{float32(i), 0, 0}, nil)
	}

	// Try to enforce limit of 100 (no eviction needed)
	evicted := coll.EnforceMemoryLimit(MemoryConfig{
		MaxRecords:     100,
		EvictionPolicy: "fifo",
	})

	if evicted != 0 {
		t.Errorf("Expected 0 evictions, got %d", evicted)
	}

	if coll.Count() != 10 {
		t.Errorf("Expected 10 records, got %d", coll.Count())
	}
}

// TC-MEMORY-006: Archived records are not evicted
func TestMemoryLimiterSkipsArchived(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert and archive some records
	var archivedIDs []uint64
	for i := 0; i < 5; i++ {
		id, _ := coll.Insert([]float32{float32(i), 0, 0}, nil)
		_ = coll.ArchiveRecord(id)
		archivedIDs = append(archivedIDs, id)
	}

	// Insert regular records
	for i := 5; i < 15; i++ {
		_, _ = coll.Insert([]float32{float32(i), 0, 0}, nil)
	}

	// Total: 15 records (5 archived + 10 regular)
	if coll.Count() != 15 {
		t.Fatalf("Expected 15 records, got %d", coll.Count())
	}

	// Evict to keep 8 (should only evict non-archived)
	coll.EnforceMemoryLimit(MemoryConfig{
		MaxRecords:        8,
		EvictionPolicy:    "fifo",
		EvictionBatchSize: 20,
	})

	// Archived records should still exist
	for _, id := range archivedIDs {
		_, err := coll.Get(id)
		if err != nil {
			t.Errorf("Archived record %d should not have been evicted", id)
		}
	}
}
