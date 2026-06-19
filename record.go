package veclite

import "time"

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		cp := make([]any, len(val))
		for i, elem := range val {
			cp[i] = deepCopyValue(elem)
		}
		return cp
	case []string:
		cp := make([]string, len(val))
		copy(cp, val)
		return cp
	case []int:
		cp := make([]int, len(val))
		copy(cp, val)
		return cp
	case []float64:
		cp := make([]float64, len(val))
		copy(cp, val)
		return cp
	case []float32:
		cp := make([]float32, len(val))
		copy(cp, val)
		return cp
	case string, int, int64, float64, float32, bool, uint64, int32, nil:
		return v
	default:
		return v
	}
}

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

	// ExpiresAt is when this record expires. Zero value means never expires.
	ExpiresAt time.Time

	// Importance is a score from 0.0 to 1.0 indicating how important this record is.
	// Higher values make the record more likely to be returned in search results.
	Importance float32

	// AccessCount tracks how many times this record has been accessed via search.
	AccessCount uint64

	// LastAccessedAt is when this record was last accessed via search.
	LastAccessedAt time.Time
}

// Clone creates a deep copy of the record.
func (r *Record) Clone() *Record {
	if r == nil {
		return nil
	}

	clone := &Record{
		ID:             r.ID,
		Vector:         make([]float32, len(r.Vector)),
		Content:        r.Content,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		ExpiresAt:      r.ExpiresAt,
		Importance:     r.Importance,
		AccessCount:    r.AccessCount,
		LastAccessedAt: r.LastAccessedAt,
	}

	copy(clone.Vector, r.Vector)

	if r.Payload != nil {
		clone.Payload = deepCopyMap(r.Payload)
	}

	return clone
}

// IsExpired returns true if the record has a TTL set and has expired.
func (r *Record) IsExpired() bool {
	if r.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(r.ExpiresAt)
}

// HasTTL returns true if the record has an expiration time set.
func (r *Record) HasTTL() bool {
	return !r.ExpiresAt.IsZero()
}

// TTL returns the remaining time until expiration.
// Returns 0 if no TTL is set or if the record has already expired.
func (r *Record) TTL() time.Duration {
	if r.ExpiresAt.IsZero() {
		return 0
	}
	remaining := time.Until(r.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
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

	// VectorCount is the number of records with a vector.
	VectorCount int

	// TextOnlyCount is the number of records without a vector.
	TextOnlyCount int

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
