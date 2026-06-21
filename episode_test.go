package veclite

import (
	"testing"
	"time"
)

func TestCreateEpisode(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("memories")

	// Insert some records
	id1, _ := coll.Insert([]float32{1, 0, 0, 0}, map[string]any{"content": "morning coffee"})
	id2, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, map[string]any{"content": "breakfast meeting"})
	id3, _ := coll.Insert([]float32{0.8, 0.2, 0, 0}, map[string]any{"content": "email review"})

	es, err := db.CreateEpisodeStore("memories")
	if err != nil {
		t.Fatalf("CreateEpisodeStore failed: %v", err)
	}

	t.Run("create episode", func(t *testing.T) {
		episode, err := es.CreateEpisode([]uint64{id1, id2, id3}, "Morning routine")
		if err != nil {
			t.Fatalf("CreateEpisode failed: %v", err)
		}

		if episode.ID == "" {
			t.Error("expected episode ID")
		}
		if episode.Title != "Morning routine" {
			t.Errorf("expected title 'Morning routine', got %q", episode.Title)
		}
		if len(episode.RecordIDs) != 3 {
			t.Errorf("expected 3 record IDs, got %d", len(episode.RecordIDs))
		}
		if len(episode.Vector) != 4 {
			t.Errorf("expected 4-dim centroid vector, got %d", len(episode.Vector))
		}
	})

	t.Run("create episode with invalid record", func(t *testing.T) {
		_, err := es.CreateEpisode([]uint64{99999}, "Invalid")
		if err == nil {
			t.Error("expected error for invalid record ID")
		}
	})

	t.Run("create episode empty records", func(t *testing.T) {
		_, err := es.CreateEpisode([]uint64{}, "Empty")
		if err == nil {
			t.Error("expected error for empty record list")
		}
	})
}

func TestDetectEpisodes(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("memories")

	// Insert records with time gaps
	// Episode 1: close in time
	_, _ = coll.Insert([]float32{1, 0, 0, 0}, nil)
	time.Sleep(10 * time.Millisecond)
	_, _ = coll.Insert([]float32{0.9, 0.1, 0, 0}, nil)
	time.Sleep(10 * time.Millisecond)
	_, _ = coll.Insert([]float32{0.8, 0.2, 0, 0}, nil)

	// Gap (simulated by just continuing - actual time gap would be larger)
	time.Sleep(50 * time.Millisecond)

	// Episode 2: another cluster
	_, _ = coll.Insert([]float32{0, 1, 0, 0}, nil)
	time.Sleep(10 * time.Millisecond)
	_, _ = coll.Insert([]float32{0.1, 0.9, 0, 0}, nil)

	es, err := db.CreateEpisodeStore("memories")
	if err != nil {
		t.Fatalf("CreateEpisodeStore failed: %v", err)
	}

	t.Run("detect with time gap", func(t *testing.T) {
		episodes, err := es.DetectEpisodes(EpisodeConfig{
			TimeGapThreshold: 40 * time.Millisecond,
			MinRecords:       2,
		})
		if err != nil {
			t.Fatalf("DetectEpisodes failed: %v", err)
		}

		if len(episodes) < 1 {
			t.Errorf("expected at least 1 episode, got %d", len(episodes))
		}
	})

	t.Run("detect with small min records", func(t *testing.T) {
		episodes, err := es.DetectEpisodes(EpisodeConfig{
			TimeGapThreshold: time.Hour, // Large gap - everything in one episode
			MinRecords:       2,
		})
		if err != nil {
			t.Fatalf("DetectEpisodes failed: %v", err)
		}

		// With a large time gap threshold, should get one or few episodes
		if len(episodes) == 0 {
			t.Error("expected at least one episode")
		}
	})
}

func TestExpandEpisode(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("memories")

	id1, _ := coll.Insert([]float32{1, 0, 0, 0}, map[string]any{"order": 1})
	time.Sleep(time.Millisecond)
	id2, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, map[string]any{"order": 2})
	time.Sleep(time.Millisecond)
	id3, _ := coll.Insert([]float32{0.8, 0.2, 0, 0}, map[string]any{"order": 3})

	es, _ := db.CreateEpisodeStore("memories")
	episode, _ := es.CreateEpisode([]uint64{id1, id2, id3}, "Test Episode")

	t.Run("expand episode", func(t *testing.T) {
		records, err := es.ExpandEpisode(episode.ID)
		if err != nil {
			t.Fatalf("ExpandEpisode failed: %v", err)
		}

		if len(records) != 3 {
			t.Errorf("expected 3 records, got %d", len(records))
		}

		// Check sorted by creation time
		for i := 1; i < len(records); i++ {
			if records[i].CreatedAt.Before(records[i-1].CreatedAt) {
				t.Error("records not sorted by creation time")
			}
		}
	})

	t.Run("expand non-existent episode", func(t *testing.T) {
		_, err := es.ExpandEpisode("non-existent")
		if err == nil {
			t.Error("expected error for non-existent episode")
		}
	})
}

func TestEpisodeOperations(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("memories")
	id1, _ := coll.Insert([]float32{1, 0, 0, 0}, nil)
	id2, _ := coll.Insert([]float32{0, 1, 0, 0}, nil)

	es, _ := db.CreateEpisodeStore("memories")
	ep1, _ := es.CreateEpisode([]uint64{id1}, "Episode 1")
	ep2, _ := es.CreateEpisode([]uint64{id2}, "Episode 2")

	t.Run("get episode", func(t *testing.T) {
		episode, err := es.GetEpisode(ep1.ID)
		if err != nil {
			t.Fatalf("GetEpisode failed: %v", err)
		}
		if episode.Title != "Episode 1" {
			t.Errorf("expected title 'Episode 1', got %q", episode.Title)
		}
	})

	t.Run("list episodes", func(t *testing.T) {
		episodes := es.ListEpisodes()
		if len(episodes) != 2 {
			t.Errorf("expected 2 episodes, got %d", len(episodes))
		}
	})

	t.Run("delete episode", func(t *testing.T) {
		err := es.DeleteEpisode(ep2.ID)
		if err != nil {
			t.Fatalf("DeleteEpisode failed: %v", err)
		}

		episodes := es.ListEpisodes()
		if len(episodes) != 1 {
			t.Errorf("expected 1 episode after delete, got %d", len(episodes))
		}
	})

	t.Run("find record episode", func(t *testing.T) {
		foundEpisode, err := es.FindRecordEpisode(id1)
		if err != nil {
			t.Fatalf("FindRecordEpisode failed: %v", err)
		}
		if foundEpisode == nil {
			t.Fatal("expected to find episode for record")
		}
		if foundEpisode.ID != ep1.ID {
			t.Errorf("expected episode %s, got %s", ep1.ID, foundEpisode.ID)
		}

		// Record not in any episode
		id3, _ := coll.Insert([]float32{0, 0, 1, 0}, nil)
		foundEpisode, err = es.FindRecordEpisode(id3)
		if err != nil {
			t.Fatalf("FindRecordEpisode failed: %v", err)
		}
		if foundEpisode != nil {
			t.Error("expected no episode for unassigned record")
		}
	})
}

func TestSearchWithEpisodeExpansion(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("memories")

	// Create records
	id1, _ := coll.Insert([]float32{1, 0, 0, 0}, map[string]any{"content": "main"})
	id2, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, map[string]any{"content": "related1"})
	id3, _ := coll.Insert([]float32{0.8, 0.2, 0, 0}, map[string]any{"content": "related2"})
	_, _ = coll.Insert([]float32{0, 0, 1, 0}, map[string]any{"content": "unrelated"})

	es, _ := db.CreateEpisodeStore("memories")
	_, _ = es.CreateEpisode([]uint64{id1, id2, id3}, "Test Episode")

	t.Run("search with expansion", func(t *testing.T) {
		results, err := es.SearchWithEpisodeExpansion([]float32{1, 0, 0, 0}, WithLimit(5))
		if err != nil {
			t.Fatalf("SearchWithEpisodeExpansion failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("expected at least one result")
		}

		// First result should be the closest match
		firstResult := results[0]
		if firstResult.Result.Record.ID != id1 {
			t.Errorf("expected first result to be id1")
		}

		// Should have episode context
		if firstResult.Episode == nil {
			t.Error("expected episode context for result")
		}

		// Should have related records
		if len(firstResult.EpisodeRecords) != 2 { // id2 and id3, not id1
			t.Errorf("expected 2 episode records, got %d", len(firstResult.EpisodeRecords))
		}
	})
}

func TestSearchEpisodes(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("memories")

	// Create records in different "directions"
	id1, _ := coll.Insert([]float32{1, 0, 0, 0}, nil)
	id2, _ := coll.Insert([]float32{0, 1, 0, 0}, nil)

	es, _ := db.CreateEpisodeStore("memories")
	_, _ = es.CreateEpisode([]uint64{id1}, "X-direction episode")
	_, _ = es.CreateEpisode([]uint64{id2}, "Y-direction episode")

	t.Run("search episodes by vector", func(t *testing.T) {
		episodes, err := es.SearchEpisodes([]float32{0.9, 0.1, 0, 0}, 5)
		if err != nil {
			t.Fatalf("SearchEpisodes failed: %v", err)
		}

		if len(episodes) != 2 {
			t.Errorf("expected 2 episodes, got %d", len(episodes))
		}

		// First episode should be closer to X-direction
		if episodes[0].Title != "X-direction episode" {
			t.Errorf("expected X-direction episode first, got %q", episodes[0].Title)
		}
	})
}

func TestEpisodeDuration(t *testing.T) {
	start := time.Now()
	end := start.Add(time.Hour)
	episode := &Episode{
		TimeRange: TimeRange{
			Start: start,
			End:   end,
		},
	}

	duration := episode.Duration()
	if duration != time.Hour {
		t.Errorf("expected duration of 1 hour, got %v", duration)
	}
}

func TestEpisodeRecordCount(t *testing.T) {
	episode := &Episode{
		RecordIDs: []uint64{1, 2, 3, 4, 5},
	}

	if episode.RecordCount() != 5 {
		t.Errorf("expected record count of 5, got %d", episode.RecordCount())
	}
}

func TestEpisodeStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.veclite"

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	coll := db.Collection("ep-data")
	_, err = coll.Insert([]float32{1, 0, 0}, map[string]any{"text": "hello world"})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	es, err := db.CreateEpisodeStore("ep-data")
	if err != nil {
		t.Fatalf("CreateEpisodeStore failed: %v", err)
	}

	_, err = es.CreateEpisode([]uint64{1}, "test episode")
	if err != nil {
		t.Fatalf("CreateEpisode failed: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Open for restore failed: %v", err)
	}
	defer func() { _ = db2.Close() }()

	es2, err := db2.GetEpisodeStore("ep-data")
	if err != nil {
		t.Fatalf("GetEpisodeStore after restore failed: %v", err)
	}

	episodes := es2.ListEpisodes()
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode after restore, got %d", len(episodes))
	}
	if episodes[0].Title != "test episode" {
		t.Errorf("expected title 'test episode', got %s", episodes[0].Title)
	}
}
