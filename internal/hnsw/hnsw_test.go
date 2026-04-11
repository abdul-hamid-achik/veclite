package hnsw

import (
	"math"
	"math/rand"
	"testing"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// generateRandomVector generates a random unit vector of the given dimension.
func generateRandomVector(dim int, rng *rand.Rand) []float32 {
	vec := make([]float32, dim)
	var norm float32
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1
		norm += vec[i] * vec[i]
	}
	norm = float32(math.Sqrt(float64(norm)))
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func TestNew(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 128, floats.DistanceCosine)

	if idx.Count() != 0 {
		t.Errorf("expected empty index, got %d nodes", idx.Count())
	}
	if idx.Dimension() != 128 {
		t.Errorf("expected dimension 128, got %d", idx.Dimension())
	}
}

func TestInsertSingle(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	vec := []float32{1.0, 0.0, 0.0, 0.0}
	err := idx.Insert(1, vec)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if idx.Count() != 1 {
		t.Errorf("expected 1 node, got %d", idx.Count())
	}

	if !idx.HasNode(1) {
		t.Error("node 1 not found")
	}
}

func TestInsertMultiple(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	vectors := [][]float32{
		{1.0, 0.0, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
		{0.0, 0.0, 1.0, 0.0},
		{0.0, 0.0, 0.0, 1.0},
	}

	for i, vec := range vectors {
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	if idx.Count() != 4 {
		t.Errorf("expected 4 nodes, got %d", idx.Count())
	}
}

func TestInsertDuplicate(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	vec := []float32{1.0, 0.0, 0.0, 0.0}
	err := idx.Insert(1, vec)
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	err = idx.Insert(1, vec)
	if err == nil {
		t.Error("expected duplicate error, got nil")
	}
}

func TestInsertEmptyVector(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	err := idx.Insert(1, []float32{})
	if err != ErrEmptyVector {
		t.Errorf("expected ErrEmptyVector, got %v", err)
	}
}

func TestInsertDimensionMismatch(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	err := idx.Insert(1, []float32{1.0, 0.0, 0.0, 0.0})
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	err = idx.Insert(2, []float32{1.0, 0.0, 0.0})
	if err == nil {
		t.Error("expected dimension error, got nil")
	}
}

func TestSearch(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	// Insert orthogonal vectors
	vectors := [][]float32{
		{1.0, 0.0, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
		{0.0, 0.0, 1.0, 0.0},
		{0.0, 0.0, 0.0, 1.0},
	}

	for i, vec := range vectors {
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Search for first vector
	query := []float32{1.0, 0.0, 0.0, 0.0}
	results, err := idx.Search(query, 1)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", results[0].ID)
	}

	// Distance should be 1.0 (cosine similarity)
	if results[0].Distance < 0.99 {
		t.Errorf("expected distance ~1.0, got %f", results[0].Distance)
	}
}

func TestSearchTopK(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	rng := rand.New(rand.NewSource(42))
	for i := uint64(1); i <= 100; i++ {
		vec := generateRandomVector(4, rng)
		err := idx.Insert(i, vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	query := generateRandomVector(4, rng)
	results, err := idx.Search(query, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}

	// Verify results are sorted by distance (highest first for cosine)
	for i := 1; i < len(results); i++ {
		if results[i].Distance > results[i-1].Distance {
			t.Errorf("results not sorted: %f > %f at position %d", results[i].Distance, results[i-1].Distance, i)
		}
	}
}

func TestDelete(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	vec := []float32{1.0, 0.0, 0.0, 0.0}
	err := idx.Insert(1, vec)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	err = idx.Delete(1)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if !idx.IsDeleted(1) {
		t.Error("node should be marked as deleted")
	}

	// Search should not return deleted node
	results, err := idx.Search(vec, 1)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	for _, r := range results {
		if r.ID == 1 {
			t.Error("deleted node should not appear in search results")
		}
	}
}

func TestCompact(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	for i := uint64(1); i <= 10; i++ {
		vec := []float32{float32(i), 0, 0, 0}
		floats.Normalize(vec)
		err := idx.Insert(i, vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Delete half the nodes
	for i := uint64(1); i <= 5; i++ {
		err := idx.Delete(i)
		if err != nil {
			t.Fatalf("delete %d failed: %v", i, err)
		}
	}

	if idx.DeletedCount() != 5 {
		t.Errorf("expected 5 deleted, got %d", idx.DeletedCount())
	}

	compacted := idx.Compact()
	if compacted != 5 {
		t.Errorf("expected 5 compacted, got %d", compacted)
	}

	if idx.Count() != 5 {
		t.Errorf("expected 5 nodes after compact, got %d", idx.Count())
	}

	if idx.DeletedCount() != 0 {
		t.Errorf("expected 0 deleted after compact, got %d", idx.DeletedCount())
	}
}

func TestSnapshot(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	vectors := [][]float32{
		{1.0, 0.0, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
		{0.0, 0.0, 1.0, 0.0},
	}

	for i, vec := range vectors {
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Create snapshot
	snap := idx.Snapshot()

	// Restore from snapshot
	idx2 := LoadFromSnapshot(snap, floats.DistanceCosine)

	if idx2.Count() != idx.Count() {
		t.Errorf("count mismatch: %d vs %d", idx2.Count(), idx.Count())
	}

	// Verify search works on restored index
	query := []float32{1.0, 0.0, 0.0, 0.0}
	results, err := idx2.Search(query, 1)
	if err != nil {
		t.Fatalf("search on restored index failed: %v", err)
	}

	if len(results) != 1 || results[0].ID != 1 {
		t.Errorf("unexpected search result on restored index")
	}
}

func TestKNNBruteForce(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceCosine)

	vectors := [][]float32{
		{1.0, 0.0, 0.0, 0.0},
		{0.9, 0.1, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
	}

	for i := range vectors {
		floats.Normalize(vectors[i])
	}

	for i, vec := range vectors {
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	query := []float32{1.0, 0.0, 0.0, 0.0}
	results, err := idx.KNNBruteForce(query, 2)
	if err != nil {
		t.Fatalf("brute force search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result should be ID 1 (exact match)
	if results[0].ID != 1 {
		t.Errorf("expected first result ID 1, got %d", results[0].ID)
	}
}

func TestRecall(t *testing.T) {
	// Test that HNSW recall is high compared to brute force
	config := NewConfig(16, 200)
	idx := New(config, 32, floats.DistanceCosine)

	rng := rand.New(rand.NewSource(42))
	numVectors := 1000

	for i := 0; i < numVectors; i++ {
		vec := generateRandomVector(32, rng)
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Run 10 test queries
	k := 10
	totalRecall := 0.0

	for q := 0; q < 10; q++ {
		query := generateRandomVector(32, rng)

		// Get ground truth with brute force
		bruteResults, _ := idx.KNNBruteForce(query, k)
		groundTruth := make(map[uint64]bool)
		for _, r := range bruteResults {
			groundTruth[r.ID] = true
		}

		// Get HNSW results
		hnswResults, _ := idx.SearchWithEf(query, k, 200)

		// Calculate recall
		hits := 0
		for _, r := range hnswResults {
			if groundTruth[r.ID] {
				hits++
			}
		}

		recall := float64(hits) / float64(k)
		totalRecall += recall
	}

	avgRecall := totalRecall / 10.0
	if avgRecall < 0.95 {
		t.Errorf("recall too low: %.2f%% (expected >= 95%%)", avgRecall*100)
	}
	t.Logf("Average recall: %.2f%%", avgRecall*100)
}

func TestEuclideanDistance(t *testing.T) {
	config := DefaultConfig()
	idx := New(config, 4, floats.DistanceEuclidean)

	vectors := [][]float32{
		{0.0, 0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
	}

	for i, vec := range vectors {
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Search for origin
	query := []float32{0.0, 0.0, 0.0, 0.0}
	results, err := idx.Search(query, 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// First result should be ID 1 (origin)
	if results[0].ID != 1 {
		t.Errorf("expected first result ID 1, got %d", results[0].ID)
	}

	// Distance to origin should be 0
	if results[0].Distance != 0 {
		t.Errorf("expected distance 0, got %f", results[0].Distance)
	}
}

func TestHeuristicNeighborSelection(t *testing.T) {
	config := DefaultConfig()
	config.UseHeuristic = true
	idx := New(config, 128, floats.DistanceCosine)
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 500; i++ {
		vec := generateRandomVector(128, rng)
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	query := generateRandomVector(128, rng)
	results, err := idx.Search(query, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results from heuristic search")
	}

	brute, err := idx.KNNBruteForce(query, 10)
	if err != nil {
		t.Fatalf("brute force failed: %v", err)
	}

	hits := 0
	bruteIDs := make(map[uint64]bool)
	for _, r := range brute {
		bruteIDs[r.ID] = true
	}
	for _, r := range results {
		if bruteIDs[r.ID] {
			hits++
		}
	}

	recall := float64(hits) / float64(len(brute))
	if recall < 0.7 {
		t.Errorf("heuristic search recall too low: %.2f", recall)
	}
}

func TestHeuristicDisabled(t *testing.T) {
	config := DefaultConfig()
	config.UseHeuristic = false
	idx := New(config, 128, floats.DistanceCosine)
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 200; i++ {
		vec := generateRandomVector(128, rng)
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	query := generateRandomVector(128, rng)
	results, err := idx.Search(query, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results from non-heuristic search")
	}
}

func TestNoDuplicateNeighbors(t *testing.T) {
	config := DefaultConfig()
	config.M = 8
	config.UseHeuristic = true
	idx := New(config, 4, floats.DistanceCosine)

	for i := 0; i < 50; i++ {
		vec := []float32{float32(i % 5), float32(i / 5), float32(i), float32(i * 2)}
		err := idx.Insert(uint64(i+1), vec)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	dupCount := 0
	for i := 0; i < idx.Count(); i++ {
		node := idx.nodes[uint64(i+1)]
		if node == nil {
			continue
		}
		for layer, neighbors := range node.Neighbors {
			seen := make(map[uint64]bool)
			for _, neighbor := range neighbors {
				if seen[neighbor] {
					dupCount++
					t.Errorf("duplicate neighbor %d in node %d layer %d", neighbor, node.ID, layer)
				}
				seen[neighbor] = true
			}
		}
	}

	if dupCount > 0 {
		t.Errorf("found %d duplicate neighbors across all nodes", dupCount)
	}
}

func BenchmarkInsert(b *testing.B) {
	config := DefaultConfig()
	idx := New(config, 128, floats.DistanceCosine)
	rng := rand.New(rand.NewSource(42))

	vectors := make([][]float32, b.N)
	for i := range vectors {
		vectors[i] = generateRandomVector(128, rng)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Insert(uint64(i+1), vectors[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	config := DefaultConfig()
	idx := New(config, 128, floats.DistanceCosine)
	rng := rand.New(rand.NewSource(42))

	// Insert 10000 vectors
	for i := 0; i < 10000; i++ {
		vec := generateRandomVector(128, rng)
		_ = idx.Insert(uint64(i+1), vec)
	}

	queries := make([][]float32, b.N)
	for i := range queries {
		queries[i] = generateRandomVector(128, rng)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Search(queries[i], 10)
	}
}

func BenchmarkBruteForce(b *testing.B) {
	config := DefaultConfig()
	idx := New(config, 128, floats.DistanceCosine)
	rng := rand.New(rand.NewSource(42))

	// Insert 10000 vectors
	for i := 0; i < 10000; i++ {
		vec := generateRandomVector(128, rng)
		_ = idx.Insert(uint64(i+1), vec)
	}

	queries := make([][]float32, b.N)
	for i := range queries {
		queries[i] = generateRandomVector(128, rng)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.KNNBruteForce(queries[i], 10)
	}
}
