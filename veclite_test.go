package veclite

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

func TestOpenMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	defer db.Close()

	if db.Path() != ":memory:" {
		t.Errorf("Path() = %v, want :memory:", db.Path())
	}
}

func TestOpenFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.veclite")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%v) failed: %v", path, err)
	}

	// Insert some data
	coll := db.Collection("test")
	_, err = coll.Insert([]float32{1, 2, 3}, map[string]any{"key": "value"})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Reopen and verify data
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer db2.Close()

	coll2, err := db2.GetCollection("test")
	if err != nil {
		t.Fatalf("GetCollection failed: %v", err)
	}

	if coll2.Count() != 1 {
		t.Errorf("Count() = %v, want 1", coll2.Count())
	}
}

func TestCollectionGetOrCreate(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	// First call creates
	coll1 := db.Collection("test")
	if coll1 == nil {
		t.Fatal("Collection() returned nil")
	}

	// Second call returns same collection
	coll2 := db.Collection("test")
	if coll1 != coll2 {
		t.Error("Collection() returned different instance")
	}
}

func TestCreateCollection(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, err := db.CreateCollection("test", WithDimension(128))
	if err != nil {
		t.Fatalf("CreateCollection failed: %v", err)
	}

	// Dimension should be set
	if coll.Dimension() != 128 {
		t.Errorf("Dimension() = %v, want 128", coll.Dimension())
	}

	// Creating again should fail
	_, err = db.CreateCollection("test")
	if !errors.Is(err, ErrCollectionExists) {
		t.Errorf("CreateCollection on existing = %v, want ErrCollectionExists", err)
	}
}

func TestDropCollection(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	db.Collection("test")
	if !db.HasCollection("test") {
		t.Error("Collection was not created")
	}

	if err := db.DropCollection("test"); err != nil {
		t.Fatalf("DropCollection failed: %v", err)
	}

	if db.HasCollection("test") {
		t.Error("Collection was not dropped")
	}

	// Dropping non-existent should fail
	err := db.DropCollection("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("DropCollection on non-existent = %v, want ErrNotFound", err)
	}
}

func TestInsertAndGet(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	vector := []float32{1, 2, 3, 4, 5}
	payload := map[string]any{"file": "main.go", "line": 42}

	id, err := coll.Insert(vector, payload)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if id != 1 {
		t.Errorf("First ID = %v, want 1", id)
	}

	// Get the record
	record, err := coll.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if record.ID != id {
		t.Errorf("Record.ID = %v, want %v", record.ID, id)
	}

	if len(record.Vector) != len(vector) {
		t.Errorf("Vector length = %v, want %v", len(record.Vector), len(vector))
	}

	if record.Payload["file"] != "main.go" {
		t.Errorf("Payload[file] = %v, want main.go", record.Payload["file"])
	}
}

func TestInsertDimensionLocking(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	// First insert sets dimension
	_, err := coll.Insert([]float32{1, 2, 3}, nil)
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	if coll.Dimension() != 3 {
		t.Errorf("Dimension() = %v, want 3", coll.Dimension())
	}

	// Same dimension should work
	_, err = coll.Insert([]float32{4, 5, 6}, nil)
	if err != nil {
		t.Fatalf("Same dimension insert failed: %v", err)
	}

	// Different dimension should fail
	_, err = coll.Insert([]float32{1, 2}, nil)
	if !errors.Is(err, ErrDimensionMismatch) {
		t.Errorf("Wrong dimension insert = %v, want ErrDimensionMismatch", err)
	}
}

func TestInsertEmptyVector(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	_, err := coll.Insert([]float32{}, nil)
	if !errors.Is(err, ErrEmptyVector) {
		t.Errorf("Empty vector insert = %v, want ErrEmptyVector", err)
	}
}

func TestInsertBatch(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	vectors := [][]float32{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
	}
	payloads := []map[string]any{
		{"index": 0},
		{"index": 1},
		{"index": 2},
	}

	ids, err := coll.InsertBatch(vectors, payloads)
	if err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("InsertBatch returned %v IDs, want 3", len(ids))
	}

	if coll.Count() != 3 {
		t.Errorf("Count() = %v, want 3", coll.Count())
	}
}

func TestDelete(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	id, _ := coll.Insert([]float32{1, 2, 3}, nil)

	if err := coll.Delete(id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if coll.Count() != 0 {
		t.Errorf("Count() after delete = %v, want 0", coll.Count())
	}

	// Deleting again should fail
	err := coll.Delete(id)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete non-existent = %v, want ErrNotFound", err)
	}
}

func TestDeleteWhere(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	coll.Insert([]float32{1, 2, 3}, map[string]any{"lang": "go"})
	coll.Insert([]float32{4, 5, 6}, map[string]any{"lang": "go"})
	coll.Insert([]float32{7, 8, 9}, map[string]any{"lang": "python"})

	deleted, err := coll.DeleteWhere(Equal("lang", "go"))
	if err != nil {
		t.Fatalf("DeleteWhere failed: %v", err)
	}

	if deleted != 2 {
		t.Errorf("DeleteWhere returned %v, want 2", deleted)
	}

	if coll.Count() != 1 {
		t.Errorf("Count() after DeleteWhere = %v, want 1", coll.Count())
	}
}

func TestSearch(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	// Insert vectors
	coll.Insert([]float32{1, 0, 0}, map[string]any{"name": "x"})
	coll.Insert([]float32{0, 1, 0}, map[string]any{"name": "y"})
	coll.Insert([]float32{0, 0, 1}, map[string]any{"name": "z"})
	coll.Insert([]float32{1, 1, 0}, map[string]any{"name": "xy"})

	// Search for vector similar to x-axis
	results, err := coll.Search([]float32{1, 0, 0}, TopK(2))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Search returned %v results, want 2", len(results))
	}

	// First result should be exact match
	if results[0].Record.Payload["name"] != "x" {
		t.Errorf("First result = %v, want x", results[0].Record.Payload["name"])
	}

	if results[0].Score != 1.0 {
		t.Errorf("First result score = %v, want 1.0", results[0].Score)
	}
}

func TestSearchWithFilter(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	coll.Insert([]float32{1, 0, 0}, map[string]any{"lang": "go"})
	coll.Insert([]float32{0.9, 0.1, 0}, map[string]any{"lang": "go"})
	coll.Insert([]float32{0.95, 0.05, 0}, map[string]any{"lang": "python"})

	// Search only Go files
	results, err := coll.Search([]float32{1, 0, 0}, TopK(10), WithFilter(Equal("lang", "go")))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Search with filter returned %v results, want 2", len(results))
	}

	for _, r := range results {
		if r.Record.Payload["lang"] != "go" {
			t.Errorf("Result has lang=%v, want go", r.Record.Payload["lang"])
		}
	}
}

func TestSearchWithThreshold(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	coll.Insert([]float32{1, 0, 0}, nil)
	coll.Insert([]float32{0, 1, 0}, nil) // orthogonal, score = 0

	results, err := coll.Search([]float32{1, 0, 0}, TopK(10), Threshold(0.5))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Search with threshold returned %v results, want 1", len(results))
	}
}

func TestFind(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	coll.Insert([]float32{1, 2, 3}, map[string]any{"type": "function", "file": "main.go"})
	coll.Insert([]float32{4, 5, 6}, map[string]any{"type": "class", "file": "main.go"})
	coll.Insert([]float32{7, 8, 9}, map[string]any{"type": "function", "file": "util.go"})

	// Find all functions
	results, err := coll.Find(Equal("type", "function"))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Find returned %v results, want 2", len(results))
	}
}

func TestFindOne(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	coll.Insert([]float32{1, 2, 3}, map[string]any{"file": "main.go"})
	coll.Insert([]float32{4, 5, 6}, map[string]any{"file": "util.go"})

	record, err := coll.FindOne(Equal("file", "main.go"))
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}

	if record.Payload["file"] != "main.go" {
		t.Errorf("FindOne returned file=%v, want main.go", record.Payload["file"])
	}

	// Find non-existent
	_, err = coll.FindOne(Equal("file", "nonexistent.go"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindOne non-existent = %v, want ErrNotFound", err)
	}
}

func TestDistanceTypes(t *testing.T) {
	tests := []struct {
		name         string
		distanceType floats.DistanceType
	}{
		{"cosine", floats.DistanceCosine},
		{"dot", floats.DistanceDot},
		{"euclidean", floats.DistanceEuclidean},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _ := Open(":memory:")
			defer db.Close()

			coll, _ := db.CreateCollection("test", WithDistanceType(tt.distanceType))
			coll.Insert([]float32{1, 0, 0}, nil)
			coll.Insert([]float32{0.5, 0.5, 0}, nil)

			results, err := coll.Search([]float32{1, 0, 0}, TopK(2))
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) != 2 {
				t.Errorf("Search returned %v results, want 2", len(results))
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	// Insert initial data
	for i := 0; i < 100; i++ {
		coll.Insert([]float32{float32(i), float32(i + 1), float32(i + 2)}, nil)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := coll.Search([]float32{50, 51, 52}, TopK(5))
				if err != nil {
					errors <- err
				}
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := coll.Insert([]float32{float32(idx*10 + j), 0, 0}, nil)
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.veclite")

	// Create and populate database
	db, _ := Open(path)
	coll := db.Collection("embeddings")

	const numRecords = 1000
	for i := 0; i < numRecords; i++ {
		coll.Insert(
			[]float32{float32(i), float32(i + 1), float32(i + 2)},
			map[string]any{"index": i},
		)
	}

	db.Close()

	// Reopen and verify
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer db2.Close()

	coll2, err := db2.GetCollection("embeddings")
	if err != nil {
		t.Fatalf("GetCollection failed: %v", err)
	}

	if coll2.Count() != numRecords {
		t.Errorf("Count() = %v, want %v", coll2.Count(), numRecords)
	}

	// Verify search still works
	results, err := coll2.Search([]float32{500, 501, 502}, TopK(5))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("Search returned %v results, want 5", len(results))
	}
}

func TestDatabaseStats(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll1 := db.Collection("coll1")
	coll1.Insert([]float32{1, 2, 3}, nil)
	coll1.Insert([]float32{4, 5, 6}, nil)

	coll2 := db.Collection("coll2")
	coll2.Insert([]float32{1, 2, 3, 4}, nil)

	stats := db.Stats()

	if stats.Collections != 2 {
		t.Errorf("Collections = %v, want 2", stats.Collections)
	}

	if stats.TotalRecords != 3 {
		t.Errorf("TotalRecords = %v, want 3", stats.TotalRecords)
	}
}

func TestCollectionClear(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	coll.Insert([]float32{1, 2, 3}, nil)
	coll.Insert([]float32{4, 5, 6}, nil)

	coll.Clear()

	if coll.Count() != 0 {
		t.Errorf("Count() after Clear = %v, want 0", coll.Count())
	}

	// Should still preserve dimension
	_, err := coll.Insert([]float32{1, 2, 3}, nil)
	if err != nil {
		t.Errorf("Insert after Clear failed: %v", err)
	}
}

func TestRecordClone(t *testing.T) {
	record := &Record{
		ID:      1,
		Vector:  []float32{1, 2, 3},
		Payload: map[string]any{"key": "value"},
	}

	clone := record.Clone()

	// Modify original
	record.Vector[0] = 100
	record.Payload["key"] = "modified"

	// Clone should be unchanged
	if clone.Vector[0] != 1 {
		t.Error("Clone vector was modified")
	}
	if clone.Payload["key"] != "value" {
		t.Error("Clone payload was modified")
	}
}

// Benchmarks

func BenchmarkInsert(b *testing.B) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	vector := make([]float32, 384)
	for i := range vector {
		vector[i] = float32(i) / 384
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coll.Insert(vector, nil)
	}
}

func BenchmarkInsertBatch(b *testing.B) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	vectors := make([][]float32, 100)
	for i := range vectors {
		vectors[i] = make([]float32, 384)
		for j := range vectors[i] {
			vectors[i][j] = float32(i*384+j) / (100 * 384)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coll.InsertBatch(vectors, nil)
	}
}

func BenchmarkSearch10K(b *testing.B) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")

	// Insert 10K vectors
	for i := 0; i < 10000; i++ {
		vector := make([]float32, 384)
		for j := range vector {
			vector[j] = float32(i*384+j) / (10000 * 384)
		}
		coll.Insert(vector, nil)
	}

	query := make([]float32, 384)
	for i := range query {
		query[i] = 0.5
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coll.Search(query, TopK(10))
	}
}
