package veclite

import "errors"

// ErrNoEmbedder is returned when an embedding operation is attempted
// without an embedder configured on the collection.
var ErrNoEmbedder = errors.New("veclite: no embedder configured")

// Embedder is the interface for auto-embedding text to vectors.
// Implementations live in separate modules to maintain zero-dependency core.
type Embedder interface {
	// Embed converts a single text into a vector embedding.
	Embed(text string) ([]float32, error)

	// EmbedBatch converts multiple texts into vector embeddings.
	EmbedBatch(texts []string) ([][]float32, error)

	// Dimension returns the output vector dimension.
	Dimension() int
}
