package veclite

import (
	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// Re-export distance types for external use.
type DistanceType = floats.DistanceType

const (
	// DistanceCosine uses cosine similarity (higher = more similar).
	DistanceCosine = floats.DistanceCosine
	// DistanceDot uses dot product (higher = more similar).
	DistanceDot = floats.DistanceDot
	// DistanceEuclidean uses Euclidean distance (lower = more similar).
	DistanceEuclidean = floats.DistanceEuclidean
)

// Option configures the database.
type Option interface {
	apply(*dbConfig)
}

// dbConfig holds database configuration.
type dbConfig struct {
	syncOnWrite bool
	readOnly    bool
	logger      Logger
}

// HNSWConfig holds HNSW index configuration.
type HNSWConfig struct {
	// M is the maximum number of connections per node.
	M int
	// EfConstruction is the size of the candidate list during index construction.
	EfConstruction int
	// EfSearch is the default size of the candidate list during search.
	EfSearch int
}

// defaultDBConfig returns the default database configuration.
func defaultDBConfig() *dbConfig {
	return &dbConfig{
		syncOnWrite: false,
		readOnly:    false,
	}
}

// dbOptionFunc is a function adapter for Option.
type dbOptionFunc func(*dbConfig)

func (f dbOptionFunc) apply(c *dbConfig) {
	f(c)
}

// WithSyncOnWrite enables automatic sync after each write operation.
// This is slower but ensures durability.
func WithSyncOnWrite(enabled bool) Option {
	return dbOptionFunc(func(c *dbConfig) {
		c.syncOnWrite = enabled
	})
}

// WithReadOnly opens the database in read-only mode.
// Write operations will return an error.
func WithReadOnly(enabled bool) Option {
	return dbOptionFunc(func(c *dbConfig) {
		c.readOnly = enabled
	})
}

// CollectionOption configures a collection.
type CollectionOption interface {
	apply(*collectionConfig)
}

// collectionConfig holds collection configuration.
type collectionConfig struct {
	dimension       int
	distanceType    floats.DistanceType
	indexType       IndexType
	hnswConfig      *HNSWConfig
	textIndexFields []string
	embedder        Embedder
}

// defaultCollectionConfig returns the default collection configuration.
func defaultCollectionConfig() *collectionConfig {
	return &collectionConfig{
		dimension:    0, // 0 means auto-detect on first insert
		distanceType: floats.DistanceCosine,
		indexType:    IndexTypeNone,
		hnswConfig:   nil,
	}
}

// collectionOptionFunc is a function adapter for CollectionOption.
type collectionOptionFunc func(*collectionConfig)

func (f collectionOptionFunc) apply(c *collectionConfig) {
	f(c)
}

// WithDimension sets the vector dimension for the collection.
// If set, all vectors must match this dimension.
// If not set (0), the dimension is determined by the first insert.
func WithDimension(dim int) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		if dim > 0 {
			c.dimension = dim
		}
	})
}

// WithDistanceType sets the distance metric for the collection.
// Default is cosine similarity.
func WithDistanceType(t floats.DistanceType) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		c.distanceType = t
	})
}

// WithHNSW enables HNSW indexing for the collection.
// m is the maximum number of connections per node (default: 16, recommended: 12-48).
// efConstruction is the candidate list size during construction (default: 200).
func WithHNSW(m, efConstruction int) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		if m < 2 {
			m = 16
		}
		if efConstruction < m {
			efConstruction = 200
		}
		c.indexType = IndexTypeHNSW
		c.hnswConfig = &HNSWConfig{
			M:              m,
			EfConstruction: efConstruction,
			EfSearch:       100,
		}
	})
}

// WithHNSWConfig enables HNSW indexing with custom configuration.
func WithHNSWConfig(config HNSWConfig) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		c.indexType = IndexTypeHNSW
		c.hnswConfig = &config
	})
}

// WithTextIndex enables BM25 full-text indexing on the specified payload fields.
// When enabled, string values in these fields are tokenized and indexed for text search.
// The Content field of records is always indexed when text indexing is enabled.
func WithTextIndex(fields ...string) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		c.textIndexFields = fields
	})
}

// WithEmbedder sets an auto-embedding plugin for the collection.
// When set, InsertText and SearchText methods become available.
func WithEmbedder(e Embedder) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		c.embedder = e
	})
}
