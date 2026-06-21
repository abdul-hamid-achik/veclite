package openai

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
)

// Integration tests for OpenAI embedder
// Run with: VECLITE_OPENAI_TEST=1 OPENAI_API_KEY=... go test ./embed/openai/... -v -run Integration

// TC-OPENAI-001: Concurrent embedding calls
func TestConcurrentEmbedIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	const numGoroutines = 5
	texts := []string{
		"First text for concurrent test",
		"Second text with different content",
		"Third text about something else",
		"Fourth text for variety",
		"Fifth text to complete the set",
	}

	var wg sync.WaitGroup
	results := make(chan error, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			vec, err := e.Embed(texts[idx])
			if err != nil {
				results <- err
				return
			}
			if len(vec) != e.Dimension() {
				results <- fmt.Errorf("goroutine %d: wrong dimension", idx)
				return
			}
			results <- nil
		}(i)
	}

	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
}

// TC-OPENAI-002: Unicode and multilingual text
func TestUnicodeTextIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	unicodeTexts := []string{
		"Hello 世界",          // Chinese
		"Привет мир",        // Russian
		"مرحبا بالعالم",     // Arabic
		"こんにちは世界",           // Japanese
		"🌍 Earth 🌎 Globe 🌏", // Emoji
		"café résumé naïve", // Accented Latin
	}

	for _, text := range unicodeTexts {
		vec, err := e.Embed(text)
		if err != nil {
			t.Errorf("Embed(%q) failed: %v", text, err)
			continue
		}
		if len(vec) != e.Dimension() {
			t.Errorf("Embed(%q): expected dim %d, got %d", text, e.Dimension(), len(vec))
		}
	}
}

// TC-OPENAI-003: Long text handling (within 8192 token limit)
func TestLongTextIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	// Create a long text (~6000 tokens, under 8192 limit)
	// Each sentence is ~11 tokens, so 500 sentences ≈ 5500 tokens
	var sb strings.Builder
	for range 500 {
		sb.WriteString("This is a test sentence for embedding long text content. ")
	}
	longText := sb.String()

	vec, err := e.Embed(longText)
	if err != nil {
		t.Fatalf("Embed long text failed: %v", err)
	}

	if len(vec) != e.Dimension() {
		t.Errorf("expected dimension %d, got %d", e.Dimension(), len(vec))
	}
}

// TC-OPENAI-004: Minimal text (single character)
func TestMinimalTextIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	minimalTexts := []string{"a", ".", "1", " "}

	for _, text := range minimalTexts {
		vec, err := e.Embed(text)
		if err != nil {
			t.Errorf("Embed(%q) failed: %v", text, err)
			continue
		}
		if len(vec) != e.Dimension() {
			t.Errorf("Embed(%q): expected dim %d, got %d", text, e.Dimension(), len(vec))
		}
	}
}

// TC-OPENAI-005: Batch with mixed content
func TestMixedBatchIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	texts := []string{
		"Short",
		"A medium length sentence for testing purposes.",
		"This is a much longer text that contains multiple sentences. It has more content than the others. We want to test batch processing with varying lengths.",
		"中文测试",
		"",
	}

	// Remove empty string for now (OpenAI may reject it)
	texts = texts[:4]

	vecs, err := e.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(vecs) != len(texts) {
		t.Errorf("expected %d vectors, got %d", len(texts), len(vecs))
	}

	for i, vec := range vecs {
		if len(vec) != e.Dimension() {
			t.Errorf("vector %d: expected dim %d, got %d", i, e.Dimension(), len(vec))
		}
	}
}

// TC-OPENAI-006: Embedding consistency (same text = same embedding)
func TestEmbeddingConsistencyIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	text := "The quick brown fox jumps over the lazy dog."

	vec1, err := e.Embed(text)
	if err != nil {
		t.Fatalf("First Embed failed: %v", err)
	}

	vec2, err := e.Embed(text)
	if err != nil {
		t.Fatalf("Second Embed failed: %v", err)
	}

	// OpenAI embeddings should be deterministic
	similarity := cosineSimilarity(vec1, vec2)
	if similarity < 0.9999 {
		t.Errorf("same text should produce nearly identical embeddings, got similarity %f", similarity)
	}
}

// TC-OPENAI-007: Model selection (text-embedding-3-large)
func TestLargeModelIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder(WithModel("text-embedding-3-large"))
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	if e.Dimension() != DimensionLarge {
		t.Errorf("expected dimension %d for large model, got %d", DimensionLarge, e.Dimension())
	}

	vec, err := e.Embed("Test with large model")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) != DimensionLarge {
		t.Errorf("expected %d dimensions, got %d", DimensionLarge, len(vec))
	}
}

// TC-OPENAI-008: Reduced dimensions (text-embedding-3-small with 512 dims)
func TestReducedDimensionsIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder(
		WithModel("text-embedding-3-small"),
		WithDimension(512),
	)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	if e.Dimension() != 512 {
		t.Errorf("expected dimension 512, got %d", e.Dimension())
	}

	vec, err := e.Embed("Test with reduced dimensions")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) != 512 {
		t.Errorf("expected 512 dimensions, got %d", len(vec))
	}

	// Verify it's still normalized
	var mag float64
	for _, v := range vec {
		mag += float64(v) * float64(v)
	}
	mag = math.Sqrt(mag)

	// OpenAI embeddings are normalized
	if math.Abs(mag-1.0) > 0.01 {
		t.Errorf("expected unit vector (mag ~1.0), got %f", mag)
	}
}

// TC-OPENAI-009: Semantic clustering
func TestSemanticClusteringIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	// Animals cluster
	cat, _ := e.Embed("cat")
	dog, _ := e.Embed("dog")
	lion, _ := e.Embed("lion")

	// Technology cluster
	computer, _ := e.Embed("computer")
	software, _ := e.Embed("software")
	programming, _ := e.Embed("programming")

	// Animals should be more similar to each other than to tech
	animalSim := (cosineSimilarity(cat, dog) + cosineSimilarity(cat, lion) + cosineSimilarity(dog, lion)) / 3
	techSim := (cosineSimilarity(computer, software) + cosineSimilarity(computer, programming) + cosineSimilarity(software, programming)) / 3
	crossSim := (cosineSimilarity(cat, computer) + cosineSimilarity(dog, software) + cosineSimilarity(lion, programming)) / 3

	t.Logf("Animal cluster similarity: %f", animalSim)
	t.Logf("Tech cluster similarity: %f", techSim)
	t.Logf("Cross-cluster similarity: %f", crossSim)

	if crossSim >= animalSim || crossSim >= techSim {
		t.Errorf("cross-cluster similarity (%f) should be lower than within-cluster similarities (animals: %f, tech: %f)",
			crossSim, animalSim, techSim)
	}
}

// TC-OPENAI-010: Large batch (50 items)
func TestLargeBatchIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	const batchSize = 50
	texts := make([]string, batchSize)
	for i := range texts {
		texts[i] = "This is test document number " + string(rune('A'+i%26))
	}

	vecs, err := e.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(vecs) != batchSize {
		t.Errorf("expected %d vectors, got %d", batchSize, len(vecs))
	}
}

// Benchmark for OpenAI embeddings
func BenchmarkEmbedOpenAI(b *testing.B) {
	if os.Getenv("VECLITE_OPENAI_TEST") == "" || os.Getenv("OPENAI_API_KEY") == "" {
		b.Skip("VECLITE_OPENAI_TEST or OPENAI_API_KEY not set")
	}

	e, err := NewEmbedder()
	if err != nil {
		b.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	text := "This is a sample text for benchmarking OpenAI embedding performance."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.Embed(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}
