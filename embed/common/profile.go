package common

// ProfileData is a provider-neutral description of how an embedder produces
// vectors.
//
// Provider packages under embed/ cannot return veclite.EmbeddingProfile
// directly: the root package's factory files (embedder_factory.go,
// embedder_factory_onnx.go) import the providers, so a provider importing the
// root package would create an import cycle. Providers therefore expose
//
//	Profile() ProfileData
//
// and the root package adapts it to veclite.EmbeddingProfile (see
// veclite.EmbedderProfile).
type ProfileData struct {
	// Provider is the embedding provider, e.g. "openai", "ollama", or "onnx".
	Provider string

	// Model is the embedding model identifier.
	Model string

	// Dimension is the embedder's configured or observed vector dimension.
	// 0 means "not yet known" (e.g. ollama before the first probe).
	Dimension int

	// Distance is the distance metric the embeddings are intended for,
	// e.g. "cosine".
	Distance string

	// Normalize records whether the vectors this embedder returns are
	// L2-normalized.
	Normalize bool

	// Version is an optional revision for the embedding pipeline.
	Version string
}
