package common

import "strings"

// knownModelDimensions maps well-known embedding model names to their output
// dimensions. Keys are lowercase and carry no ollama-style ":tag" suffix;
// lookups go through KnownModelDimensions, which normalizes the input the
// same way.
var knownModelDimensions = map[string]int{
	// OpenAI
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,

	// Ollama / open models
	"nomic-embed-text":        768,
	"mxbai-embed-large":       1024,
	"snowflake-arctic-embed":  1024,
	"snowflake-arctic-embed2": 1024,
	"all-minilm":              384,
	"all-minilm-l6-v2":        384,
	"bge-m3":                  1024,
	"bge-large":               1024,
	"granite-embedding":       384,
	"embeddinggemma":          768,
	"qwen3-embedding":         1024,
}

// KnownModelDimensions returns the output vector dimension for a well-known
// embedding model name. The lookup is case-insensitive and strips any
// ollama-style ":tag" suffix, so "nomic-embed-text:latest" and
// "All-MiniLM-L6-v2" both resolve. It returns (0, false) for unknown models.
func KnownModelDimensions(model string) (int, bool) {
	name := model
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	dim, ok := knownModelDimensions[strings.ToLower(name)]
	return dim, ok
}
