package veclite

import (
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/hnsw"
)

// HNSWStats contains statistics about an HNSW index.
// This is the public wrapper for internal index statistics.
type HNSWStats struct {
	// NodeCount is the number of vectors in the index.
	NodeCount int `json:"node_count"`
	// MaxLevel is the highest level in the HNSW graph.
	MaxLevel int `json:"max_level"`
	// EntryPointID is the ID of the entry point node.
	EntryPointID uint64 `json:"entry_point_id"`
}

// SearchExplanation provides details about how a search was performed.
type SearchExplanation struct {
	// Results contains the search results.
	Results []Result

	// IndexType is the type of index used (none, hnsw).
	IndexType string

	// NodesVisited is the number of nodes visited during search.
	// Only populated for HNSW searches.
	NodesVisited int

	// LayersVisited is the number of HNSW layers visited.
	// Only populated for HNSW searches.
	LayersVisited int

	// Duration is how long the search took.
	Duration time.Duration

	// BruteForce indicates whether brute-force search was used.
	BruteForce bool
}

// SearchExplain performs a search and returns detailed statistics.
func (c *Collection) SearchExplain(query []float32, opts ...SearchOption) (*SearchExplanation, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.dimension > 0 && len(query) != c.dimension {
		return nil, &DimensionError{Expected: c.dimension, Got: len(query)}
	}

	// Apply options
	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	start := time.Now()
	explanation := &SearchExplanation{
		IndexType: string(c.indexType),
	}

	// Use HNSW index if available and no filters
	if c.index != nil && len(config.filters) == 0 && config.threshold == nil {
		results, stats, err := c.searchWithIndexExplain(query, config)
		if err != nil {
			return nil, err
		}
		explanation.Results = config.applyPagination(results)
		explanation.NodesVisited = stats.NodesVisited
		explanation.LayersVisited = stats.LayersVisited
		explanation.BruteForce = false
	} else {
		results, err := c.searchBruteForce(query, config)
		if err != nil {
			return nil, err
		}
		explanation.Results = config.applyPagination(results)
		explanation.NodesVisited = len(c.records)
		explanation.BruteForce = true
	}

	explanation.Duration = time.Since(start)
	return explanation, nil
}

// searchWithIndexExplain performs HNSW search with statistics.
func (c *Collection) searchWithIndexExplain(query []float32, config *searchConfig) ([]Result, hnsw.SearchStats, error) {
	hi, ok := c.index.(*hnswIndex)
	if !ok {
		results, err := c.searchBruteForce(query, config)
		return results, hnsw.SearchStats{}, err
	}
	if hi.Count() == 0 {
		return nil, hnsw.SearchStats{}, nil
	}

	// Determine ef parameter
	ef := config.efSearch
	if ef == 0 && c.hnswConfig != nil {
		ef = c.hnswConfig.EfSearch
	}
	if ef == 0 {
		ef = 100
	}

	indexResults, stats, err := hi.internal().SearchWithStats(query, config.effectiveTopK(), ef)
	if err != nil {
		return nil, stats, err
	}

	// Convert index results to full results with records
	results := make([]Result, 0, len(indexResults))
	for _, ir := range indexResults {
		record, ok := c.records[ir.ID]
		if !ok {
			continue
		}
		results = append(results, Result{
			Record: config.cloneRecordForResult(record),
			Score:  ir.Distance,
		})
	}

	return results, stats, nil
}

// IndexStats returns statistics about the collection's HNSW index.
// Returns nil if no HNSW index is configured.
func (c *Collection) IndexStats() *HNSWStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.index == nil {
		return nil
	}

	hi, ok := c.index.(*hnswIndex)
	if !ok {
		return nil
	}

	stats := hi.stats()
	return &HNSWStats{
		NodeCount:    stats.NodeCount,
		MaxLevel:     stats.MaxLevel,
		EntryPointID: stats.EntryPoint,
	}
}
