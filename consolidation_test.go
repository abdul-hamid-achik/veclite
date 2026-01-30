package veclite

import (
	"strings"
	"testing"
)

func TestFindSimilarClusters(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	// Insert similar vectors that should cluster together
	// Cluster 1: vectors pointing mostly in X direction
	coll.Insert([]float32{0.9, 0.1, 0.0, 0.0}, map[string]any{"group": "x"})
	coll.Insert([]float32{0.85, 0.15, 0.0, 0.0}, map[string]any{"group": "x"})
	coll.Insert([]float32{0.95, 0.05, 0.0, 0.0}, map[string]any{"group": "x"})

	// Cluster 2: vectors pointing mostly in Y direction
	coll.Insert([]float32{0.1, 0.9, 0.0, 0.0}, map[string]any{"group": "y"})
	coll.Insert([]float32{0.15, 0.85, 0.0, 0.0}, map[string]any{"group": "y"})

	// Outlier (not similar to either cluster)
	coll.Insert([]float32{0.0, 0.0, 1.0, 0.0}, map[string]any{"group": "z"})

	t.Run("basic clustering", func(t *testing.T) {
		clusters, err := coll.FindSimilarClusters(ConsolidationConfig{
			SimilarityThreshold: 0.9,
			MinGroupSize:        2,
		})
		if err != nil {
			t.Fatalf("FindSimilarClusters failed: %v", err)
		}

		if len(clusters) < 2 {
			t.Errorf("expected at least 2 clusters, got %d", len(clusters))
		}

		// Check that clusters have the right size
		for _, cluster := range clusters {
			if len(cluster.Records) < 2 {
				t.Error("cluster should have at least 2 records")
			}
		}
	})

	t.Run("max group size", func(t *testing.T) {
		clusters, err := coll.FindSimilarClusters(ConsolidationConfig{
			SimilarityThreshold: 0.9,
			MinGroupSize:        2,
			MaxGroupSize:        2,
		})
		if err != nil {
			t.Fatalf("FindSimilarClusters failed: %v", err)
		}

		for _, cluster := range clusters {
			if len(cluster.Records) > 2 {
				t.Errorf("cluster exceeded max size: %d", len(cluster.Records))
			}
		}
	})

	t.Run("filter records", func(t *testing.T) {
		clusters, err := coll.FindSimilarClusters(ConsolidationConfig{
			SimilarityThreshold: 0.9,
			MinGroupSize:        2,
			Filters:             []Filter{Equal("group", "x")},
		})
		if err != nil {
			t.Fatalf("FindSimilarClusters failed: %v", err)
		}

		// Should only find cluster in group x
		for _, cluster := range clusters {
			for _, r := range cluster.Records {
				if r.Payload["group"] != "x" {
					t.Error("found record outside filter")
				}
			}
		}
	})
}

func TestConsolidate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	// Insert similar vectors
	coll.InsertWithOptions([]float32{0.9, 0.1, 0.0, 0.0}, map[string]any{"content": "apple"}, WithImportance(0.7))
	coll.InsertWithOptions([]float32{0.85, 0.15, 0.0, 0.0}, map[string]any{"content": "apricot"}, WithImportance(0.9))
	coll.InsertWithOptions([]float32{0.95, 0.05, 0.0, 0.0}, map[string]any{"content": "avocado"}, WithImportance(0.5))

	t.Run("without summary generator", func(t *testing.T) {
		result, err := coll.Consolidate(ConsolidationConfig{
			SimilarityThreshold: 0.9,
			MinGroupSize:        2,
		})
		if err != nil {
			t.Fatalf("Consolidate failed: %v", err)
		}

		if result.ClustersFound == 0 {
			t.Error("expected at least one cluster")
		}
		// No consolidation records created without summary generator
		if len(result.ConsolidatedRecordIDs) > 0 {
			t.Error("should not create consolidation records without summary generator")
		}
	})

	t.Run("with summary generator", func(t *testing.T) {
		// Create a fresh collection
		coll2 := db.Collection("test2")
		coll2.InsertWithOptions([]float32{0.9, 0.1, 0.0, 0.0}, map[string]any{"content": "apple"}, WithImportance(0.7))
		coll2.InsertWithOptions([]float32{0.85, 0.15, 0.0, 0.0}, map[string]any{"content": "apricot"}, WithImportance(0.9))
		coll2.InsertWithOptions([]float32{0.95, 0.05, 0.0, 0.0}, map[string]any{"content": "avocado"}, WithImportance(0.5))

		summaryGenerator := func(records []*Record) (string, map[string]any, error) {
			var contents []string
			for _, r := range records {
				if c, ok := r.Payload["content"].(string); ok {
					contents = append(contents, c)
				}
			}
			return "Summary of: " + strings.Join(contents, ", "), map[string]any{"source": "consolidation"}, nil
		}

		embedder := &testConsolidationEmbedder{dim: 4}

		result, err := coll2.Consolidate(ConsolidationConfig{
			SimilarityThreshold:  0.9,
			MinGroupSize:         2,
			SummaryGenerator:     summaryGenerator,
			Embedder:             embedder,
			ArchiveOriginals:     true,
		})
		if err != nil {
			t.Fatalf("Consolidate failed: %v", err)
		}

		if len(result.ConsolidatedRecordIDs) == 0 {
			t.Error("expected consolidation records")
		}

		// Check consolidated record
		if len(result.ConsolidatedRecordIDs) > 0 {
			record, err := coll2.Get(result.ConsolidatedRecordIDs[0])
			if err != nil {
				t.Fatalf("Get consolidated record failed: %v", err)
			}

			if record.Payload[PayloadKeyIsConsolidation] != true {
				t.Error("expected IsConsolidation flag")
			}

			// Check importance is max of originals
			if record.Importance != 0.9 {
				t.Errorf("expected importance 0.9 (max), got %v", record.Importance)
			}
		}

		// Check archives
		if len(result.ArchivedRecordIDs) == 0 {
			t.Error("expected archived records")
		}
	})

	t.Run("requires embedder with summary generator", func(t *testing.T) {
		coll3 := db.Collection("test3")
		coll3.Insert([]float32{0.9, 0.1, 0.0, 0.0}, nil)
		coll3.Insert([]float32{0.85, 0.15, 0.0, 0.0}, nil)

		_, err := coll3.Consolidate(ConsolidationConfig{
			SimilarityThreshold: 0.9,
			MinGroupSize:        2,
			SummaryGenerator: func([]*Record) (string, map[string]any, error) {
				return "summary", nil, nil
			},
			// Missing embedder
		})
		if err == nil {
			t.Error("expected error when embedder is missing")
		}
	})
}

func TestArchiveRecord(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	id, _ := coll.Insert([]float32{1, 0, 0, 0}, map[string]any{"data": "test"})

	t.Run("archive record", func(t *testing.T) {
		err := coll.ArchiveRecord(id)
		if err != nil {
			t.Fatalf("ArchiveRecord failed: %v", err)
		}

		record, _ := coll.Get(id)
		if record.Payload[PayloadKeyArchived] != true {
			t.Error("expected record to be archived")
		}
	})

	t.Run("get archived", func(t *testing.T) {
		archived, err := coll.GetArchived()
		if err != nil {
			t.Fatalf("GetArchived failed: %v", err)
		}

		if len(archived) != 1 {
			t.Errorf("expected 1 archived record, got %d", len(archived))
		}
	})

	t.Run("unarchive record", func(t *testing.T) {
		err := coll.UnarchiveRecord(id)
		if err != nil {
			t.Fatalf("UnarchiveRecord failed: %v", err)
		}

		record, _ := coll.Get(id)
		if _, ok := record.Payload[PayloadKeyArchived]; ok {
			t.Error("expected archived flag to be removed")
		}
	})

	t.Run("archive not found", func(t *testing.T) {
		err := coll.ArchiveRecord(99999)
		if err == nil {
			t.Error("expected error for non-existent record")
		}
	})
}

func TestGetConsolidations(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	// Insert regular records
	coll.Insert([]float32{1, 0, 0, 0}, map[string]any{"type": "regular"})

	// Insert consolidation record
	coll.Insert([]float32{0, 1, 0, 0}, map[string]any{
		PayloadKeyIsConsolidation: true,
		"type":                    "consolidated",
	})

	consolidations, err := coll.GetConsolidations()
	if err != nil {
		t.Fatalf("GetConsolidations failed: %v", err)
	}

	if len(consolidations) != 1 {
		t.Errorf("expected 1 consolidation, got %d", len(consolidations))
	}
}

func TestExpandConsolidation(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	coll := db.Collection("test")

	// Insert original records
	id1, _ := coll.Insert([]float32{0.9, 0.1, 0, 0}, map[string]any{"content": "one"})
	id2, _ := coll.Insert([]float32{0.85, 0.15, 0, 0}, map[string]any{"content": "two"})

	// Insert consolidation record
	consolidatedID, _ := coll.Insert([]float32{0.87, 0.13, 0, 0}, map[string]any{
		PayloadKeyIsConsolidation:  true,
		PayloadKeyConsolidatedFrom: []uint64{id1, id2},
	})

	t.Run("expand consolidation", func(t *testing.T) {
		records, err := coll.ExpandConsolidation(consolidatedID)
		if err != nil {
			t.Fatalf("ExpandConsolidation failed: %v", err)
		}

		if len(records) != 2 {
			t.Errorf("expected 2 original records, got %d", len(records))
		}
	})

	t.Run("expand non-consolidation", func(t *testing.T) {
		_, err := coll.ExpandConsolidation(id1)
		if err == nil {
			t.Error("expected error for non-consolidation record")
		}
	})

	t.Run("expand not found", func(t *testing.T) {
		_, err := coll.ExpandConsolidation(99999)
		if err == nil {
			t.Error("expected error for non-existent record")
		}
	})
}

func TestClusterComputation(t *testing.T) {
	t.Run("compute centroid", func(t *testing.T) {
		records := []*Record{
			{Vector: []float32{1, 0, 0, 0}},
			{Vector: []float32{0, 1, 0, 0}},
		}

		centroid := computeCentroid(records)

		if len(centroid) != 4 {
			t.Fatalf("expected 4-dim centroid, got %d", len(centroid))
		}

		if centroid[0] != 0.5 || centroid[1] != 0.5 {
			t.Errorf("unexpected centroid: %v", centroid)
		}
	})

	t.Run("compute time range", func(t *testing.T) {
		now := make([]Record, 3)
		records := make([]*Record, 3)
		for i := 0; i < 3; i++ {
			records[i] = &now[i]
		}

		// Set creation times
		// This is a simplified test - actual times would be set by Insert

		tr := computeTimeRange(records)
		_ = tr // Just ensure it doesn't panic
	})
}

// testConsolidationEmbedder for testing
type testConsolidationEmbedder struct {
	dim int
}

func (m *testConsolidationEmbedder) Embed(text string) ([]float32, error) {
	vec := make([]float32, m.dim)
	for i := range vec {
		vec[i] = float32(len(text)%10) / 10.0
	}
	return vec, nil
}

func (m *testConsolidationEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		vec, _ := m.Embed(text)
		result[i] = vec
	}
	return result, nil
}

func (m *testConsolidationEmbedder) Dimension() int {
	return m.dim
}
