// Package hnsw implements the Hierarchical Navigable Small World graph algorithm
// for approximate nearest neighbor search.
package hnsw

import "math"

// Config holds HNSW index parameters.
type Config struct {
	// M is the maximum number of connections per node at layers > 0.
	// Higher M leads to better recall but slower search and more memory.
	// Default: 16, recommended range: 12-48.
	M int

	// Mmax is the maximum number of connections at layer 0.
	// Typically set to 2*M for better recall at the base layer.
	Mmax int

	// EfConstruction is the size of the dynamic candidate list during index construction.
	// Higher values lead to better quality index but slower construction.
	// Default: 200, recommended range: 100-500.
	EfConstruction int

	// EfSearch is the default size of the dynamic candidate list during search.
	// Higher values lead to better recall but slower search.
	// Default: 100, can be overridden per-query.
	EfSearch int

	// ML is the level multiplier used to determine node levels.
	// Typically set to 1/ln(M). Smaller values create shallower graphs.
	ML float64
}

// DefaultConfig returns sensible defaults for HNSW.
func DefaultConfig() Config {
	m := 16
	return Config{
		M:              m,
		Mmax:           m * 2,
		EfConstruction: 200,
		EfSearch:       100,
		ML:             1.0 / math.Log(float64(m)),
	}
}

// NewConfig creates a config with the given M and efConstruction.
// Other values are derived from these.
func NewConfig(m, efConstruction int) Config {
	if m < 2 {
		m = 2
	}
	if efConstruction < m {
		efConstruction = m
	}
	return Config{
		M:              m,
		Mmax:           m * 2,
		EfConstruction: efConstruction,
		EfSearch:       100,
		ML:             1.0 / math.Log(float64(m)),
	}
}

// Validate checks if the config values are sensible.
func (c *Config) Validate() bool {
	return c.M >= 2 &&
		c.Mmax >= c.M &&
		c.EfConstruction >= c.M &&
		c.EfSearch >= 1 &&
		c.ML > 0
}
