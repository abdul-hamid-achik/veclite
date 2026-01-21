package veclite

import (
	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// Option configures the database.
type Option interface {
	apply(*dbConfig)
}

// dbConfig holds database configuration.
type dbConfig struct {
	syncOnWrite bool
	readOnly    bool
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
	dimension    int
	distanceType floats.DistanceType
}

// defaultCollectionConfig returns the default collection configuration.
func defaultCollectionConfig() *collectionConfig {
	return &collectionConfig{
		dimension:    0, // 0 means auto-detect on first insert
		distanceType: floats.DistanceCosine,
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
