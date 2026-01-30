package openclaw

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockEmbedder is a simple embedder for testing.
type mockEmbedder struct {
	dimension int
}

func (e *mockEmbedder) Embed(text string) ([]float32, error) {
	// Return a simple deterministic vector based on text length
	vec := make([]float32, e.dimension)
	for i := range vec {
		vec[i] = float32(len(text)%10) / 10.0
	}
	return vec, nil
}

func (e *mockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := e.Embed(text)
		if err != nil {
			return nil, err
		}
		result[i] = vec
	}
	return result, nil
}

func (e *mockEmbedder) Dimension() int {
	return e.dimension
}

func TestMemoryBasic(t *testing.T) {
	mem, err := New(Config{
		DBPath:   ":memory:",
		Embedder: &mockEmbedder{dimension: 128},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer mem.Close()

	t.Run("remember and recall", func(t *testing.T) {
		id, err := mem.Remember("Test memory content", RememberOptions{
			Importance: 0.8,
			Tags:       []string{"test", "important"},
		})
		if err != nil {
			t.Fatalf("Remember failed: %v", err)
		}
		if id == 0 {
			t.Error("expected non-zero ID")
		}

		// Recall by similar content
		entries, err := mem.Recall("test content", RecallOptions{Limit: 10})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]
		if entry.ID != id {
			t.Errorf("expected ID %d, got %d", id, entry.ID)
		}
		if entry.Content != "Test memory content" {
			t.Errorf("unexpected content: %q", entry.Content)
		}
		if entry.Importance != 0.8 {
			t.Errorf("expected importance 0.8, got %v", entry.Importance)
		}
		if len(entry.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(entry.Tags))
		}
	})

	t.Run("remember with TTL", func(t *testing.T) {
		id, err := mem.Remember("Temporary memory", RememberOptions{
			TTL: time.Hour,
		})
		if err != nil {
			t.Fatalf("Remember failed: %v", err)
		}

		entries, err := mem.Recall("temporary", RecallOptions{Limit: 10})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}

		var found *MemoryEntry
		for i := range entries {
			if entries[i].ID == id {
				found = &entries[i]
				break
			}
		}

		if found == nil {
			t.Fatal("could not find memory with TTL")
		}
		if found.ExpiresAt.IsZero() {
			t.Error("expected ExpiresAt to be set")
		}
	})

	t.Run("remember with metadata", func(t *testing.T) {
		id, err := mem.Remember("Memory with metadata", RememberOptions{
			Metadata: map[string]any{
				"source":  "test",
				"version": 1,
			},
		})
		if err != nil {
			t.Fatalf("Remember failed: %v", err)
		}

		entries, err := mem.Recall("metadata", RecallOptions{Limit: 10})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}

		var found *MemoryEntry
		for i := range entries {
			if entries[i].ID == id {
				found = &entries[i]
				break
			}
		}

		if found == nil {
			t.Fatal("could not find memory with metadata")
		}
		if found.Metadata["source"] != "test" {
			t.Errorf("unexpected source: %v", found.Metadata["source"])
		}
	})
}

func TestMemoryFilters(t *testing.T) {
	mem, err := New(Config{
		DBPath:   ":memory:",
		Embedder: &mockEmbedder{dimension: 128},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer mem.Close()

	// Create memories with different properties
	_, _ = mem.Remember("High importance memory", RememberOptions{Importance: 0.9, Tags: []string{"important"}})
	_, _ = mem.Remember("Low importance memory", RememberOptions{Importance: 0.1, Tags: []string{"trivial"}})
	_, _ = mem.Remember("Tagged memory", RememberOptions{Tags: []string{"specific"}})

	t.Run("filter by min importance", func(t *testing.T) {
		entries, err := mem.Recall("memory", RecallOptions{
			MinImportance: 0.5,
		})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}

		for _, e := range entries {
			if e.Importance < 0.5 {
				t.Errorf("found entry with importance %v below threshold", e.Importance)
			}
		}
	})

	t.Run("filter by tags", func(t *testing.T) {
		entries, err := mem.Recall("memory", RecallOptions{
			Tags: []string{"specific"},
		})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}

		if len(entries) != 1 {
			t.Errorf("expected 1 entry with 'specific' tag, got %d", len(entries))
		}
	})
}

func TestMemoryForget(t *testing.T) {
	mem, err := New(Config{
		DBPath:   ":memory:",
		Embedder: &mockEmbedder{dimension: 128},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer mem.Close()

	// Create some memories
	_, _ = mem.Remember("To forget by tag", RememberOptions{Tags: []string{"delete-me"}})
	_, _ = mem.Remember("To keep", RememberOptions{Tags: []string{"keep"}})
	_, _ = mem.Remember("Low importance to forget", RememberOptions{Importance: 0.1})

	t.Run("forget by tags", func(t *testing.T) {
		deleted, err := mem.Forget(ForgetOptions{
			Tags: []string{"delete-me"},
		})
		if err != nil {
			t.Fatalf("Forget failed: %v", err)
		}
		if deleted != 1 {
			t.Errorf("expected 1 deleted, got %d", deleted)
		}
	})

	t.Run("forget by importance", func(t *testing.T) {
		deleted, err := mem.Forget(ForgetOptions{
			BelowImportance: 0.3,
		})
		if err != nil {
			t.Fatalf("Forget failed: %v", err)
		}
		if deleted != 1 {
			t.Errorf("expected 1 deleted, got %d", deleted)
		}
	})

	t.Run("forget requires criteria", func(t *testing.T) {
		_, err := mem.Forget(ForgetOptions{})
		if err == nil {
			t.Error("expected error for empty criteria")
		}
	})
}

func TestMemoryExportMarkdown(t *testing.T) {
	mem, err := New(Config{
		DBPath:   ":memory:",
		Embedder: &mockEmbedder{dimension: 128},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer mem.Close()

	// Create some memories
	_, _ = mem.Remember("First memory", RememberOptions{
		Importance: 0.8,
		Tags:       []string{"export", "test"},
	})
	_, _ = mem.Remember("Second memory", RememberOptions{
		TTL: time.Hour,
	})

	// Export to temp directory
	outputDir := filepath.Join(t.TempDir(), "export")
	err = mem.ExportMarkdown(outputDir)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	// Check files were created
	files, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Check content of first file
	content, err := os.ReadFile(filepath.Join(outputDir, "memory_1.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty file")
	}
}

func TestMemoryRecallRecent(t *testing.T) {
	mem, err := New(Config{
		DBPath:   ":memory:",
		Embedder: &mockEmbedder{dimension: 128},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer mem.Close()

	// Create memories
	for i := 0; i < 5; i++ {
		_, _ = mem.Remember("Memory content", RememberOptions{})
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	entries, err := mem.RecallRecent(3, RecallOptions{})
	if err != nil {
		t.Fatalf("RecallRecent failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Check they're sorted by creation time (most recent first)
	for i := 1; i < len(entries); i++ {
		if entries[i].CreatedAt.After(entries[i-1].CreatedAt) {
			t.Error("entries not sorted by creation time")
		}
	}
}

func TestMemoryStats(t *testing.T) {
	mem, err := New(Config{
		DBPath:   ":memory:",
		Embedder: &mockEmbedder{dimension: 128},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer mem.Close()

	// Create some memories
	_, _ = mem.Remember("Memory 1", RememberOptions{Importance: 0.8})
	_, _ = mem.Remember("Memory 2", RememberOptions{Importance: 0.6})

	stats := mem.Stats()

	if stats.TotalMemories != 2 {
		t.Errorf("expected 2 memories, got %d", stats.TotalMemories)
	}
	if stats.AverageImportance < 0.69 || stats.AverageImportance > 0.71 {
		t.Errorf("expected average importance ~0.7, got %v", stats.AverageImportance)
	}
}

func TestMemoryWithPrecomputedVector(t *testing.T) {
	mem, err := New(Config{
		DBPath: ":memory:",
		// No embedder - using pre-computed vectors
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer mem.Close()

	// Remember with pre-computed vector
	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = 0.1
	}

	id, err := mem.Remember("Test content", RememberOptions{
		Vector: vec,
	})
	if err != nil {
		t.Fatalf("Remember failed: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}

	// Recall with pre-computed vector
	entries, err := mem.Recall("", RecallOptions{
		Vector: vec,
	})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}
