package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/embed/common"
)

// Default configuration values.
const (
	// DefaultBaseURL is the default Ollama API endpoint.
	DefaultBaseURL = "http://localhost:11434"

	// DefaultModel is the default embedding model.
	DefaultModel = "nomic-embed-text"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
)

// Known model dimensions for common models.
var knownDimensions = map[string]int{
	"nomic-embed-text":  768,
	"mxbai-embed-large": 1024,
	"all-minilm":        384,
	"snowflake-arctic-embed": 1024,
}

// config holds embedder configuration.
type config struct {
	baseURL string
	model   string
	timeout time.Duration
}

// Option configures the embedder.
type Option func(*config)

// WithBaseURL sets the Ollama API base URL.
func WithBaseURL(url string) Option {
	return func(c *config) {
		c.baseURL = url
	}
}

// WithModel sets the embedding model.
func WithModel(model string) Option {
	return func(c *config) {
		c.model = model
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.timeout = timeout
	}
}

// Embedder implements veclite.Embedder using the Ollama embedding API.
type Embedder struct {
	cfg        *config
	httpClient *http.Client
	dimension  int // Auto-detected on first call
	mu         sync.Mutex
	closed     bool
}

// NewEmbedder creates a new Ollama embedder with the given options.
func NewEmbedder(opts ...Option) (*Embedder, error) {
	cfg := &config{
		baseURL: DefaultBaseURL,
		model:   DefaultModel,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Use known dimension if available
	dimension := knownDimensions[cfg.model]

	return &Embedder{
		cfg:        cfg,
		httpClient: common.DefaultHTTPClient(cfg.timeout),
		dimension:  dimension,
	}, nil
}

// embeddingRequest is the Ollama embedding API request format.
type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// embeddingResponse is the Ollama embedding API response format.
type embeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// errorResponse is the Ollama API error format.
type errorResponse struct {
	Error string `json:"error"`
}

// Embed converts a single text into a vector embedding.
func (e *Embedder) Embed(text string) ([]float32, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, common.ErrClosed
	}
	e.mu.Unlock()

	// Build request
	reqBody := embeddingRequest{
		Model:  e.cfg.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := e.cfg.baseURL + "/api/embeddings"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("ollama: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Set GetBody for retry support
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(jsonBody)), nil
	}

	// Execute with retry
	resp, err := common.DoWithRetry(context.Background(), e.httpClient, req, common.DefaultRetryConfig())
	if err != nil {
		// Check for connection refused
		return nil, fmt.Errorf("ollama: request failed (is Ollama running at %s?): %w", e.cfg.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, common.NewAPIError("ollama", resp.StatusCode, errResp.Error)
		}
		return nil, common.NewAPIError("ollama", resp.StatusCode, string(body))
	}

	// Parse response
	var embResp embeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("ollama: failed to parse response: %w", err)
	}

	if len(embResp.Embedding) == 0 {
		return nil, fmt.Errorf("ollama: empty embedding returned")
	}

	// Auto-detect dimension on first call
	e.mu.Lock()
	if e.dimension == 0 {
		e.dimension = len(embResp.Embedding)
	}
	e.mu.Unlock()

	// Convert float64 to float32
	return common.Float64ToFloat32(embResp.Embedding), nil
}

// EmbedBatch converts multiple texts into vector embeddings.
// Note: Ollama doesn't support batch embedding, so this calls Embed for each text.
func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, common.ErrClosed
	}
	e.mu.Unlock()

	results := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := e.Embed(text)
		if err != nil {
			return nil, err
		}
		results[i] = vec
	}
	return results, nil
}

// Dimension returns the output vector dimension.
// Returns 0 if no embedding has been performed yet (dimension auto-detected on first call).
func (e *Embedder) Dimension() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dimension
}

// Close releases resources. Safe to call multiple times.
func (e *Embedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	return nil
}

// Ensure Embedder implements the interface at compile time.
var _ interface {
	Embed(string) ([]float32, error)
	EmbedBatch([]string) ([][]float32, error)
	Dimension() int
} = (*Embedder)(nil)
