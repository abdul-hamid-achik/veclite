package openai

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
		model:   DefaultModel,
		baseURL: DefaultBaseURL,
		timeout: DefaultTimeout,
	}

	WithAPIKey("test-key")(cfg)
	if cfg.apiKey != "test-key" {
		t.Errorf("expected apiKey 'test-key', got %q", cfg.apiKey)
	}

	WithModel("text-embedding-3-large")(cfg)
	if cfg.model != "text-embedding-3-large" {
		t.Errorf("expected model 'text-embedding-3-large', got %q", cfg.model)
	}

	WithBaseURL("https://custom.api.com")(cfg)
	if cfg.baseURL != "https://custom.api.com" {
		t.Errorf("expected baseURL 'https://custom.api.com', got %q", cfg.baseURL)
	}

	WithDimension(512)(cfg)
	if cfg.dimension != 512 {
		t.Errorf("expected dimension 512, got %d", cfg.dimension)
	}
}

// TestNewEmbedderNoAPIKey verifies error when no API key is provided.
func TestNewEmbedderNoAPIKey(t *testing.T) {
	// Ensure env var is not set for this test
	orig := os.Getenv("OPENAI_API_KEY")
	_ = os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if orig != "" {
			_ = os.Setenv("OPENAI_API_KEY", orig)
		}
	}()

	_, err := NewEmbedder()
	if err != common.ErrNoAPIKey {
		t.Errorf("expected ErrNoAPIKey, got %v", err)
	}
}

// TestNewEmbedderWithEnvKey verifies API key from environment.
func TestNewEmbedderWithEnvKey(t *testing.T) {
	orig := os.Getenv("OPENAI_API_KEY")
	_ = os.Setenv("OPENAI_API_KEY", "env-test-key")
	defer func() {
		if orig != "" {
			_ = os.Setenv("OPENAI_API_KEY", orig)
		} else {
			_ = os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.cfg.apiKey != "env-test-key" {
		t.Errorf("expected apiKey from env, got %q", e.cfg.apiKey)
	}
}

// TestDimensionForModel verifies default dimensions for models.
func TestDimensionForModel(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"text-embedding-3-small", DimensionSmall},
		{"text-embedding-3-large", DimensionLarge},
		{"text-embedding-ada-002", DimensionAda},
		{"unknown-model", DimensionSmall}, // defaults to small
	}

	for _, tt := range tests {
		got := dimensionForModel(tt.model)
		if got != tt.expected {
			t.Errorf("dimensionForModel(%q) = %d, expected %d", tt.model, got, tt.expected)
		}
	}
}

// TestEmbedderDimension verifies dimension getter.
func TestEmbedderDimension(t *testing.T) {
	e := &Embedder{cfg: &config{dimension: 1536}}
	if e.Dimension() != 1536 {
		t.Errorf("expected dimension 1536, got %d", e.Dimension())
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
	e := &Embedder{cfg: &config{apiKey: "test"}, httpClient: http.DefaultClient}

	if err := e.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	_, err := e.EmbedBatch([]string{"test"})
	if err != common.ErrClosed {
		t.Errorf("expected ErrClosed, got: %v", err)
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
		if r.URL.Path != "/embeddings" {
			t.Errorf("expected path /embeddings, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header")
		}

		// Parse request
		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		texts := req.Input.([]any)

		// Build response
		resp := embeddingResponse{
			Object: "list",
			Model:  req.Model,
		}
		for i := range texts {
			// Generate fake embedding (768 dims for testing)
			embedding := make([]float64, 768)
			for j := range embedding {
				embedding[j] = float64(i+1) * 0.001 * float64(j)
			}
			resp.Data = append(resp.Data, struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float64 `json:"embedding"`
			}{
				Object:    "embedding",
				Index:     i,
				Embedding: embedding,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create embedder with mock server
	e, err := NewEmbedder(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithDimension(768),
	)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer func() { _ = e.Close() }()

	// Test single embed
	vec, err := e.Embed("Hello world")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(vec) != 768 {
		t.Errorf("expected 768 dimensions, got %d", len(vec))
	}

	// Test batch embed
	texts := []string{"First", "Second", "Third"}
	vecs, err := e.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Errorf("expected %d vectors, got %d", len(texts), len(vecs))
	}
}

// TestEmbedAPIError tests error handling for API errors.
func TestEmbedAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	e, err := NewEmbedder(
		WithAPIKey("bad-key"),
		WithBaseURL(server.URL),
	)
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer func() { _ = e.Close() }()

	_, err = e.Embed("test")
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}

	apiErr, ok := err.(*common.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
}

// Integration tests require OPENAI_API_KEY.
// Set VECLITE_OPENAI_TEST=1 to run these tests.

func skipWithoutOpenAI(t *testing.T) {
	if os.Getenv("VECLITE_OPENAI_TEST") == "" {
		t.Skip("VECLITE_OPENAI_TEST not set")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
}

func TestEmbedIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer func() { _ = e.Close() }()

	vec, err := e.Embed("Hello, world!")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) != e.Dimension() {
		t.Errorf("expected dimension %d, got %d", e.Dimension(), len(vec))
	}
}

func TestEmbedBatchIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer func() { _ = e.Close() }()

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

	for i, vec := range vecs {
		if len(vec) != e.Dimension() {
			t.Errorf("vector %d: expected dimension %d, got %d", i, e.Dimension(), len(vec))
		}
	}
}

func TestSemanticSimilarityIntegration(t *testing.T) {
	skipWithoutOpenAI(t)

	e, err := NewEmbedder()
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}
	defer func() { _ = e.Close() }()

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
