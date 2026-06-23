package veclite

import (
	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
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
	// DistanceEuclideanSquared uses squared Euclidean distance (lower = more similar).
	// Faster than Euclidean since it avoids sqrt.
	DistanceEuclideanSquared = floats.DistanceEuclideanSquared
)

// Option configures the database.
type Option interface {
	apply(*dbConfig)
}

// dbConfig holds database configuration.
type dbConfig struct {
	syncOnWrite bool
	readOnly    bool
	sharedRead  bool
	logger      Logger
}

// HNSWConfig holds HNSW index configuration.
type HNSWConfig = storage.HNSWConfig

// defaultDBConfig returns the default database configuration.
func defaultDBConfig() *dbConfig {
	return &dbConfig{
		syncOnWrite: false,
		readOnly:    false,
		sharedRead:  false,
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

// WithSharedRead enables a shared file lock when opening a read-only database,
// allowing multiple processes to open the same database file simultaneously for
// read-only access. This is useful for scenarios where one process writes (e.g.
// an indexer) while other processes read (e.g. search tools).
//
// SharedRead requires ReadOnly — opening a writable database with a shared
// lock would risk data loss from concurrent full-snapshot saves. If SharedRead
// is enabled without ReadOnly, Open returns an error.
//
// Readers opened with SharedRead see a point-in-time snapshot taken at Open.
// Call Reload() to pick up writes from other processes.
func WithSharedRead(enabled bool) Option {
	return dbOptionFunc(func(c *dbConfig) {
		c.sharedRead = enabled
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
	memoryConfig    *MemoryConfig
	vectorSpaces    []VectorSpaceConfig
	profile         *EmbeddingProfile
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
			UseHeuristic:   true,
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

// WithVectorSpace declares an additional named vector space at collection
// creation time. May be passed multiple times to declare several spaces. The
// reserved DefaultVectorSpace name is rejected (the default space always exists).
// Equivalent to calling Collection.AddVectorSpace after creation.
func WithVectorSpace(config VectorSpaceConfig) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		c.vectorSpaces = append(c.vectorSpaces, config)
	})
}

// WithEmbeddingProfile attaches a first-class embedding profile to the
// collection's default vector space. Vectors inserted into the default space are
// then validated against the profile's declared dimension.
func WithEmbeddingProfile(profile EmbeddingProfile) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		p := profile
		c.profile = &p
		if c.dimension == 0 && profile.Dimension > 0 {
			c.dimension = profile.Dimension
		}
		if profile.Distance != "" {
			c.distanceType = profile.Distance
		}
	})
}
