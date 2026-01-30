package veclite

import (
	"testing"
	"time"
)

func TestTimestampFilters(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	record := &Record{
		ID:             1,
		CreatedAt:      past,
		UpdatedAt:      now,
		ExpiresAt:      future,
		Importance:     0.7,
		AccessCount:    5,
		LastAccessedAt: past.Add(30 * time.Minute),
	}

	t.Run("CreatedAfter", func(t *testing.T) {
		// Should match: created after 2 hours ago
		if !CreatedAfter(past.Add(-time.Hour)).Match(record) {
			t.Error("expected match for time before creation")
		}
		// Should not match: created after 30 minutes ago
		if CreatedAfter(now.Add(-30 * time.Minute)).Match(record) {
			t.Error("expected no match for time after creation")
		}
	})

	t.Run("CreatedBefore", func(t *testing.T) {
		// Should match: created before now
		if !CreatedBefore(now).Match(record) {
			t.Error("expected match for time after creation")
		}
		// Should not match: created before 2 hours ago
		if CreatedBefore(past.Add(-time.Hour)).Match(record) {
			t.Error("expected no match for time before creation")
		}
	})

	t.Run("UpdatedAfter", func(t *testing.T) {
		// Should match: updated after 1 minute ago
		if !UpdatedAfter(now.Add(-time.Minute)).Match(record) {
			t.Error("expected match for time before update")
		}
		// Should not match: updated after now + 1 minute
		if UpdatedAfter(now.Add(time.Minute)).Match(record) {
			t.Error("expected no match for time after update")
		}
	})

	t.Run("UpdatedBefore", func(t *testing.T) {
		// Should match: updated before future
		if !UpdatedBefore(future).Match(record) {
			t.Error("expected match for time after update")
		}
		// Should not match: updated before past
		if UpdatedBefore(past).Match(record) {
			t.Error("expected no match for time before update")
		}
	})

	t.Run("AgeOlderThan", func(t *testing.T) {
		// Should match: older than 30 minutes
		if !AgeOlderThan(30 * time.Minute).Match(record) {
			t.Error("expected match for age older than 30 minutes")
		}
		// Should not match: older than 2 hours
		if AgeOlderThan(2 * time.Hour).Match(record) {
			t.Error("expected no match for age older than 2 hours")
		}
	})

	t.Run("AgeNewerThan", func(t *testing.T) {
		// Should match: newer than 2 hours
		if !AgeNewerThan(2 * time.Hour).Match(record) {
			t.Error("expected match for age newer than 2 hours")
		}
		// Should not match: newer than 30 minutes
		if AgeNewerThan(30 * time.Minute).Match(record) {
			t.Error("expected no match for age newer than 30 minutes")
		}
	})
}

func TestTTLFilters(t *testing.T) {
	now := time.Now()

	t.Run("NotExpired", func(t *testing.T) {
		// Record without TTL
		noTTL := &Record{ID: 1}
		if !NotExpired().Match(noTTL) {
			t.Error("expected match for record without TTL")
		}

		// Record with future expiration
		futureExpire := &Record{ID: 2, ExpiresAt: now.Add(time.Hour)}
		if !NotExpired().Match(futureExpire) {
			t.Error("expected match for non-expired record")
		}

		// Record with past expiration
		pastExpire := &Record{ID: 3, ExpiresAt: now.Add(-time.Hour)}
		if NotExpired().Match(pastExpire) {
			t.Error("expected no match for expired record")
		}
	})

	t.Run("ExpiredBefore", func(t *testing.T) {
		// Record without TTL
		noTTL := &Record{ID: 1}
		if ExpiredBefore(now).Match(noTTL) {
			t.Error("expected no match for record without TTL")
		}

		// Record expired 2 hours ago
		oldExpire := &Record{ID: 2, ExpiresAt: now.Add(-2 * time.Hour)}
		if !ExpiredBefore(now.Add(-time.Hour)).Match(oldExpire) {
			t.Error("expected match for record expired before threshold")
		}

		// Record expired 30 minutes ago
		recentExpire := &Record{ID: 3, ExpiresAt: now.Add(-30 * time.Minute)}
		if ExpiredBefore(now.Add(-time.Hour)).Match(recentExpire) {
			t.Error("expected no match for record expired after threshold")
		}
	})

	t.Run("HasTTLFilter", func(t *testing.T) {
		// Record without TTL
		noTTL := &Record{ID: 1}
		if HasTTLFilter().Match(noTTL) {
			t.Error("expected no match for record without TTL")
		}

		// Record with TTL
		withTTL := &Record{ID: 2, ExpiresAt: now.Add(time.Hour)}
		if !HasTTLFilter().Match(withTTL) {
			t.Error("expected match for record with TTL")
		}
	})
}

func TestImportanceFilters(t *testing.T) {
	record := &Record{ID: 1, Importance: 0.7}

	t.Run("ImportanceAbove", func(t *testing.T) {
		if !ImportanceAbove(0.5).Match(record) {
			t.Error("expected match for importance > 0.5")
		}
		if ImportanceAbove(0.8).Match(record) {
			t.Error("expected no match for importance > 0.8")
		}
		if ImportanceAbove(0.7).Match(record) {
			t.Error("expected no match for importance > 0.7 (exact)")
		}
	})

	t.Run("ImportanceBelow", func(t *testing.T) {
		if !ImportanceBelow(0.8).Match(record) {
			t.Error("expected match for importance < 0.8")
		}
		if ImportanceBelow(0.5).Match(record) {
			t.Error("expected no match for importance < 0.5")
		}
		if ImportanceBelow(0.7).Match(record) {
			t.Error("expected no match for importance < 0.7 (exact)")
		}
	})

	t.Run("ImportanceBetween", func(t *testing.T) {
		if !ImportanceBetween(0.5, 0.9).Match(record) {
			t.Error("expected match for 0.5 <= importance <= 0.9")
		}
		if !ImportanceBetween(0.7, 0.7).Match(record) {
			t.Error("expected match for exact match 0.7 <= importance <= 0.7")
		}
		if ImportanceBetween(0.8, 1.0).Match(record) {
			t.Error("expected no match for 0.8 <= importance <= 1.0")
		}
	})
}

func TestAccessFilters(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)

	accessed := &Record{
		ID:             1,
		AccessCount:    5,
		LastAccessedAt: past,
	}

	neverAccessed := &Record{
		ID:          2,
		AccessCount: 0,
	}

	t.Run("AccessedAfter", func(t *testing.T) {
		if !AccessedAfter(past.Add(-30 * time.Minute)).Match(accessed) {
			t.Error("expected match for accessed after")
		}
		if AccessedAfter(now).Match(accessed) {
			t.Error("expected no match for accessed after future")
		}
		if AccessedAfter(past).Match(neverAccessed) {
			t.Error("expected no match for never accessed record")
		}
	})

	t.Run("AccessedBefore", func(t *testing.T) {
		if !AccessedBefore(now).Match(accessed) {
			t.Error("expected match for accessed before now")
		}
		if AccessedBefore(past.Add(-time.Hour)).Match(accessed) {
			t.Error("expected no match for accessed before earlier time")
		}
		if !AccessedBefore(now).Match(neverAccessed) {
			t.Error("expected match for never accessed record (before any time)")
		}
	})

	t.Run("NeverAccessed", func(t *testing.T) {
		if NeverAccessed().Match(accessed) {
			t.Error("expected no match for accessed record")
		}
		if !NeverAccessed().Match(neverAccessed) {
			t.Error("expected match for never accessed record")
		}
	})

	t.Run("AccessCountAbove", func(t *testing.T) {
		if !AccessCountAbove(3).Match(accessed) {
			t.Error("expected match for access count > 3")
		}
		if AccessCountAbove(5).Match(accessed) {
			t.Error("expected no match for access count > 5 (exact)")
		}
		if AccessCountAbove(0).Match(neverAccessed) {
			t.Error("expected no match for never accessed (count > 0)")
		}
	})

	t.Run("AccessCountBelow", func(t *testing.T) {
		if !AccessCountBelow(10).Match(accessed) {
			t.Error("expected match for access count < 10")
		}
		if AccessCountBelow(5).Match(accessed) {
			t.Error("expected no match for access count < 5 (exact)")
		}
		if !AccessCountBelow(1).Match(neverAccessed) {
			t.Error("expected match for never accessed (count < 1)")
		}
	})
}

func TestFiltersWithCollection(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")
	vec := []float32{0.1, 0.2, 0.3}

	// Insert records with different properties
	// High importance, no TTL
	coll.InsertWithOptions(vec, map[string]any{"name": "high"}, WithImportance(0.9))

	// Low importance, with TTL
	coll.InsertWithOptions(vec, map[string]any{"name": "low"}, WithImportance(0.2), WithTTL(time.Hour))

	// Medium importance, expired
	coll.InsertWithOptions(vec, map[string]any{"name": "expired"}, WithImportance(0.5), WithExpiresAt(time.Now().Add(-time.Hour)))

	// Search with NotExpired filter
	t.Run("Find with NotExpired", func(t *testing.T) {
		records, err := coll.Find(NotExpired())
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}
		if len(records) != 2 {
			t.Errorf("expected 2 non-expired records, got %d", len(records))
		}
	})

	// Search with ImportanceAbove filter
	t.Run("Find with ImportanceAbove", func(t *testing.T) {
		records, err := coll.Find(ImportanceAbove(0.5))
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}
		if len(records) != 1 {
			t.Errorf("expected 1 high importance record, got %d", len(records))
		}
		if len(records) > 0 && records[0].Payload["name"] != "high" {
			t.Errorf("expected 'high' record, got %v", records[0].Payload["name"])
		}
	})

	// Search with combined filters
	t.Run("Find with combined filters", func(t *testing.T) {
		records, err := coll.Find(
			NotExpired(),
			ImportanceBetween(0.1, 0.5),
		)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}
		if len(records) != 1 {
			t.Errorf("expected 1 record, got %d", len(records))
		}
		if len(records) > 0 && records[0].Payload["name"] != "low" {
			t.Errorf("expected 'low' record, got %v", records[0].Payload["name"])
		}
	})
}
