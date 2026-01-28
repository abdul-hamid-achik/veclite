package veclite

import "time"

// Record represents a stored vector with its metadata.
type Record struct {
	// ID is the unique identifier for this record.
	ID uint64

	// Vector is the embedding vector.
	Vector []float32

	// Payload contains arbitrary metadata associated with the vector.
	Payload map[string]any

	// Content is the optional original text content associated with this record.
	// Used for document-oriented storage and automatically indexed by BM25 when text indexing is enabled.
	Content string

	// CreatedAt is when the record was inserted.
	CreatedAt time.Time

	// UpdatedAt is when the record was last updated.
	UpdatedAt time.Time
}

// Clone creates a deep copy of the record.
func (r *Record) Clone() *Record {
	if r == nil {
		return nil
	}

	clone := &Record{
		ID:        r.ID,
		Vector:    make([]float32, len(r.Vector)),
		Content:   r.Content,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}

	copy(clone.Vector, r.Vector)

	if r.Payload != nil {
		clone.Payload = make(map[string]any, len(r.Payload))
		for k, v := range r.Payload {
			clone.Payload[k] = v
		}
	}

	return clone
}

// Result represents a search result with its similarity score.
type Result struct {
	// Record is the matched record.
	Record *Record

	// Score is the similarity/distance score.
	// For cosine/dot: higher is more similar.
	// For euclidean: lower is more similar.
	Score float32
}

// CollectionStats contains statistics about a collection.
type CollectionStats struct {
	// Name is the collection name.
	Name string

	// Count is the number of records in the collection.
	Count int

	// Dimension is the vector dimension (0 if not yet set).
	Dimension int

	// DistanceType is the distance metric used.
	DistanceType string

	// IndexType is the index type (none, hnsw).
	IndexType string
}

// DatabaseStats contains statistics about the database.
type DatabaseStats struct {
	// Path is the database file path (":memory:" for in-memory).
	Path string

	// Collections is the number of collections.
	Collections int

	// TotalRecords is the total number of records across all collections.
	TotalRecords int

	// CollectionStats contains stats for each collection.
	CollectionStats []CollectionStats
}
