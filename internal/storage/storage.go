// Package storage provides persistence backends for VecLite databases.
//
// This is an internal package. Use veclite.Open() to create databases.
package storage

import (
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/hnsw"
)

// Backend is the interface for database persistence.
type Backend interface {
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

	// Metadata contains arbitrary database-level metadata.
	Metadata map[string]any

	// Collections maps collection names to their snapshots.
	Collections map[string]*CollectionSnapshot

	// KnowledgeGraphs maps graph names to their snapshots.
	KnowledgeGraphs map[string]*GraphSnapshot

	// EpisodeStores maps collection names to their episode store snapshots.
	EpisodeStores map[string]*EpisodeStoreSnapshot

	// CreatedAt is when the database was created.
	CreatedAt time.Time

	// UpdatedAt is when the database was last modified.
	UpdatedAt time.Time
}

// CollectionSnapshot is the serializable state of a collection.
type CollectionSnapshot struct {
	// Name is the collection name.
	Name string

	// Metadata contains arbitrary collection-level metadata.
	Metadata map[string]any

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
	IndexType string

	// HNSWConfig holds the HNSW configuration (if IndexType is hnsw).
	HNSWConfig *HNSWConfig

	// HNSWSnapshot holds the HNSW index state (if IndexType is hnsw).
	HNSWSnapshot *hnsw.Snapshot

	// TextIndexSnapshot holds the BM25 text index state (if text indexing is enabled).
	TextIndexSnapshot *InvertedIndexSnapshot
}

// HNSWConfig holds HNSW index configuration.
type HNSWConfig struct {
	// M is the maximum number of connections per node.
	M int
	// EfConstruction is the size of the candidate list during index construction.
	EfConstruction int
	// EfSearch is the default size of the candidate list during search.
	EfSearch int
	// UseHeuristic enables diversity-preserving neighbor selection (recommended).
	UseHeuristic bool
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

	// ExpiresAt is when this record expires. Zero value means never expires.
	ExpiresAt time.Time

	// Importance is a score from 0.0 to 1.0.
	Importance float32

	// AccessCount tracks how many times this record has been accessed.
	AccessCount uint64

	// LastAccessedAt is when this record was last accessed.
	LastAccessedAt time.Time
}

// TFEntry stores a document ID and its term frequency for a term.
type TFEntry struct {
	ID    uint64
	Count int
}

// InvertedIndexSnapshot is the serializable state of the BM25 inverted index.
type InvertedIndexSnapshot struct {
	Postings    map[string][]TFEntry
	DocLengths  map[uint64]int
	TotalDocLen int64
	DocCount    int
	Fields      []string
}

// EntitySnapshot is the serializable state of a knowledge graph entity.
type EntitySnapshot struct {
	ID         string
	Type       string
	Name       string
	Vector     []float32
	Properties map[string]any
}

// RelationshipSnapshot is the serializable state of a knowledge graph relationship.
type RelationshipSnapshot struct {
	ID            string
	SourceID      string
	TargetID      string
	Type          string
	Weight        float32
	Properties    map[string]any
	Bidirectional bool
}

// GraphSnapshot is the serializable state of a knowledge graph.
type GraphSnapshot struct {
	Name          string
	Entities      []EntitySnapshot
	Relationships []RelationshipSnapshot
	Outgoing      map[string][]string
	Incoming      map[string][]string
}

// EpisodeSnapshot is the serializable state of a single episode.
type EpisodeSnapshot struct {
	ID        string
	Title     string
	Vector    []float32
	TimeRange TimeRangeSnapshot
	RecordIDs []uint64
	CreatedAt time.Time
	Metadata  map[string]any
}

// TimeRangeSnapshot is the serializable state of a time range.
type TimeRangeSnapshot struct {
	Start time.Time
	End   time.Time
}

// EpisodeStoreSnapshot is the serializable state of an episode store.
type EpisodeStoreSnapshot struct {
	CollectionName string
	Episodes       []EpisodeSnapshot
}

// NewDatabaseSnapshot creates a new empty database snapshot.
func NewDatabaseSnapshot() *DatabaseSnapshot {
	now := time.Now()
	return &DatabaseSnapshot{
		Version:         3,
		Metadata:        make(map[string]any),
		Collections:     make(map[string]*CollectionSnapshot),
		KnowledgeGraphs: make(map[string]*GraphSnapshot),
		EpisodeStores:   make(map[string]*EpisodeStoreSnapshot),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// NewCollectionSnapshot creates a new empty collection snapshot.
func NewCollectionSnapshot(name string, dimension int, distanceType floats.DistanceType) *CollectionSnapshot {
	now := time.Now()
	return &CollectionSnapshot{
		Name:         name,
		Metadata:     make(map[string]any),
		Dimension:    dimension,
		DistanceType: distanceType,
		NextID:       1,
		Records:      make([]*RecordSnapshot, 0),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
