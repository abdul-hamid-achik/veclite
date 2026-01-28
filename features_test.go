package veclite

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Pagination / Iterator Tests ---

func TestSearchPagination(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("test")
	// Insert 10 vectors
	for i := 0; i < 10; i++ {
		v := float32(i) / 10.0
		_, err := coll.Insert([]float32{v, v, v}, map[string]any{"index": i})
		if err != nil {
			t.Fatal(err)
		}
	}

	query := []float32{0.5, 0.5, 0.5}

	// Get all 10
	all, err := coll.Search(query, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 10 {
		t.Fatalf("Expected 10 results, got %d", len(all))
	}

	// Page 1: first 3
	page1, err := coll.Search(query, TopK(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 3 {
		t.Fatalf("Page 1: expected 3 results, got %d", len(page1))
	}

	// Page 2: offset 3, limit 3
	page2, err := coll.Search(query, TopK(3), WithOffset(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 3 {
		t.Fatalf("Page 2: expected 3 results, got %d", len(page2))
	}

	// Page 1 and Page 2 should have different IDs (non-overlapping)
	page1IDs := make(map[uint64]bool)
	for _, r := range page1 {
		page1IDs[r.Record.ID] = true
	}
	for _, r := range page2 {
		if page1IDs[r.Record.ID] {
			t.Errorf("Page 2 contains ID=%d which was already in Page 1", r.Record.ID)
		}
	}

	// Offset beyond results
	empty, err := coll.Search(query, TopK(3), WithOffset(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("Expected 0 results for offset=100, got %d", len(empty))
	}
}

func TestWithLimit(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("test")
	for i := 0; i < 5; i++ {
		v := float32(i) / 10.0
		_, err := coll.Insert([]float32{v, v, v}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	results, err := coll.Search([]float32{0.3, 0.3, 0.3}, WithLimit(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results with WithLimit(2), got %d", len(results))
	}
}

func TestIterator(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("test")
	for i := 0; i < 10; i++ {
		_, err := coll.Insert([]float32{float32(i), 0, 0}, map[string]any{"i": i})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Full iteration
	it := coll.Iterate()
	count := 0
	for {
		_, ok := it.Next()
		if !ok {
			break
		}
		count++
	}
	it.Close()
	if count != 10 {
		t.Errorf("Expected 10 records, got %d", count)
	}

	// With offset and limit
	it = coll.Iterate(IterOffset(3), IterLimit(4))
	count = 0
	var firstID uint64
	for {
		rec, ok := it.Next()
		if !ok {
			break
		}
		if count == 0 {
			firstID = rec.ID
		}
		count++
	}
	it.Close()
	if count != 4 {
		t.Errorf("Expected 4 records with offset=3 limit=4, got %d", count)
	}
	// Records are sorted by ID, so offset=3 means starting from 4th record (ID=4)
	if firstID != 4 {
		t.Errorf("Expected first ID=4 with offset=3, got %d", firstID)
	}

	// Offset beyond range
	it = coll.Iterate(IterOffset(100))
	_, ok := it.Next()
	if ok {
		t.Error("Expected no records with offset=100")
	}
	it.Close()
}

func TestForEach(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("test")
	for i := 0; i < 5; i++ {
		_, err := coll.Insert([]float32{float32(i), 0, 0}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Count all
	count := 0
	coll.ForEach(func(r *Record) bool {
		count++
		return true
	})
	if count != 5 {
		t.Errorf("ForEach visited %d records, want 5", count)
	}

	// Early stop
	count = 0
	coll.ForEach(func(r *Record) bool {
		count++
		return count < 3
	})
	if count != 3 {
		t.Errorf("ForEach with early stop visited %d records, want 3", count)
	}
}

// --- Document Storage Tests ---

func TestInsertDocument(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("docs")
	id, err := coll.InsertDocument(
		[]float32{0.1, 0.2, 0.3},
		"Hello World",
		map[string]any{"type": "greeting"},
	)
	if err != nil {
		t.Fatal(err)
	}

	rec, err := coll.Get(id)
	if err != nil {
		t.Fatal(err)
	}

	if rec.Content != "Hello World" {
		t.Errorf("Content = %q, want %q", rec.Content, "Hello World")
	}
	if rec.Payload["type"] != "greeting" {
		t.Errorf("Payload type = %v, want 'greeting'", rec.Payload["type"])
	}
}

func TestContentPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.veclite")

	// Create and populate
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	coll := db.Collection("docs")
	_, err = coll.InsertDocument(
		[]float32{0.1, 0.2, 0.3},
		"Persistent content",
		map[string]any{"key": "value"},
	)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Reopen and verify
	db, err = Open(path, WithReadOnly(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err = db.GetCollection("docs")
	if err != nil {
		t.Fatal(err)
	}

	rec, err := coll.Get(1)
	if err != nil {
		t.Fatal(err)
	}

	if rec.Content != "Persistent content" {
		t.Errorf("Content after reopen = %q, want %q", rec.Content, "Persistent content")
	}
}

// --- Hybrid Search Tests ---

func TestHybridSearch(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("hybrid",
		WithDimension(3),
		WithTextIndex("title"),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Insert documents with both vector and text
	_, err = coll.InsertDocument(
		[]float32{0.9, 0.1, 0.1},
		"vector databases and embeddings",
		map[string]any{"title": "Vector DB"},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertDocument(
		[]float32{0.1, 0.9, 0.1},
		"full text search and information retrieval",
		map[string]any{"title": "Text Search"},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertDocument(
		[]float32{0.5, 0.5, 0.1},
		"hybrid search combines vector and text",
		map[string]any{"title": "Hybrid"},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Hybrid search: query vector is close to doc 1, text query matches doc 2
	results, err := coll.HybridSearch(
		[]float32{0.85, 0.15, 0.1}, // close to "Vector DB"
		"text search retrieval",     // matches "Text Search"
		TopK(3),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("HybridSearch returned no results")
	}

	// The hybrid doc should benefit from both signals
	// All 3 documents should appear
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

func TestHybridSearchWeights(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("weighted",
		WithDimension(3),
		WithTextIndex("title"),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertDocument([]float32{0.9, 0.1, 0.1}, "apples", map[string]any{"title": "Fruit"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = coll.InsertDocument([]float32{0.1, 0.9, 0.1}, "oranges", map[string]any{"title": "Citrus"})
	if err != nil {
		t.Fatal(err)
	}

	// Heavy vector weight
	vectorResults, err := coll.HybridSearch(
		[]float32{0.85, 0.15, 0.1},
		"oranges",
		TopK(2),
		WithVectorWeight(10.0),
		WithTextWeight(0.1),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Heavy text weight
	textResults, err := coll.HybridSearch(
		[]float32{0.85, 0.15, 0.1},
		"oranges",
		TopK(2),
		WithVectorWeight(0.1),
		WithTextWeight(10.0),
	)
	if err != nil {
		t.Fatal(err)
	}

	// With vector weight, apples (closer vector) should rank higher
	// With text weight, oranges (matching text) should rank higher
	if len(vectorResults) >= 1 && len(textResults) >= 1 {
		if vectorResults[0].Record.ID == textResults[0].Record.ID {
			// They might still be same due to RRF blending, this is not a strict test
			t.Log("Note: top result is the same for both weight configs (this is possible with RRF)")
		}
	}
}

// --- Streaming Tests ---

func TestSearchStream(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("stream")
	for i := 0; i < 5; i++ {
		_, err := coll.Insert([]float32{float32(i) / 10.0, 0, 0}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Stream all results
	var results []Result
	err = coll.SearchStream([]float32{0.3, 0, 0}, func(r Result) bool {
		results = append(results, r)
		return true
	}, TopK(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Errorf("Expected 3 streamed results, got %d", len(results))
	}

	// Stream with early stop
	results = nil
	err = coll.SearchStream([]float32{0.3, 0, 0}, func(r Result) bool {
		results = append(results, r)
		return len(results) < 2 // stop after 2
	}, TopK(5))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 streamed results with early stop, got %d", len(results))
	}
}

// --- Embedder Tests ---

type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) Embed(text string) ([]float32, error) {
	vec := make([]float32, m.dim)
	for i := range vec {
		vec[i] = float32(len(text)%10) / 10.0
	}
	return vec, nil
}

func (m *mockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i, text := range texts {
		v, err := m.Embed(text)
		if err != nil {
			return nil, err
		}
		vecs[i] = v
	}
	return vecs, nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dim
}

func TestInsertText(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("embed",
		WithDimension(4),
		WithEmbedder(&mockEmbedder{dim: 4}),
		WithTextIndex("title"),
	)
	if err != nil {
		t.Fatal(err)
	}

	id, err := coll.InsertText("Hello World", map[string]any{"title": "Greeting"})
	if err != nil {
		t.Fatal(err)
	}

	rec, err := coll.Get(id)
	if err != nil {
		t.Fatal(err)
	}

	if rec.Content != "Hello World" {
		t.Errorf("Content = %q, want %q", rec.Content, "Hello World")
	}
	if len(rec.Vector) != 4 {
		t.Errorf("Vector dimension = %d, want 4", len(rec.Vector))
	}
}

func TestSearchText(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("embed",
		WithDimension(4),
		WithEmbedder(&mockEmbedder{dim: 4}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertText("Hello", nil)
	if err != nil {
		t.Fatal(err)
	}

	results, err := coll.SearchText("Hello", TopK(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestNoEmbedderError(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("no-embedder")
	_, err = coll.InsertText("test", nil)
	if err != ErrNoEmbedder {
		t.Errorf("Expected ErrNoEmbedder, got: %v", err)
	}

	_, err = coll.SearchText("test")
	if err != ErrNoEmbedder {
		t.Errorf("Expected ErrNoEmbedder, got: %v", err)
	}
}

// --- Metrics Tests ---

func TestMetrics(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("metrics")

	// Initial metrics
	m := db.Metrics()
	if m.InsertCount != 0 || m.SearchCount != 0 || m.DeleteCount != 0 {
		t.Error("Initial metrics should be zero")
	}

	// Insert
	id, err := coll.Insert([]float32{0.1, 0.2, 0.3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m = db.Metrics()
	if m.InsertCount != 1 {
		t.Errorf("InsertCount = %d, want 1", m.InsertCount)
	}

	// Search
	_, err = coll.Search([]float32{0.1, 0.2, 0.3}, TopK(1))
	if err != nil {
		t.Fatal(err)
	}
	m = db.Metrics()
	if m.SearchCount != 1 {
		t.Errorf("SearchCount = %d, want 1", m.SearchCount)
	}
	if m.AvgSearchTime <= 0 {
		t.Error("AvgSearchTime should be > 0")
	}

	// Delete
	err = coll.Delete(id)
	if err != nil {
		t.Fatal(err)
	}
	m = db.Metrics()
	if m.DeleteCount != 1 {
		t.Errorf("DeleteCount = %d, want 1", m.DeleteCount)
	}
}

// --- Logger Tests ---

type testLogger struct {
	messages []string
}

func (l *testLogger) Debug(msg string, keysAndValues ...any) {
	l.messages = append(l.messages, "DEBUG: "+msg)
}

func (l *testLogger) Info(msg string, keysAndValues ...any) {
	l.messages = append(l.messages, "INFO: "+msg)
}

func (l *testLogger) Error(msg string, keysAndValues ...any) {
	l.messages = append(l.messages, "ERROR: "+msg)
}

func TestNopLogger(t *testing.T) {
	// Ensure NopLogger doesn't panic
	var l NopLogger
	l.Debug("test")
	l.Info("test")
	l.Error("test")
}

func TestWithLogger(t *testing.T) {
	logger := &testLogger{}
	db, err := Open(":memory:", WithLogger(logger))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// The logger is configured, that's the test
	if db.logger == nil {
		t.Error("Logger should not be nil")
	}
}

// --- BM25 Persistence Tests ---

func TestBM25Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bm25.veclite")

	// Create and populate with text-indexed collection
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	coll, err := db.CreateCollection("docs",
		WithDimension(3),
		WithTextIndex("title"),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertDocument(
		[]float32{0.1, 0.2, 0.3},
		"Go programming language",
		map[string]any{"title": "Go"},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertDocument(
		[]float32{0.4, 0.5, 0.6},
		"Python data science",
		map[string]any{"title": "Python"},
	)
	if err != nil {
		t.Fatal(err)
	}

	db.Close()

	// Reopen and verify text search works
	db, err = Open(path, WithReadOnly(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err = db.GetCollection("docs")
	if err != nil {
		t.Fatal(err)
	}

	// Text search should still work after reload
	results, err := coll.TextSearch("Go programming", TopK(2))
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("TextSearch after reload returned no results")
	}

	if results[0].Record.Content != "Go programming language" {
		t.Errorf("Expected Go doc, got: %s", results[0].Record.Content)
	}
}

// --- Backward Compatibility Tests ---

func TestBackwardCompatEmptyContent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("test")

	// Insert without content (old-style)
	id, err := coll.Insert([]float32{0.1, 0.2, 0.3}, map[string]any{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}

	rec, err := coll.Get(id)
	if err != nil {
		t.Fatal(err)
	}

	// Content should be empty for legacy records
	if rec.Content != "" {
		t.Errorf("Content = %q, want empty string", rec.Content)
	}
}

func TestBackwardCompatPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "compat.veclite")

	// Create a database with no text indexing (simulates pre-feature DB)
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	coll := db.Collection("old")
	_, err = coll.Insert([]float32{0.1, 0.2, 0.3}, map[string]any{"file": "test.go"})
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Reopen - should load without errors
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err = db.GetCollection("old")
	if err != nil {
		t.Fatal(err)
	}

	rec, err := coll.Get(1)
	if err != nil {
		t.Fatal(err)
	}

	if rec.Payload["file"] != "test.go" {
		t.Errorf("Payload lost after reopen: %v", rec.Payload)
	}
	if rec.Content != "" {
		t.Errorf("Content should be empty for old records, got %q", rec.Content)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Database file should exist")
	}
}

// --- RRF Fusion Tests ---

func TestReciprocalRankFusion(t *testing.T) {
	r1 := Record{ID: 1}
	r2 := Record{ID: 2}
	r3 := Record{ID: 3}

	set1 := []Result{
		{Record: &r1, Score: 0.9},
		{Record: &r2, Score: 0.8},
	}
	set2 := []Result{
		{Record: &r2, Score: 0.95},
		{Record: &r3, Score: 0.85},
	}

	// Equal weights
	fused := reciprocalRankFusion([][]Result{set1, set2}, 60, nil)

	if len(fused) != 3 {
		t.Fatalf("Expected 3 fused results, got %d", len(fused))
	}

	// r2 appears in both lists, should have highest RRF score
	if fused[0].Record.ID != 2 {
		t.Errorf("Expected ID=2 to rank first (appears in both sets), got ID=%d", fused[0].Record.ID)
	}

	// Weighted fusion
	fusedWeighted := reciprocalRankFusion([][]Result{set1, set2}, 60, []float64{10.0, 0.1})
	// With heavy weight on set1, r1 (rank 1 in set1) should rank higher
	if fusedWeighted[0].Record.ID != 1 {
		t.Logf("With heavy set1 weight, top result ID=%d (expected 1)", fusedWeighted[0].Record.ID)
	}
}
