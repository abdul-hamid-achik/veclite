package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/embed/common"
)

// Model dimension defaults.
const (
	// DimensionSmall is the default dimension for text-embedding-3-small.
	DimensionSmall = 1536

	// DimensionLarge is the default dimension for text-embedding-3-large.
	DimensionLarge = 3072

	// DimensionAda is the dimension for text-embedding-ada-002.
	DimensionAda = 1536

	// DefaultModel is the default embedding model.
	DefaultModel = "text-embedding-3-small"

	// DefaultBaseURL is the default OpenAI API base URL.
	DefaultBaseURL = "https://api.openai.com/v1"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
)

// config holds embedder configuration.
type config struct {
	apiKey    string
	model     string
	baseURL   string
	dimension int
	timeout   time.Duration
}

// Option configures the embedder.
type Option func(*config)

// WithAPIKey sets the OpenAI API key.
func WithAPIKey(key string) Option {
	return func(c *config) {
		c.apiKey = key
	}
}

// WithModel sets the embedding model.
func WithModel(model string) Option {
	return func(c *config) {
		c.model = model
	}
}

// WithBaseURL sets the API base URL (useful for Azure OpenAI or proxies).
func WithBaseURL(url string) Option {
	return func(c *config) {
		c.baseURL = url
	}
}

// WithDimension sets the embedding dimension (for text-embedding-3-* models).
func WithDimension(dim int) Option {
	return func(c *config) {
		c.dimension = dim
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.timeout = timeout
	}
}

// Embedder implements veclite.Embedder using the OpenAI embedding API.
type Embedder struct {
	cfg        *config
	httpClient *http.Client
	mu         sync.Mutex
	closed     bool
}

// NewEmbedder creates a new OpenAI embedder with the given options.
func NewEmbedder(opts ...Option) (*Embedder, error) {
	cfg := &config{
		model:   DefaultModel,
		baseURL: DefaultBaseURL,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Try environment variable if no API key provided
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.apiKey == "" {
		return nil, common.ErrNoAPIKey
	}

	// Set default dimension based on model
	if cfg.dimension == 0 {
		cfg.dimension = dimensionForModel(cfg.model)
	}

	return &Embedder{
		cfg:        cfg,
		httpClient: common.DefaultHTTPClient(cfg.timeout),
	}, nil
}

// dimensionForModel returns the default dimension for a given model.
func dimensionForModel(model string) int {
	switch model {
	case "text-embedding-3-large":
		return DimensionLarge
	case "text-embedding-ada-002":
		return DimensionAda
	default:
		return DimensionSmall
	}
}

// Embed converts a single text into a vector embedding.
func (e *Embedder) Embed(text string) ([]float32, error) {
	results, err := e.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// embeddingRequest is the OpenAI embedding API request format.
type embeddingRequest struct {
	Input      any    `json:"input"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions,omitempty"`
}

// embeddingResponse is the OpenAI embedding API response format.
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// errorResponse is the OpenAI API error format.
type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// EmbedBatch converts multiple texts into vector embeddings.
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

	// Build request
	reqBody := embeddingRequest{
		Input: texts,
		Model: e.cfg.model,
	}

	// Include dimensions for text-embedding-3-* models if specified
	if e.cfg.dimension > 0 && (e.cfg.model == "text-embedding-3-small" || e.cfg.model == "text-embedding-3-large") {
		reqBody.Dimensions = e.cfg.dimension
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := e.cfg.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.apiKey)

	// Set GetBody for retry support
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(jsonBody)), nil
	}

	// Execute with retry
	resp, err := common.DoWithRetry(context.Background(), e.httpClient, req, common.DefaultRetryConfig())
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, common.NewAPIError("openai", resp.StatusCode, errResp.Error.Message)
		}
		return nil, common.NewAPIError("openai", resp.StatusCode, string(body))
	}

	// Parse response
	var embResp embeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("openai: failed to parse response: %w", err)
	}

	if len(embResp.Data) != len(texts) {
		return nil, fmt.Errorf("openai: expected %d embeddings, got %d", len(texts), len(embResp.Data))
	}

	// Convert to float32 and ensure correct order
	results := make([][]float32, len(texts))
	for _, data := range embResp.Data {
		results[data.Index] = common.Float64ToFloat32(data.Embedding)
	}

	return results, nil
}

// Dimension returns the output vector dimension.
func (e *Embedder) Dimension() int {
	return e.cfg.dimension
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
