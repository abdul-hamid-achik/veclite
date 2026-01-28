package veclite

import (
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/hnsw"
)

// Storage is the interface for database persistence.
type Storage interface {
	// Load reads the database from storage.
	// Returns nil, nil if the database doesn't exist yet.
	Load() (*DatabaseSnapshot, error)

	// Save writes the database to storage.
	Save(snapshot *DatabaseSnapshot) error

	// Close releases any resources held by the storage.
	Close() error
}

// DatabaseSnapshot is the serializable state of the database.
type DatabaseSnapshot struct {
	// Version is the file format version.
	Version uint32

	// Collections maps collection names to their snapshots.
	Collections map[string]*CollectionSnapshot

	// CreatedAt is when the database was created.
	CreatedAt time.Time

	// UpdatedAt is when the database was last modified.
	UpdatedAt time.Time
}

// CollectionSnapshot is the serializable state of a collection.
type CollectionSnapshot struct {
	// Name is the collection name.
	Name string

	// Dimension is the vector dimension.
	Dimension int

	// DistanceType is the distance metric.
	DistanceType floats.DistanceType

	// NextID is the next record ID to assign.
	NextID uint64

	// Records contains all records in the collection.
	Records []*RecordSnapshot

	// CreatedAt is when the collection was created.
	CreatedAt time.Time

	// UpdatedAt is when the collection was last modified.
	UpdatedAt time.Time

	// IndexType is the type of index (none, hnsw).
	IndexType IndexType

	// HNSWConfig holds the HNSW configuration (if IndexType is hnsw).
	HNSWConfig *HNSWConfig

	// HNSWSnapshot holds the HNSW index state (if IndexType is hnsw).
	HNSWSnapshot *hnsw.Snapshot

	// TextIndexSnapshot holds the BM25 text index state (if text indexing is enabled).
	TextIndexSnapshot *InvertedIndexSnapshot
}

// RecordSnapshot is the serializable state of a record.
type RecordSnapshot struct {
	// ID is the record's unique identifier.
	ID uint64

	// Vector is the embedding vector.
	Vector []float32

	// Payload contains arbitrary metadata.
	Payload map[string]any

	// Content is the optional text content.
	Content string

	// CreatedAt is when the record was inserted.
	CreatedAt time.Time

	// UpdatedAt is when the record was last updated.
	UpdatedAt time.Time
}

// NewDatabaseSnapshot creates a new empty database snapshot.
func NewDatabaseSnapshot() *DatabaseSnapshot {
	now := time.Now()
	return &DatabaseSnapshot{
		Version:     1,
		Collections: make(map[string]*CollectionSnapshot),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewCollectionSnapshot creates a new empty collection snapshot.
func NewCollectionSnapshot(name string, dimension int, distanceType floats.DistanceType) *CollectionSnapshot {
	now := time.Now()
	return &CollectionSnapshot{
		Name:         name,
		Dimension:    dimension,
		DistanceType: distanceType,
		NextID:       1,
		Records:      make([]*RecordSnapshot, 0),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
