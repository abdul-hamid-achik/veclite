package veclite

import (
	"fmt"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// DefaultVectorSpace is the reserved name of the implicit vector space backed by
// Record.Vector and the collection's primary dimension/distance/index. Every
// collection has this space, including those created before named vector spaces
// existed. It cannot be removed or redeclared with AddVectorSpace.
const DefaultVectorSpace = "default"

// VectorSpaceConfig declares a named vector space on a collection.
//
// A vector space is an independent index over one named embedding per record:
// its own dimension, distance metric, and optional HNSW index. One logical
// record (see RecordInput) can carry vectors in several spaces at once, for
// example a "text" embedding and an "image" embedding for the same item.
//
// Apps still own embedding generation; VecLite only stores and searches the
// vectors the app provides for each space.
type VectorSpaceConfig struct {
	// Name uniquely identifies the space within its collection. Required, and
	// must not be the reserved DefaultVectorSpace name.
	Name string

	// Dimension fixes the vector length for this space. 0 auto-detects from the
	// first inserted vector.
	Dimension int

	// Distance is the distance metric for this space. Empty defaults to cosine.
	Distance DistanceType

	// Modality is an optional free-form hint such as "text", "image", or "audio".
	Modality string

	// Provider and Model record the embedding source. They are advisory metadata
	// used by embedding-profile compatibility checks.
	Provider string
	Model    string

	// HNSW, when non-nil, enables an HNSW index for this space. Nil uses
	// brute-force search.
	HNSW *HNSWConfig

	// Profile, when non-nil, attaches a first-class embedding profile to the
	// space and enables compatibility validation on insert.
	Profile *EmbeddingProfile
}

// RecordInput is one logical record that may carry vectors in several named
// vector spaces simultaneously, plus optional text content and payload.
//
// Keys of Vectors are vector-space names. The reserved key DefaultVectorSpace
// (or the empty string) targets the default space (Record.Vector). Spaces other
// than "default" must already be declared via AddVectorSpace. A record may omit
// any space; it is then absent from that space's index. A record with no vectors
// at all behaves like a text-only document.
type RecordInput struct {
	// ID selects the record to upsert. 0 assigns a fresh auto-incremented ID.
	ID uint64

	// Content is optional text indexed by BM25 when text indexing is enabled.
	Content string

	// Payload is arbitrary metadata stored with the record.
	Payload map[string]any

	// Vectors maps vector-space name to embedding. See the type doc for the
	// reserved "default" key.
	Vectors map[string][]float32
}

// EmbeddingProfile is a first-class description of how an embedding was produced.
//
// It is the typed replacement for storing provider/model details loosely in
// metadata. Attaching a profile to a collection or vector space lets VecLite
// reject vectors that do not match the declared dimension (and, between two
// profiles, detect when a model/provider/distance change invalidates an index).
type EmbeddingProfile struct {
	// Provider is the embedding provider, e.g. "openai" or "ollama".
	Provider string

	// Model is the embedding model identifier.
	Model string

	// Dimension is the expected vector dimension. 0 means "unspecified".
	Dimension int

	// Distance is the distance metric the embeddings are intended for.
	Distance DistanceType

	// Normalize records whether vectors are L2-normalized.
	Normalize bool

	// Version is an optional app-defined revision for the embedding pipeline.
	Version string
}

// IsZero reports whether the profile carries no information.
func (p EmbeddingProfile) IsZero() bool {
	return p == EmbeddingProfile{}
}

// Compatible reports whether two embedding profiles describe interchangeable
// vectors. It returns nil when compatible, or an error describing the first
// mismatch. Provider, Model, Dimension (when both set), Distance (when both
// set), and Normalize must agree. Version is advisory and never causes an error.
func (p EmbeddingProfile) Compatible(other EmbeddingProfile) error {
	if p.Provider != "" && other.Provider != "" && p.Provider != other.Provider {
		return fmt.Errorf("%w: provider %q vs %q", ErrProfileMismatch, p.Provider, other.Provider)
	}
	if p.Model != "" && other.Model != "" && p.Model != other.Model {
		return fmt.Errorf("%w: model %q vs %q", ErrProfileMismatch, p.Model, other.Model)
	}
	if p.Dimension != 0 && other.Dimension != 0 && p.Dimension != other.Dimension {
		return fmt.Errorf("%w: dimension %d vs %d", ErrProfileMismatch, p.Dimension, other.Dimension)
	}
	if p.Distance != "" && other.Distance != "" && p.Distance != other.Distance {
		return fmt.Errorf("%w: distance %q vs %q", ErrProfileMismatch, p.Distance, other.Distance)
	}
	if p.Normalize != other.Normalize {
		return fmt.Errorf("%w: normalize %v vs %v", ErrProfileMismatch, p.Normalize, other.Normalize)
	}
	return nil
}

// validateVector checks a vector against the profile's declared dimension.
func (p EmbeddingProfile) validateVector(vector []float32) error {
	if p.Dimension != 0 && len(vector) != 0 && len(vector) != p.Dimension {
		return &DimensionError{Expected: p.Dimension, Got: len(vector)}
	}
	return nil
}

func (p *EmbeddingProfile) toSnapshot() *storage.EmbeddingProfileSnapshot {
	if p == nil {
		return nil
	}
	return &storage.EmbeddingProfileSnapshot{
		Provider:  p.Provider,
		Model:     p.Model,
		Dimension: p.Dimension,
		Distance:  p.Distance,
		Normalize: p.Normalize,
		Version:   p.Version,
	}
}

func profileFromSnapshot(s *storage.EmbeddingProfileSnapshot) *EmbeddingProfile {
	if s == nil {
		return nil
	}
	return &EmbeddingProfile{
		Provider:  s.Provider,
		Model:     s.Model,
		Dimension: s.Dimension,
		Distance:  s.Distance,
		Normalize: s.Normalize,
		Version:   s.Version,
	}
}

// VectorSpaceInfo is a read-only view of a vector space's configuration and
// current state, returned by Collection.VectorSpaces and VectorSpace.
type VectorSpaceInfo struct {
	// Name is the vector-space name ("default" for the implicit space).
	Name string

	// Dimension is the configured/auto-detected dimension (0 if not yet known).
	Dimension int

	// Distance is the distance metric used by this space.
	Distance DistanceType

	// Modality is the optional modality hint.
	Modality string

	// Provider and Model record the embedding source, if declared.
	Provider string
	Model    string

	// IndexType is "none" or "hnsw".
	IndexType string

	// VectorCount is the number of records carrying a vector in this space.
	VectorCount int

	// Profile is the space-level embedding profile, if any.
	Profile *EmbeddingProfile
}

// vectorSpace is the internal runtime state of a non-default named vector space.
// The default space is represented by the Collection's own fields and is never
// stored here.
type vectorSpace struct {
	name         string
	dimension    int
	distanceType floats.DistanceType
	distanceFunc floats.DistanceFunc
	higherBetter bool
	modality     string
	provider     string
	model        string
	indexType    IndexType
	hnswConfig   *HNSWConfig
	index        Index
	profile      *EmbeddingProfile
}
