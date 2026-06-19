package veclite

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

func TestMemoryStorage(t *testing.T) {
	store := storage.NewMemory()

	// Initially empty
	snapshot, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if snapshot != nil {
		t.Error("Initial load should return nil")
	}

	// Save some data
	data := NewDatabaseSnapshot()
	data.Collections["test"] = NewCollectionSnapshot("test", 3, floats.DistanceCosine)

	if err := store.Save(data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load it back
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil after save")
	}
	if _, ok := loaded.Collections["test"]; !ok {
		t.Error("Collection not found after save/load")
	}

	// Close is a no-op
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestFileStorage(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.veclite")

	store := storage.NewFile(path)

	// Initially file doesn't exist
	snapshot, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if snapshot != nil {
		t.Error("Initial load should return nil for non-existent file")
	}

	// Save some data
	data := NewDatabaseSnapshot()
	data.Collections["embeddings"] = &CollectionSnapshot{
		Name:         "embeddings",
		Dimension:    384,
		DistanceType: floats.DistanceCosine,
		NextID:       3,
		Records: []*RecordSnapshot{
			{
				ID:        1,
				Vector:    []float32{1, 2, 3},
				Payload:   map[string]any{"file": "main.go"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        2,
				Vector:    []float32{4, 5, 6},
				Payload:   map[string]any{"file": "util.go"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.Save(data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if !store.Exists() {
		t.Error("File should exist after save")
	}

	// Load it back
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil after save")
	}

	coll, ok := loaded.Collections["embeddings"]
	if !ok {
		t.Fatal("Collection not found after save/load")
	}

	if coll.Dimension != 384 {
		t.Errorf("Dimension = %v, want 384", coll.Dimension)
	}

	if len(coll.Records) != 2 {
		t.Errorf("Records count = %v, want 2", len(coll.Records))
	}

	if coll.Records[0].Payload["file"] != "main.go" {
		t.Errorf("Record payload file = %v, want main.go", coll.Records[0].Payload["file"])
	}

	// Close
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestFileStorageMetadataAndTextOnlyRecord(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.veclite")

	store := storage.NewFile(path)

	data := NewDatabaseSnapshot()
	data.Metadata = map[string]any{
		"app":     "vecgrep",
		"version": int64(1),
		"enabled": true,
		"tags":    []any{"semantic", "video"},
		"weights": []float64{0.7, 0.3},
		"profile": map[string]any{
			"provider":   "local",
			"dimensions": 3,
		},
	}

	coll := NewCollectionSnapshot("evidence", 0, floats.DistanceCosine)
	coll.Metadata = map[string]any{
		"embedding_profile": map[string]any{
			"model":      "keyword-only",
			"dimensions": int32(0),
		},
	}
	coll.Records = append(coll.Records, &RecordSnapshot{
		ID:      1,
		Payload: map[string]any{"path": "video.mp4", "lines": []int{10, 11}},
		Content: "checkout failed in transcript",
	})
	data.Collections["evidence"] = coll

	if err := store.Save(data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Metadata["app"] != "vecgrep" {
		t.Fatalf("loaded app metadata = %v, want vecgrep", loaded.Metadata["app"])
	}
	if loaded.Metadata["enabled"] != true {
		t.Fatalf("loaded enabled metadata = %v, want true", loaded.Metadata["enabled"])
	}
	if tags := loaded.Metadata["tags"].([]any); len(tags) != 2 || tags[0] != "semantic" || tags[1] != "video" {
		t.Fatalf("loaded tags metadata = %#v, want [semantic video]", tags)
	}
	if profile := loaded.Metadata["profile"].(map[string]any); profile["dimensions"] != 3 {
		t.Fatalf("loaded profile metadata = %#v, want dimensions=3", profile)
	}

	loadedColl := loaded.Collections["evidence"]
	if loadedColl == nil {
		t.Fatal("loaded collection is nil")
	}
	if model := loadedColl.Metadata["embedding_profile"].(map[string]any)["model"]; model != "keyword-only" {
		t.Fatalf("loaded collection profile model = %v, want keyword-only", model)
	}
	if len(loadedColl.Records) != 1 {
		t.Fatalf("loaded records = %d, want 1", len(loadedColl.Records))
	}
	record := loadedColl.Records[0]
	if len(record.Vector) != 0 {
		t.Fatalf("loaded text-only record vector len = %d, want 0", len(record.Vector))
	}
	if record.Content != "checkout failed in transcript" {
		t.Fatalf("loaded text-only content = %q", record.Content)
	}
	if lines := record.Payload["lines"].([]int); len(lines) != 2 || lines[0] != 10 || lines[1] != 11 {
		t.Fatalf("loaded record lines = %#v, want [10 11]", lines)
	}
}

func TestFileStorageAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.veclite")

	store := storage.NewFile(path)

	// First save
	data1 := NewDatabaseSnapshot()
	data1.Collections["v1"] = NewCollectionSnapshot("v1", 3, floats.DistanceCosine)
	if err := store.Save(data1); err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// Second save (overwrites)
	data2 := NewDatabaseSnapshot()
	data2.Collections["v2"] = NewCollectionSnapshot("v2", 3, floats.DistanceCosine)
	if err := store.Save(data2); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Load should get v2
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if _, ok := loaded.Collections["v2"]; !ok {
		t.Error("Should have v2 collection after overwrite")
	}
	if _, ok := loaded.Collections["v1"]; ok {
		t.Error("Should not have v1 collection after overwrite")
	}

	// Backup should have been cleaned up
	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Error("Backup file should be cleaned up")
	}
}

func TestFileStorageNestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "dir", "test.veclite")

	store := storage.NewFile(path)

	data := NewDatabaseSnapshot()
	if err := store.Save(data); err != nil {
		t.Fatalf("Save to nested path failed: %v", err)
	}

	if !store.Exists() {
		t.Error("File should exist in nested directory")
	}
}

func TestFileStorageDelete(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.veclite")

	store := storage.NewFile(path)
	if err := store.Save(NewDatabaseSnapshot()); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !store.Exists() {
		t.Fatal("File should exist before delete")
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if store.Exists() {
		t.Error("File should not exist after delete")
	}
}

func TestFileStorageCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "corrupted.veclite")

	// Write garbage data
	if err := os.WriteFile(path, []byte("not a veclite file"), 0644); err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}

	store := storage.NewFile(path)
	_, err := store.Load()
	if err == nil {
		t.Error("Load should fail on corrupted file")
	}
}

func TestFileStorageInvalidMagic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.veclite")

	// Write file with wrong magic but correct size
	data := make([]byte, 64)
	copy(data[:8], "WRONGMAG")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	store := storage.NewFile(path)
	_, err := store.Load()
	if err == nil {
		t.Error("Load should fail on invalid magic")
	}
}

func TestNewDatabaseSnapshot(t *testing.T) {
	snapshot := NewDatabaseSnapshot()

	if snapshot.Version != 3 {
		t.Errorf("Version = %v, want 3", snapshot.Version)
	}

	if snapshot.Collections == nil {
		t.Error("Collections should be initialized")
	}

	if snapshot.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestNewCollectionSnapshot(t *testing.T) {
	snapshot := NewCollectionSnapshot("test", 128, floats.DistanceDot)

	if snapshot.Name != "test" {
		t.Errorf("Name = %v, want test", snapshot.Name)
	}

	if snapshot.Dimension != 128 {
		t.Errorf("Dimension = %v, want 128", snapshot.Dimension)
	}

	if snapshot.DistanceType != floats.DistanceDot {
		t.Errorf("DistanceType = %v, want dot", snapshot.DistanceType)
	}

	if snapshot.NextID != 1 {
		t.Errorf("NextID = %v, want 1", snapshot.NextID)
	}

	if snapshot.Records == nil {
		t.Error("Records should be initialized")
	}
}

func TestFileStoragePath(t *testing.T) {
	store := storage.NewFile("/path/to/db.veclite")

	if store.Path() != "/path/to/db.veclite" {
		t.Errorf("Path() = %v, want /path/to/db.veclite", store.Path())
	}
}

func BenchmarkFileStorageSave(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "bench.veclite")

	store := storage.NewFile(path)

	// Create snapshot with some data
	snapshot := NewDatabaseSnapshot()
	coll := NewCollectionSnapshot("test", 384, floats.DistanceCosine)
	for i := 0; i < 1000; i++ {
		vector := make([]float32, 384)
		for j := range vector {
			vector[j] = float32(i*384+j) / (1000 * 384)
		}
		coll.Records = append(coll.Records, &RecordSnapshot{
			ID:        uint64(i + 1),
			Vector:    vector,
			Payload:   map[string]any{"index": i},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}
	snapshot.Collections["test"] = coll

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Save(snapshot)
	}
}

func BenchmarkFileStorageLoad(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "bench.veclite")

	store := storage.NewFile(path)

	// Create and save snapshot with some data
	snapshot := NewDatabaseSnapshot()
	coll := NewCollectionSnapshot("test", 384, floats.DistanceCosine)
	for i := 0; i < 1000; i++ {
		vector := make([]float32, 384)
		for j := range vector {
			vector[j] = float32(i*384+j) / (1000 * 384)
		}
		coll.Records = append(coll.Records, &RecordSnapshot{
			ID:        uint64(i + 1),
			Vector:    vector,
			Payload:   map[string]any{"index": i},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}
	snapshot.Collections["test"] = coll
	if err := store.Save(snapshot); err != nil {
		b.Fatalf("Save failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Load()
	}
}
