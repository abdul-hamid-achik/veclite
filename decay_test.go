package veclite

import (
	"testing"
	"time"
)

func TestDecayFunctions(t *testing.T) {
	t.Run("exponential decay", func(t *testing.T) {
		config := &DecayConfig{
			Type:     DecayExponential,
			HalfLife: time.Hour,
		}

		// At time 0, score should be unchanged
		score := applyDecay(1.0, 0, config)
		if score != 1.0 {
			t.Errorf("expected score 1.0 at age 0, got %v", score)
		}

		// At half-life, score should be 0.5
		score = applyDecay(1.0, time.Hour, config)
		if score < 0.49 || score > 0.51 {
			t.Errorf("expected score ~0.5 at half-life, got %v", score)
		}

		// At 2x half-life, score should be 0.25
		score = applyDecay(1.0, 2*time.Hour, config)
		if score < 0.24 || score > 0.26 {
			t.Errorf("expected score ~0.25 at 2x half-life, got %v", score)
		}
	})

	t.Run("linear decay", func(t *testing.T) {
		config := &DecayConfig{
			Type:     DecayLinear,
			HalfLife: time.Hour, // Used as max age for linear
		}

		// At time 0, score should be unchanged
		score := applyDecay(1.0, 0, config)
		if score != 1.0 {
			t.Errorf("expected score 1.0 at age 0, got %v", score)
		}

		// At half the max age, score should be 0.5
		score = applyDecay(1.0, 30*time.Minute, config)
		if score < 0.49 || score > 0.51 {
			t.Errorf("expected score ~0.5 at half max age, got %v", score)
		}

		// At max age, score should be 0
		score = applyDecay(1.0, time.Hour, config)
		if score != 0.0 {
			t.Errorf("expected score 0.0 at max age, got %v", score)
		}

		// Beyond max age, score should still be 0
		score = applyDecay(1.0, 2*time.Hour, config)
		if score != 0.0 {
			t.Errorf("expected score 0.0 beyond max age, got %v", score)
		}
	})

	t.Run("gaussian decay", func(t *testing.T) {
		config := &DecayConfig{
			Type:     DecayGaussian,
			HalfLife: time.Hour, // Used as sigma
		}

		// At time 0, score should be unchanged
		score := applyDecay(1.0, 0, config)
		if score != 1.0 {
			t.Errorf("expected score 1.0 at age 0, got %v", score)
		}

		// At sigma, score should be ~0.606 (exp(-0.5))
		score = applyDecay(1.0, time.Hour, config)
		if score < 0.59 || score > 0.62 {
			t.Errorf("expected score ~0.606 at sigma, got %v", score)
		}

		// At 2x sigma, score should be much lower
		score = applyDecay(1.0, 2*time.Hour, config)
		if score > 0.2 {
			t.Errorf("expected score < 0.2 at 2x sigma, got %v", score)
		}
	})

	t.Run("no decay", func(t *testing.T) {
		// Nil config
		score := applyDecay(1.0, time.Hour, nil)
		if score != 1.0 {
			t.Errorf("expected score unchanged with nil config, got %v", score)
		}

		// DecayNone type
		config := &DecayConfig{Type: DecayNone}
		score = applyDecay(1.0, time.Hour, config)
		if score != 1.0 {
			t.Errorf("expected score unchanged with DecayNone, got %v", score)
		}

		// Zero half-life
		config = &DecayConfig{Type: DecayExponential, HalfLife: 0}
		score = applyDecay(1.0, time.Hour, config)
		if score != 1.0 {
			t.Errorf("expected score unchanged with zero half-life, got %v", score)
		}
	})
}

func TestImportanceBoost(t *testing.T) {
	t.Run("importance boost applies correctly", func(t *testing.T) {
		// No boost
		score := applyImportanceBoost(1.0, 0.5, 0)
		if score != 1.0 {
			t.Errorf("expected score unchanged with zero boost, got %v", score)
		}

		// With 1.0 boost and 0.5 importance: 1.0 * (1 + 1.0 * 0.5) = 1.5
		score = applyImportanceBoost(1.0, 0.5, 1.0)
		if score != 1.5 {
			t.Errorf("expected score 1.5 with 1.0 boost and 0.5 importance, got %v", score)
		}

		// With 1.0 boost and 1.0 importance: 1.0 * (1 + 1.0 * 1.0) = 2.0
		score = applyImportanceBoost(1.0, 1.0, 1.0)
		if score != 2.0 {
			t.Errorf("expected score 2.0 with 1.0 boost and 1.0 importance, got %v", score)
		}

		// With 0.5 boost and 0.8 importance: 1.0 * (1 + 0.5 * 0.8) = 1.4
		score = applyImportanceBoost(1.0, 0.8, 0.5)
		if score < 1.39 || score > 1.41 {
			t.Errorf("expected score ~1.4 with 0.5 boost and 0.8 importance, got %v", score)
		}
	})
}

func TestSearchWithDecay(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("test")

	// Insert records with different ages (simulated via direct field modification)
	vec := []float32{1.0, 0.0, 0.0}

	// Recent record
	_, _ = coll.InsertWithOptions(vec, map[string]any{"name": "recent"}, WithImportance(0.5))
	// Older record
	id2, _ := coll.InsertWithOptions(vec, map[string]any{"name": "old"}, WithImportance(0.5))

	// Manually adjust CreatedAt to simulate age
	coll.mu.Lock()
	coll.records[id2].CreatedAt = time.Now().Add(-24 * time.Hour)
	coll.mu.Unlock()

	t.Run("search without decay returns both with same score", func(t *testing.T) {
		results, err := coll.Search(vec, TopK(10))
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		// Scores should be identical (both have same vector)
		if results[0].Score != results[1].Score {
			t.Errorf("expected identical scores without decay, got %v and %v", results[0].Score, results[1].Score)
		}
	})

	t.Run("search with decay prefers recent records", func(t *testing.T) {
		results, err := coll.Search(vec, TopK(10), WithDecay(DecayExponential, 12*time.Hour))
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		// Recent record should have higher score
		if results[0].Record.Payload["name"] != "recent" {
			t.Error("expected recent record to rank first with decay")
		}
		if results[0].Score <= results[1].Score {
			t.Errorf("expected recent record to have higher score, got %v <= %v", results[0].Score, results[1].Score)
		}
	})
}

func TestSearchWithImportanceBoost(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("test")

	// Insert records with different importance
	vec := []float32{1.0, 0.0, 0.0}

	_, _ = coll.InsertWithOptions(vec, map[string]any{"name": "low"}, WithImportance(0.1))
	_, _ = coll.InsertWithOptions(vec, map[string]any{"name": "high"}, WithImportance(0.9))

	t.Run("search with importance boost prefers high importance", func(t *testing.T) {
		results, err := coll.Search(vec, TopK(10), WithImportanceBoost(2.0))
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		// High importance record should rank first
		if results[0].Record.Payload["name"] != "high" {
			t.Error("expected high importance record to rank first")
		}
	})
}

func TestAccessTracking(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("test")

	vec := []float32{1.0, 0.0, 0.0}
	id, _ := coll.Insert(vec, nil)

	// Check initial state
	record, _ := coll.Get(id)
	if record.AccessCount != 0 {
		t.Errorf("expected initial AccessCount 0, got %d", record.AccessCount)
	}
	if !record.LastAccessedAt.IsZero() {
		t.Error("expected initial LastAccessedAt to be zero")
	}

	// Search without access tracking
	_, err = coll.Search(vec, TopK(10))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	record, _ = coll.Get(id)
	if record.AccessCount != 0 {
		t.Errorf("expected AccessCount to remain 0 without tracking, got %d", record.AccessCount)
	}

	// Search with access tracking
	_, err = coll.Search(vec, TopK(10), WithAccessTracking(true))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	record, _ = coll.Get(id)
	if record.AccessCount != 1 {
		t.Errorf("expected AccessCount 1 after tracked search, got %d", record.AccessCount)
	}
	if record.LastAccessedAt.IsZero() {
		t.Error("expected LastAccessedAt to be set after tracked search")
	}

	// Search again
	_, err = coll.Search(vec, TopK(10), WithAccessTracking(true))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	record, _ = coll.Get(id)
	if record.AccessCount != 2 {
		t.Errorf("expected AccessCount 2 after second tracked search, got %d", record.AccessCount)
	}
}
