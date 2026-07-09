package common

import "testing"

// TestKnownModelDimensions verifies the registry lookup, including
// case-insensitivity and ollama-style ":tag" suffix stripping.
func TestKnownModelDimensions(t *testing.T) {
	tests := []struct {
		model   string
		wantDim int
		wantOK  bool
	}{
		// OpenAI models
		{"text-embedding-3-small", 1536, true},
		{"text-embedding-3-large", 3072, true},
		{"text-embedding-ada-002", 1536, true},

		// Ollama / open models
		{"nomic-embed-text", 768, true},
		{"mxbai-embed-large", 1024, true},
		{"snowflake-arctic-embed", 1024, true},
		{"snowflake-arctic-embed2", 1024, true},
		{"all-minilm", 384, true},
		{"all-MiniLM-L6-v2", 384, true},
		{"bge-m3", 1024, true},
		{"bge-large", 1024, true},
		{"granite-embedding", 384, true},
		{"embeddinggemma", 768, true},
		{"qwen3-embedding", 1024, true},

		// ":tag" suffix stripping (ollama style)
		{"nomic-embed-text:latest", 768, true},
		{"mxbai-embed-large:v1", 1024, true},
		{"all-minilm:33m", 384, true},
		{"qwen3-embedding:0.6b", 1024, true},

		// Case-insensitivity
		{"Nomic-Embed-Text", 768, true},
		{"TEXT-EMBEDDING-3-LARGE", 3072, true},
		{"ALL-MINILM-L6-V2", 384, true},
		{"BGE-M3:LATEST", 1024, true},

		// Unknown models
		{"", 0, false},
		{"unknown-model", 0, false},
		{"unknown-model:latest", 0, false},
		{"gpt-4o", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			dim, ok := KnownModelDimensions(tt.model)
			if ok != tt.wantOK {
				t.Fatalf("KnownModelDimensions(%q) ok = %v, want %v", tt.model, ok, tt.wantOK)
			}
			if dim != tt.wantDim {
				t.Errorf("KnownModelDimensions(%q) = %d, want %d", tt.model, dim, tt.wantDim)
			}
		})
	}
}
