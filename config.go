package veclite

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/abdul-hamid-achik/veclite/config"
	"gopkg.in/yaml.v3"
)

// Config represents the full veclite configuration.
// YAML configuration loading is available via the veclite/config sub-package.
type Config = config.Config

// EmbedderConfig specifies which embedder provider to use and its settings.
type EmbedderConfig = config.EmbedderConfig

// OpenAIConfig holds OpenAI embedder configuration.
type OpenAIConfig = config.OpenAIConfig

// OllamaConfig holds Ollama embedder configuration.
type OllamaConfig = config.OllamaConfig

// ONNXConfig holds ONNX embedder configuration.
type ONNXConfig = config.ONNXConfig

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return config.DefaultConfig()
}

// LoadConfig loads configuration from a YAML file.
// It searches for config files in the following order:
// 1. The explicit path provided (if not empty)
// 2. ./veclite.yaml
// 3. ~/.veclite/config.yaml
// If no config file is found, returns default configuration.
func LoadConfig(path string) (*Config, error) {
	return config.LoadConfig(path)
}

// ExpandPath expands ~ and environment variables in a path.
func ExpandPath(path string) string {
	return config.ExpandPath(path)
}

// parseDuration parses a duration string with a default fallback.
// Kept for internal use; prefer config.ParseDuration from the sub-package.
func parseDuration(s string, defaultDur time.Duration) time.Duration {
	return config.ParseDuration(s, defaultDur)
}

// The yaml.v3 import and env var expansion functions below are kept in this
// file for backward compatibility. The config sub-package has the canonical
// implementations.
var envVarPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

func expandEnvVars(s string) string {
	return config.ExpandEnvVars(s)
}

// loadConfigFile is kept for backward compatibility; prefer config.LoadConfig.
func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("veclite: failed to read config file: %w", err)
	}

	cfg := config.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("veclite: failed to parse config file: %w", err)
	}

	cfg.Embedder.OpenAI.APIKey = config.ExpandPath(cfg.Embedder.OpenAI.APIKey)
	cfg.Embedder.OpenAI.BaseURL = config.ExpandPath(cfg.Embedder.OpenAI.BaseURL)
	cfg.Embedder.Ollama.BaseURL = config.ExpandPath(cfg.Embedder.Ollama.BaseURL)
	cfg.Embedder.ONNX.ModelDir = config.ExpandPath(cfg.Embedder.ONNX.ModelDir)

	return cfg, nil
}
