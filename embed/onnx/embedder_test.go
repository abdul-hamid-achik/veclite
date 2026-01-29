//go:build onnx
// +build onnx

package onnx

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestEmbedderInterface verifies that Embedder implements the expected interface.
func TestEmbedderInterface(t *testing.T) {
	var _ interface {
		Embed(string) ([]float32, error)
		EmbedBatch([]string) ([][]float32, error)
		Dimension() int
	} = (*Embedder)(nil)
}

// TestL2Normalize verifies L2 normalization.
func TestL2Normalize(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
	}{
		{
			name:  "simple vector",
			input: []float32{3.0, 4.0},
		},
		{
			name:  "uniform vector",
			input: []float32{1.0, 1.0, 1.0, 1.0},
		},
		{
			name:  "mixed signs",
			input: []float32{-1.0, 2.0, -3.0, 4.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := l2Normalize(tt.input)

			// Calculate magnitude
			var mag float64
			for _, v := range result {
				mag += float64(v) * float64(v)
			}
			mag = math.Sqrt(mag)

			// Should be approximately 1.0
			if math.Abs(mag-1.0) > 1e-6 {
				t.Errorf("expected magnitude 1.0, got %f", mag)
			}
		})
	}
}

// TestL2NormalizeZeroVector verifies zero vector handling.
func TestL2NormalizeZeroVector(t *testing.T) {
	input := []float32{0.0, 0.0, 0.0}
	result := l2Normalize(input)

	// Should return same vector (avoid division by zero)
	for i, v := range result {
		if v != 0.0 {
			t.Errorf("expected 0.0 at index %d, got %f", i, v)
		}
	}
}

// TestSqrt32 verifies square root calculation.
func TestSqrt32(t *testing.T) {
	tests := []struct {
		input    float32
		expected float32
	}{
		{4.0, 2.0},
		{9.0, 3.0},
		{16.0, 4.0},
		{2.0, 1.414213562},
		{0.0, 0.0},
	}

	for _, tt := range tests {
		result := sqrt32(tt.input)
		if math.Abs(float64(result-tt.expected)) > 1e-5 {
			t.Errorf("sqrt32(%f) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}

// TestOptions verifies option functions.
func TestOptions(t *testing.T) {
	cfg := &config{
		maxLen: DefaultMaxLength,
		dim:    MiniLMDimension,
	}

	WithMaxLength(512)(cfg)
	if cfg.maxLen != 512 {
		t.Errorf("expected maxLen 512, got %d", cfg.maxLen)
	}

	WithDimension(768)(cfg)
	if cfg.dim != 768 {
		t.Errorf("expected dim 768, got %d", cfg.dim)
	}
}

// TestNewMiniLMMissingFiles verifies error handling for missing files.
func TestNewMiniLMMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := NewMiniLM(tmpDir)
	if err == nil {
		t.Error("expected error for missing model files")
	}
}

// TestEmbedderDimension verifies dimension getter.
func TestEmbedderDimension(t *testing.T) {
	e := &Embedder{dim: 384}
	if e.Dimension() != 384 {
		t.Errorf("expected dimension 384, got %d", e.Dimension())
	}
}

// TestConstants verifies expected constant values.
func TestConstants(t *testing.T) {
	if MiniLMDimension != 384 {
		t.Errorf("expected MiniLMDimension 384, got %d", MiniLMDimension)
	}

	if DefaultMaxLength != 256 {
		t.Errorf("expected DefaultMaxLength 256, got %d", DefaultMaxLength)
	}
}

// Integration tests require model files to be present.
// Set VECLITE_ONNX_MODEL_DIR environment variable to run these tests.

func getModelDir(t *testing.T) string {
	dir := os.Getenv("VECLITE_ONNX_MODEL_DIR")
	if dir == "" {
		t.Skip("VECLITE_ONNX_MODEL_DIR not set, skipping integration test")
	}
	return dir
}

func TestEmbedIntegration(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	vec, err := embedder.Embed("Hello, world!")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) != MiniLMDimension {
		t.Errorf("expected dimension %d, got %d", MiniLMDimension, len(vec))
	}

	// Verify L2 normalized
	var mag float64
	for _, v := range vec {
		mag += float64(v) * float64(v)
	}
	mag = math.Sqrt(mag)
	if math.Abs(mag-1.0) > 1e-5 {
		t.Errorf("expected unit vector, got magnitude %f", mag)
	}
}

func TestEmbedBatchIntegration(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	texts := []string{
		"The quick brown fox jumps over the lazy dog.",
		"Machine learning is a subset of artificial intelligence.",
		"Golang is a statically typed programming language.",
	}

	vecs, err := embedder.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(vecs) != len(texts) {
		t.Errorf("expected %d vectors, got %d", len(texts), len(vecs))
	}

	for i, vec := range vecs {
		if len(vec) != MiniLMDimension {
			t.Errorf("vector %d: expected dimension %d, got %d", i, MiniLMDimension, len(vec))
		}
	}
}

func TestEmbedBatchEmpty(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	vecs, err := embedder.EmbedBatch(nil)
	if err != nil {
		t.Fatalf("EmbedBatch(nil) failed: %v", err)
	}
	if vecs != nil {
		t.Error("expected nil for empty input")
	}

	vecs, err = embedder.EmbedBatch([]string{})
	if err != nil {
		t.Fatalf("EmbedBatch([]) failed: %v", err)
	}
	if vecs != nil {
		t.Error("expected nil for empty slice")
	}
}

func TestSemanticSimilarityIntegration(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	// Embed semantically similar and dissimilar texts
	similar1, _ := embedder.Embed("The cat sat on the mat.")
	similar2, _ := embedder.Embed("A kitten was sitting on a rug.")
	different, _ := embedder.Embed("Quantum computing uses qubits for computation.")

	// Calculate cosine similarity (vectors are already normalized)
	simSimilar := cosineSimilarity(similar1, similar2)
	simDifferent := cosineSimilarity(similar1, different)

	// Similar texts should have higher similarity
	if simSimilar <= simDifferent {
		t.Errorf("expected similar texts to have higher similarity: similar=%f, different=%f",
			simSimilar, simDifferent)
	}

	// Similar texts should have reasonably high similarity
	if simSimilar < 0.5 {
		t.Errorf("expected similar texts to have similarity > 0.5, got %f", simSimilar)
	}
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	// Vectors are already L2 normalized, so dot product = cosine similarity
	return dot
}

// TC-ONNX-001: Concurrent embedding calls
func TestConcurrentEmbed(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	const numGoroutines = 10
	texts := []string{
		"First text for concurrent test",
		"Second text with different content",
		"Third text about something else",
		"Fourth text for variety",
		"Fifth text to fill the batch",
		"Sixth text continues the pattern",
		"Seventh text almost there",
		"Eighth text getting close",
		"Ninth text nearly done",
		"Tenth text final one",
	}

	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			vec, err := embedder.Embed(texts[idx])
			if err != nil {
				results <- err
				return
			}
			if len(vec) != MiniLMDimension {
				results <- fmt.Errorf("goroutine %d: expected dim %d, got %d", idx, MiniLMDimension, len(vec))
				return
			}
			results <- nil
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}

// TC-ONNX-002: Close() is idempotent
func TestCloseIdempotent(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}

	// First close should succeed
	if err := embedder.Close(); err != nil {
		t.Errorf("first Close() failed: %v", err)
	}

	// Second close should not panic or error
	if err := embedder.Close(); err != nil {
		t.Errorf("second Close() failed: %v", err)
	}
}

// TC-ONNX-003: Use after Close() returns error
func TestUseAfterClose(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}

	// Close the embedder
	if err := embedder.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Attempt to use after close
	_, err = embedder.Embed("test text")
	if err == nil {
		t.Error("expected error when using closed embedder")
	}
	if err != ErrClosed {
		t.Errorf("expected ErrClosed, got: %v", err)
	}
}

// TC-ONNX-004: Text exceeding maxLen is truncated
func TestLongTextTruncation(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	// Create text with many words (should exceed 256 tokens)
	longText := ""
	for i := 0; i < 500; i++ {
		longText += "word "
	}

	vec, err := embedder.Embed(longText)
	if err != nil {
		t.Fatalf("Embed long text failed: %v", err)
	}

	if len(vec) != MiniLMDimension {
		t.Errorf("expected dimension %d, got %d", MiniLMDimension, len(vec))
	}
}

// TC-ONNX-005: Unicode and emoji text
func TestUnicodeText(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	unicodeTexts := []string{
		"Hello 世界",
		"Привет мир",
		"مرحبا بالعالم",
		"🌍 Earth 🌎 Globe 🌏",
		"café résumé naïve",
	}

	for _, text := range unicodeTexts {
		vec, err := embedder.Embed(text)
		if err != nil {
			t.Errorf("Embed(%q) failed: %v", text, err)
			continue
		}
		if len(vec) != MiniLMDimension {
			t.Errorf("Embed(%q): expected dim %d, got %d", text, MiniLMDimension, len(vec))
		}
	}
}

// TC-ONNX-006: Single character and minimal text
func TestMinimalText(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	minimalTexts := []string{
		"a",
		".",
		"1",
	}

	for _, text := range minimalTexts {
		vec, err := embedder.Embed(text)
		if err != nil {
			t.Errorf("Embed(%q) failed: %v", text, err)
			continue
		}
		if len(vec) != MiniLMDimension {
			t.Errorf("Embed(%q): expected dim %d, got %d", text, MiniLMDimension, len(vec))
		}

		// Verify normalized
		var mag float64
		for _, v := range vec {
			mag += float64(v) * float64(v)
		}
		mag = math.Sqrt(mag)
		if math.Abs(mag-1.0) > 1e-5 {
			t.Errorf("Embed(%q): expected unit vector, got magnitude %f", text, mag)
		}
	}
}

// TC-ONNX-007: Whitespace-only text
func TestWhitespaceText(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	// Whitespace text - tokenizer may produce minimal tokens
	vec, err := embedder.Embed("   \t\n  ")
	if err != nil {
		t.Fatalf("Embed whitespace failed: %v", err)
	}

	if len(vec) != MiniLMDimension {
		t.Errorf("expected dimension %d, got %d", MiniLMDimension, len(vec))
	}
}

// TC-ONNX-008/009: NewEmbedder with invalid paths (unit test, no model needed)
func TestNewEmbedderInvalidPaths(t *testing.T) {
	// Non-existent tokenizer (checked first by tokenizers.FromFile)
	_, err := NewEmbedder("/nonexistent/model.onnx", "/nonexistent/tokenizer.json")
	if err == nil {
		t.Error("expected error for non-existent paths")
	}
}

// TC-ONNX-010: Large batch
func TestLargeBatch(t *testing.T) {
	modelDir := getModelDir(t)

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	const batchSize = 100 // Use 100 instead of 1000 for reasonable test time
	texts := make([]string, batchSize)
	for i := range texts {
		texts[i] = fmt.Sprintf("Text number %d for large batch test", i)
	}

	vecs, err := embedder.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(vecs) != batchSize {
		t.Errorf("expected %d vectors, got %d", batchSize, len(vecs))
	}
}

// Benchmarks

func BenchmarkEmbed(b *testing.B) {
	modelDir := os.Getenv("VECLITE_ONNX_MODEL_DIR")
	if modelDir == "" {
		b.Skip("VECLITE_ONNX_MODEL_DIR not set")
	}

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		b.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	text := "This is a sample text for benchmarking the embedding performance."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := embedder.Embed(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEmbedBatch10(b *testing.B) {
	modelDir := os.Getenv("VECLITE_ONNX_MODEL_DIR")
	if modelDir == "" {
		b.Skip("VECLITE_ONNX_MODEL_DIR not set")
	}

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		b.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	texts := make([]string, 10)
	for i := range texts {
		texts[i] = "This is a sample text for benchmarking batch embedding performance."
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := embedder.EmbedBatch(texts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEmbedBatch100(b *testing.B) {
	modelDir := os.Getenv("VECLITE_ONNX_MODEL_DIR")
	if modelDir == "" {
		b.Skip("VECLITE_ONNX_MODEL_DIR not set")
	}

	embedder, err := NewMiniLM(modelDir)
	if err != nil {
		b.Fatalf("failed to create embedder: %v", err)
	}
	defer embedder.Close()

	texts := make([]string, 100)
	for i := range texts {
		texts[i] = "This is a sample text for benchmarking batch embedding performance."
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := embedder.EmbedBatch(texts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestDownloadIntegration tests the download functionality.
// This test actually downloads files and is skipped by default.
func TestDownloadIntegration(t *testing.T) {
	if os.Getenv("VECLITE_TEST_DOWNLOAD") == "" {
		t.Skip("VECLITE_TEST_DOWNLOAD not set, skipping download test")
	}

	tmpDir := t.TempDir()

	err := DownloadMiniLM(tmpDir)
	if err != nil {
		t.Fatalf("DownloadMiniLM failed: %v", err)
	}

	// Verify files exist
	modelPath := filepath.Join(tmpDir, "model.onnx")
	tokenizerPath := filepath.Join(tmpDir, "tokenizer.json")

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Error("model.onnx not downloaded")
	}

	if _, err := os.Stat(tokenizerPath); os.IsNotExist(err) {
		t.Error("tokenizer.json not downloaded")
	}
}
