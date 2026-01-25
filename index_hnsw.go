package veclite

import (
	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/hnsw"
)

// HNSWIndex wraps the HNSW index to implement the Index interface.
type HNSWIndex struct {
	idx *hnsw.Index
}

// NewHNSWIndex creates a new HNSW index with the given parameters.
func NewHNSWIndex(dimension int, distanceType floats.DistanceType, m, efConstruction int) *HNSWIndex {
	config := hnsw.NewConfig(m, efConstruction)
	return &HNSWIndex{
		idx: hnsw.New(config, dimension, distanceType),
	}
}

// NewHNSWIndexWithConfig creates a new HNSW index with a custom configuration.
func NewHNSWIndexWithConfig(dimension int, distanceType floats.DistanceType, config hnsw.Config) *HNSWIndex {
	return &HNSWIndex{
		idx: hnsw.New(config, dimension, distanceType),
	}
}

// Insert adds a vector with the given ID to the index.
func (h *HNSWIndex) Insert(id uint64, vector []float32) error {
	return h.idx.Insert(id, vector)
}

// Delete removes a vector from the index (soft delete).
func (h *HNSWIndex) Delete(id uint64) error {
	return h.idx.Delete(id)
}

// HardDelete removes a vector completely from the index.
// This is needed for update operations where we re-insert with the same ID.
func (h *HNSWIndex) HardDelete(id uint64) error {
	return h.idx.HardDelete(id)
}

// Search finds the k nearest neighbors to the query vector.
func (h *HNSWIndex) Search(query []float32, k int) ([]IndexResult, error) {
	results, err := h.idx.Search(query, k)
	if err != nil {
		return nil, err
	}
	return convertHNSWResults(results), nil
}

// SearchWithEf searches with a custom ef parameter.
func (h *HNSWIndex) SearchWithEf(query []float32, k int, ef int) ([]IndexResult, error) {
	results, err := h.idx.SearchWithEf(query, k, ef)
	if err != nil {
		return nil, err
	}
	return convertHNSWResults(results), nil
}

// Count returns the number of vectors in the index.
func (h *HNSWIndex) Count() int {
	return h.idx.Count()
}

// Type returns "hnsw".
func (h *HNSWIndex) Type() string {
	return string(IndexTypeHNSW)
}

// Stats returns statistics about the index.
func (h *HNSWIndex) Stats() hnsw.IndexStats {
	return h.idx.Stats()
}

// Internal returns the underlying HNSW index (for serialization).
func (h *HNSWIndex) Internal() *hnsw.Index {
	return h.idx
}

// SetInternal sets the underlying HNSW index (for deserialization).
func (h *HNSWIndex) SetInternal(idx *hnsw.Index) {
	h.idx = idx
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

// Ensure HNSWIndex implements Index.
var _ Index = (*HNSWIndex)(nil)
