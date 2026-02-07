package veclite

import (
	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/hnsw"
)

// hnswIndex wraps the internal HNSW index to implement the Index interface.
// This type is unexported because consumers never create indexes directly;
// use WithHNSW or WithHNSWConfig collection options instead.
type hnswIndex struct {
	idx *hnsw.Index
}

// newHNSWIndex creates a new HNSW index with the given parameters.
func newHNSWIndex(dimension int, distanceType floats.DistanceType, m, efConstruction int) *hnswIndex {
	config := hnsw.NewConfig(m, efConstruction)
	return &hnswIndex{
		idx: hnsw.New(config, dimension, distanceType),
	}
}

// Insert adds a vector with the given ID to the index.
func (h *hnswIndex) Insert(id uint64, vector []float32) error {
	return h.idx.Insert(id, vector)
}

// Delete removes a vector from the index (soft delete).
func (h *hnswIndex) Delete(id uint64) error {
	return h.idx.Delete(id)
}

// hardDelete removes a vector completely from the index.
// Needed for update operations where we re-insert with the same ID.
func (h *hnswIndex) hardDelete(id uint64) error {
	return h.idx.HardDelete(id)
}

// Search finds the k nearest neighbors to the query vector.
func (h *hnswIndex) Search(query []float32, k int) ([]IndexResult, error) {
	results, err := h.idx.Search(query, k)
	if err != nil {
		return nil, err
	}
	return convertHNSWResults(results), nil
}

// SearchWithEf searches with a custom ef parameter.
func (h *hnswIndex) SearchWithEf(query []float32, k int, ef int) ([]IndexResult, error) {
	results, err := h.idx.SearchWithEf(query, k, ef)
	if err != nil {
		return nil, err
	}
	return convertHNSWResults(results), nil
}

// Count returns the number of vectors in the index.
func (h *hnswIndex) Count() int {
	return h.idx.Count()
}

// Type returns "hnsw".
func (h *hnswIndex) Type() string {
	return string(IndexTypeHNSW)
}

// stats returns statistics about the index.
func (h *hnswIndex) stats() hnsw.IndexStats {
	return h.idx.Stats()
}

// internal returns the underlying HNSW index (for serialization).
func (h *hnswIndex) internal() *hnsw.Index {
	return h.idx
}

// convertHNSWResults converts HNSW search results to IndexResults.
func convertHNSWResults(results []hnsw.SearchResult) []IndexResult {
	out := make([]IndexResult, len(results))
	for i, r := range results {
		out[i] = IndexResult{
			ID:       r.ID,
			Distance: r.Distance,
		}
	}
	return out
}

// Ensure hnswIndex implements Index.
var _ Index = (*hnswIndex)(nil)
