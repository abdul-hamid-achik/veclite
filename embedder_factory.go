//go:build !onnx

package veclite

import (
	"fmt"

	"github.com/abdul-hamid-achik/veclite/embed/ollama"
	"github.com/abdul-hamid-achik/veclite/embed/openai"
)

// NewEmbedderFromConfig creates an embedder based on the provided configuration.
// It returns an Embedder that can be used with veclite collections.
//
// Supported providers:
//   - "openai": OpenAI embedding API (requires API key)
//   - "ollama": Ollama local embedding (requires Ollama running)
//   - "onnx": Local ONNX inference (requires onnx build tag)
//
// Example:
//
//	cfg, _ := veclite.LoadConfig("veclite.yaml")
//	embedder, _ := veclite.NewEmbedderFromConfig(cfg.Embedder)
//	defer embedder.Close()
//
//	db, _ := veclite.Open("data.veclite")
//	coll, _ := db.CreateCollection("docs",
//	    veclite.WithDimension(embedder.Dimension()),
//	    veclite.WithEmbedder(embedder),
//	)
func NewEmbedderFromConfig(cfg EmbedderConfig) (Embedder, error) {
	switch cfg.Provider {
	case "openai":
		return newOpenAIEmbedder(cfg.OpenAI)
	case "ollama":
		return newOllamaEmbedder(cfg.Ollama)
	case "onnx":
		return nil, fmt.Errorf("veclite: ONNX embedder requires building with -tags onnx")
	case "":
		return nil, fmt.Errorf("veclite: no embedder provider specified in config")
	default:
		return nil, fmt.Errorf("veclite: unknown embedder provider: %s", cfg.Provider)
	}
}

// newOpenAIEmbedder creates an OpenAI embedder from config.
func newOpenAIEmbedder(cfg OpenAIConfig) (Embedder, error) {
	var opts []openai.Option

	if cfg.APIKey != "" {
		opts = append(opts, openai.WithAPIKey(cfg.APIKey))
	}
	if cfg.Model != "" {
		opts = append(opts, openai.WithModel(cfg.Model))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	}
	if cfg.Dimension > 0 {
		opts = append(opts, openai.WithDimension(cfg.Dimension))
	}
	if cfg.Timeout != "" {
		opts = append(opts, openai.WithTimeout(parseDuration(cfg.Timeout, 0)))
	}

	return openai.NewEmbedder(opts...)
}

// newOllamaEmbedder creates an Ollama embedder from config.
func newOllamaEmbedder(cfg OllamaConfig) (Embedder, error) {
	var opts []ollama.Option

	if cfg.BaseURL != "" {
		opts = append(opts, ollama.WithBaseURL(cfg.BaseURL))
	}
	if cfg.Model != "" {
		opts = append(opts, ollama.WithModel(cfg.Model))
	}
	if cfg.Timeout != "" {
		opts = append(opts, ollama.WithTimeout(parseDuration(cfg.Timeout, 0)))
	}

	return ollama.NewEmbedder(opts...)
}
