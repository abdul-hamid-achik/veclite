// Package config provides YAML configuration loading for VecLite.
//
// This package imports gopkg.in/yaml.v3. Users who only need the core
// vector database functionality (package veclite) do not need to import
// this package and will not incur the yaml.v3 dependency.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the full veclite configuration.
type Config struct {
	Embedder EmbedderConfig `yaml:"embedder"`
}

// EmbedderConfig specifies which embedder provider to use and its settings.
type EmbedderConfig struct {
	Provider string       `yaml:"provider"`
	OpenAI   OpenAIConfig `yaml:"openai"`
	Ollama   OllamaConfig `yaml:"ollama"`
	ONNX     ONNXConfig   `yaml:"onnx"`
}

// OpenAIConfig holds OpenAI embedder configuration.
type OpenAIConfig struct {
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	Dimension int    `yaml:"dimension"`
	Timeout   string `yaml:"timeout"`
}

// OllamaConfig holds Ollama embedder configuration.
type OllamaConfig struct {
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
	Timeout string `yaml:"timeout"`
}

// ONNXConfig holds ONNX embedder configuration.
type ONNXConfig struct {
	ModelDir string `yaml:"model_dir"`
	Model    string `yaml:"model"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Embedder: EmbedderConfig{
			Provider: "ollama",
			OpenAI: OpenAIConfig{
				Model:   "text-embedding-3-small",
				BaseURL: "https://api.openai.com/v1",
				Timeout: "30s",
			},
			Ollama: OllamaConfig{
				BaseURL: "http://localhost:11434",
				Model:   "nomic-embed-text",
				Timeout: "30s",
			},
			ONNX: ONNXConfig{
				Model: "minilm",
			},
		},
	}
}

// LoadConfig loads configuration from a YAML file.
// It searches for config files in the following order:
// 1. The explicit path provided (if not empty)
// 2. ./veclite.yaml
// 3. ~/.veclite/config.yaml
// If no config file is found, returns default configuration.
func LoadConfig(path string) (*Config, error) {
	paths := []string{}

	if path != "" {
		paths = append(paths, path)
	}

	paths = append(paths, "veclite.yaml")
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".veclite", "config.yaml"))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return loadConfigFile(p)
		}
	}

	return DefaultConfig(), nil
}

func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("veclite: failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("veclite: failed to parse config file: %w", err)
	}

	cfg.Embedder.OpenAI.APIKey = ExpandEnvVars(cfg.Embedder.OpenAI.APIKey)
	cfg.Embedder.OpenAI.BaseURL = ExpandEnvVars(cfg.Embedder.OpenAI.BaseURL)
	cfg.Embedder.Ollama.BaseURL = ExpandEnvVars(cfg.Embedder.Ollama.BaseURL)
	cfg.Embedder.ONNX.ModelDir = ExpandEnvVars(cfg.Embedder.ONNX.ModelDir)

	return cfg, nil
}

var envVarPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

// ExpandEnvVars expands ${VAR} and ${VAR:-default} patterns in a string.
func ExpandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		parts := envVarPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		varName := parts[1]
		defaultValue := ""
		if len(parts) >= 3 {
			defaultValue = parts[2]
		}

		if value := os.Getenv(varName); value != "" {
			return value
		}
		return defaultValue
	})
}

// ParseDuration parses a duration string with a default fallback.
func ParseDuration(s string, defaultDur time.Duration) time.Duration {
	if s == "" {
		return defaultDur
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDur
	}
	return d
}

// ExpandPath expands ~ and environment variables in a path.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return ExpandEnvVars(path)
}
