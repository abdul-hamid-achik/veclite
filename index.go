package veclite

// Index is the interface for vector search indexes.
// Implementations can provide different algorithms (brute-force, HNSW, etc.).
type Index interface {
	// Insert adds a vector with the given ID to the index.
	Insert(id uint64, vector []float32) error

	// Delete removes a vector from the index.
	Delete(id uint64) error

	// Search finds the k nearest neighbors to the query vector.
	// Returns IDs and distances/similarities.
	Search(query []float32, k int) ([]IndexResult, error)

	// SearchWithEf searches with a custom ef parameter (for HNSW).
	// For indexes that don't support ef, this should behave like Search.
	SearchWithEf(query []float32, k int, ef int) ([]IndexResult, error)

	// Count returns the number of vectors in the index.
	Count() int

	// Type returns the index type name.
	Type() string
}

// IndexResult represents a search result from an index.
type IndexResult struct {
	ID       uint64
	Distance float32
}

// IndexType represents the type of index.
type IndexType string

const (
	// IndexTypeNone means no index (brute force search).
	IndexTypeNone IndexType = "none"

	// IndexTypeHNSW means HNSW approximate nearest neighbor index.
	IndexTypeHNSW IndexType = "hnsw"
)
