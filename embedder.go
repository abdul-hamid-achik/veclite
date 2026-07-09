package veclite

import (
	"errors"

	"github.com/abdul-hamid-achik/veclite/embed/common"
)

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

// ProfiledEmbedder is an optional extension of Embedder for implementations
// that can describe how they produce vectors. It is not required: an Embedder
// may or may not support it, and callers discover support with a type
// assertion (or via EmbedderProfile, which also understands the built-in
// providers under embed/).
//
//	if pe, ok := e.(veclite.ProfiledEmbedder); ok {
//	    profile := pe.Profile()
//	    ...
//	}
type ProfiledEmbedder interface {
	Embedder

	// Profile describes the embedder's provider, model, dimension,
	// intended distance metric, and normalization behavior.
	Profile() EmbeddingProfile
}

// commonProfiled matches the built-in providers (embed/ollama, embed/openai,
// embed/onnx), which describe themselves with the provider-neutral
// common.ProfileData to avoid importing this package (the factory files here
// import them, so the reverse import would cycle).
type commonProfiled interface {
	Profile() common.ProfileData
}

// EmbedderProfile extracts an EmbeddingProfile from an embedder when it
// self-describes. It supports both ProfiledEmbedder implementations and the
// built-in providers under embed/ (which return common.ProfileData). The
// second return value reports whether the embedder provided a profile.
func EmbedderProfile(e Embedder) (EmbeddingProfile, bool) {
	switch p := e.(type) {
	case ProfiledEmbedder:
		return p.Profile(), true
	case commonProfiled:
		d := p.Profile()
		return EmbeddingProfile{
			Provider:  d.Provider,
			Model:     d.Model,
			Dimension: d.Dimension,
			Distance:  DistanceType(d.Distance),
			Normalize: d.Normalize,
			Version:   d.Version,
		}, true
	}
	return EmbeddingProfile{}, false
}
