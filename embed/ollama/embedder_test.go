package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/abdul-hamid-achik/veclite/embed/common"
)

// TestEmbedderInterface verifies that Embedder implements the expected interface.
func TestEmbedderInterface(t *testing.T) {
	var _ interface {
		Embed(string) ([]float32, error)
		EmbedBatch([]string) ([][]float32, error)
		Dimension() int
	} = (*Embedder)(nil)
}

// TestOptions verifies option functions.
func TestOptions(t *testing.T) {
	cfg := &config{
		baseURL: DefaultBaseURL,
		model:   DefaultModel,
		timeout: DefaultTimeout,
	}

	WithBaseURL("http://custom:11434")(cfg)
	if cfg.baseURL != "http://custom:11434" {
		t.Errorf("expected baseURL 'http://custom:11434', got %q", cfg.baseURL)
	}

	WithModel("mxbai-embed-large")(cfg)
	if cfg.model != "mxbai-embed-large" {
		t.Errorf("expected model 'mxbai-embed-large', got %q", cfg.model)
	}
}

// TestNewEmbedderDefaults verifies default configuration.
func TestNewEmbedderDefaults(t *testing.T) {
	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.cfg.baseURL != DefaultBaseURL {
		t.Errorf("expected baseURL %q, got %q", DefaultBaseURL, e.cfg.baseURL)
	}
	if e.cfg.model != DefaultModel {
		t.Errorf("expected model %q, got %q", DefaultModel, e.cfg.model)
	}
}

// TestKnownDimensions verifies known model dimensions.
func TestKnownDimensions(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"nomic-embed-text", 768},
		{"mxbai-embed-large", 1024},
		{"all-minilm", 384},
	}

	for _, tt := range tests {
		e, err := NewEmbedder(WithModel(tt.model))
		if err != nil {
			t.Fatalf("NewEmbedder failed: %v", err)
		}
		if e.dimension != tt.expected {
			t.Errorf("model %q: expected dimension %d, got %d", tt.model, tt.expected, e.dimension)
		}
	}
}

// TestEmbedBatchEmpty verifies empty batch handling.
func TestEmbedBatchEmpty(t *testing.T) {
	e := &Embedder{cfg: &config{}}

	vecs, err := e.EmbedBatch(nil)
	if err != nil {
		t.Fatalf("EmbedBatch(nil) failed: %v", err)
	}
	if vecs != nil {
		t.Error("expected nil for empty input")
	}

	vecs, err = e.EmbedBatch([]string{})
	if err != nil {
		t.Fatalf("EmbedBatch([]) failed: %v", err)
	}
	if vecs != nil {
		t.Error("expected nil for empty slice")
	}
}

// TestUseAfterClose verifies error after close.
func TestUseAfterClose(t *testing.T) {
	e := &Embedder{cfg: &config{}, httpClient: http.DefaultClient}

	if err := e.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	_, err := e.Embed("test")
	if err != common.ErrClosed {
		t.Errorf("expected ErrClosed, got: %v", err)
	}

	_, err = e.EmbedBatch([]string{"test"})
	if err != common.ErrClosed {
		t.Errorf("expected ErrClosed for batch, got: %v", err)
	}
}

// TestCloseIdempotent verifies close is safe to call multiple times.
func TestCloseIdempotent(t *testing.T) {
	e := &Embedder{cfg: &config{}, httpClient: http.DefaultClient}

	if err := e.Close(); err != nil {
		t.Errorf("first Close() failed: %v", err)
	}
	if err := e.Close(); err != nil {
		t.Errorf("second Close() failed: %v", err)
	}
}

// TestEmbedWithMockServer tests embedding with a mock HTTP server.
func TestEmbedWithMockServer(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("expected path /api/embeddings, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Parse request
		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Model != "nomic-embed-text" {
			t.Errorf("expected model 'nomic-embed-text', got %q", req.Model)
		}

		// Generate fake embedding (768 dims for nomic)
		embedding := make([]float64, 768)
		for i := range embedding {
			embedding[i] = float64(i) * 0.001
		}

		resp := embeddingResponse{
			Embedding: embedding,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create embedder with mock server
	e, err := NewEmbedder(WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	// Test single embed
	vec, err := e.Embed("Hello world")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(vec) != 768 {
		t.Errorf("expected 768 dimensions, got %d", len(vec))
	}

	// Dimension should be auto-detected
	if e.Dimension() != 768 {
		t.Errorf("expected Dimension() = 768, got %d", e.Dimension())
	}
}

// TestEmbedBatchWithMockServer tests batch embedding.
func TestEmbedBatchWithMockServer(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		embedding := make([]float64, 768)
		for i := range embedding {
			embedding[i] = float64(callCount) * 0.001 * float64(i)
		}

		resp := embeddingResponse{Embedding: embedding}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e, err := NewEmbedder(WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	texts := []string{"First", "Second", "Third"}
	vecs, err := e.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(vecs) != len(texts) {
		t.Errorf("expected %d vectors, got %d", len(texts), len(vecs))
	}

	// Should have made 3 separate calls (Ollama doesn't batch)
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

// TestEmbedAPIError tests error handling for API errors.
func TestEmbedAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "model 'unknown-model' not found",
		})
	}))
	defer server.Close()

	e, err := NewEmbedder(
		WithBaseURL(server.URL),
		WithModel("unknown-model"),
	)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	_, err = e.Embed("test")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}

	apiErr, ok := err.(*common.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

// TestEmptyEmbeddingError tests error handling for empty embeddings.
func TestEmptyEmbeddingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{Embedding: []float64{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e, err := NewEmbedder(WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	_, err = e.Embed("test")
	if err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

// Integration tests require Ollama to be running.
// Set VECLITE_OLLAMA_TEST=1 to run these tests.

func skipWithoutOllama(t *testing.T) {
	if os.Getenv("VECLITE_OLLAMA_TEST") == "" {
		t.Skip("VECLITE_OLLAMA_TEST not set")
	}

	// Check if Ollama is running
	resp, err := http.Get("http://localhost:11434/api/version")
	if err != nil {
		t.Skip("Ollama not running at localhost:11434")
	}
	resp.Body.Close()
}

func TestEmbedIntegration(t *testing.T) {
	skipWithoutOllama(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	vec, err := e.Embed("Hello, world!")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) == 0 {
		t.Error("expected non-empty embedding")
	}

	// Dimension should now be set
	if e.Dimension() == 0 {
		t.Error("expected Dimension() > 0 after embedding")
	}
}

func TestEmbedBatchIntegration(t *testing.T) {
	skipWithoutOllama(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	texts := []string{
		"The quick brown fox jumps over the lazy dog.",
		"Machine learning is a subset of artificial intelligence.",
		"Golang is a statically typed programming language.",
	}

	vecs, err := e.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(vecs) != len(texts) {
		t.Errorf("expected %d vectors, got %d", len(texts), len(vecs))
	}

	dim := e.Dimension()
	for i, vec := range vecs {
		if len(vec) != dim {
			t.Errorf("vector %d: expected dimension %d, got %d", i, dim, len(vec))
		}
	}
}

func TestSemanticSimilarityIntegration(t *testing.T) {
	skipWithoutOllama(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer e.Close()

	similar1, _ := e.Embed("The cat sat on the mat.")
	similar2, _ := e.Embed("A kitten was sitting on a rug.")
	different, _ := e.Embed("Quantum computing uses qubits for computation.")

	simSimilar := cosineSimilarity(similar1, similar2)
	simDifferent := cosineSimilarity(similar1, different)

	if simSimilar <= simDifferent {
		t.Errorf("expected similar texts to have higher similarity: similar=%f, different=%f",
			simSimilar, simDifferent)
	}
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for range 10 {
		z = (z + x/z) / 2
	}
	return z
}
