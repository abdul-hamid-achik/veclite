package veclite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Embedder.Provider != "ollama" {
		t.Errorf("expected default provider 'ollama', got %q", cfg.Embedder.Provider)
	}

	if cfg.Embedder.OpenAI.Model != "text-embedding-3-small" {
		t.Errorf("expected OpenAI model 'text-embedding-3-small', got %q", cfg.Embedder.OpenAI.Model)
	}

	if cfg.Embedder.Ollama.Model != "nomic-embed-text" {
		t.Errorf("expected Ollama model 'nomic-embed-text', got %q", cfg.Embedder.Ollama.Model)
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set up test env var
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		input    string
		expected string
	}{
		{"${TEST_VAR}", "test_value"},
		{"prefix_${TEST_VAR}_suffix", "prefix_test_value_suffix"},
		{"${NONEXISTENT_VAR}", ""},
		{"${NONEXISTENT_VAR:-default}", "default"},
		{"no_vars", "no_vars"},
		{"${TEST_VAR:-fallback}", "test_value"},
	}

	for _, tt := range tests {
		result := expandEnvVars(tt.input)
		if result != tt.expected {
			t.Errorf("expandEnvVars(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "veclite.yaml")

	configContent := `
embedder:
  provider: openai
  openai:
    api_key: test-key
    model: text-embedding-3-large
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Embedder.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", cfg.Embedder.Provider)
	}

	if cfg.Embedder.OpenAI.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got %q", cfg.Embedder.OpenAI.APIKey)
	}

	if cfg.Embedder.OpenAI.Model != "text-embedding-3-large" {
		t.Errorf("expected model 'text-embedding-3-large', got %q", cfg.Embedder.OpenAI.Model)
	}
}

func TestLoadConfigWithEnvExpansion(t *testing.T) {
	os.Setenv("TEST_API_KEY", "expanded-key")
	defer os.Unsetenv("TEST_API_KEY")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "veclite.yaml")

	configContent := `
embedder:
  provider: openai
  openai:
    api_key: ${TEST_API_KEY}
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Embedder.OpenAI.APIKey != "expanded-key" {
		t.Errorf("expected api_key 'expanded-key', got %q", cfg.Embedder.OpenAI.APIKey)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// When no config file exists, should return defaults
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadConfig with nonexistent path should return defaults, got error: %v", err)
	}

	if cfg.Embedder.Provider != "ollama" {
		t.Errorf("expected default provider 'ollama', got %q", cfg.Embedder.Provider)
	}
}

func TestExpandPath(t *testing.T) {
	os.Setenv("TEST_DIR", "/test/dir")
	defer os.Unsetenv("TEST_DIR")

	tests := []struct {
		input    string
		expected string
	}{
		{"${TEST_DIR}/models", "/test/dir/models"},
		{"/absolute/path", "/absolute/path"},
	}

	for _, tt := range tests {
		result := ExpandPath(tt.input)
		if result != tt.expected {
			t.Errorf("ExpandPath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}

	// Test ~ expansion (can't easily test exact value)
	result := ExpandPath("~/veclite")
	if result == "~/veclite" {
		// If it didn't expand, home dir might not be available
		t.Log("~ expansion may not work in test environment")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"30s", "30s"},
		{"1m", "1m0s"},
		{"", "10s"}, // default fallback
		{"invalid", "10s"},
	}

	for _, tt := range tests {
		result := parseDuration(tt.input, 10e9) // 10s default
		if result.String() != tt.expected {
			t.Errorf("parseDuration(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}
