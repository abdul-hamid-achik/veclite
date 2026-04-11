package veclite

import (
	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// Storage is the interface for database persistence.
// Implement this interface to provide custom storage backends.
type Storage = storage.Backend

// Snapshot types are re-exported from internal/storage for use
// in custom Storage implementations.
type (
	// DatabaseSnapshot is the serializable state of the database.
	DatabaseSnapshot = storage.DatabaseSnapshot

	// CollectionSnapshot is the serializable state of a collection.
	CollectionSnapshot = storage.CollectionSnapshot

	// RecordSnapshot is the serializable state of a record.
	RecordSnapshot = storage.RecordSnapshot

	// TFEntry stores a document ID and its term frequency for a term.
	TFEntry = storage.TFEntry

	// InvertedIndexSnapshot is the serializable state of the BM25 inverted index.
	InvertedIndexSnapshot = storage.InvertedIndexSnapshot

	// EntitySnapshot is the serializable state of a knowledge graph entity.
	EntitySnapshot = storage.EntitySnapshot

	// RelationshipSnapshot is the serializable state of a knowledge graph relationship.
	RelationshipSnapshot = storage.RelationshipSnapshot

	// GraphSnapshot is the serializable state of a knowledge graph.
	GraphSnapshot = storage.GraphSnapshot

	// EpisodeSnapshot is the serializable state of a single episode.
	EpisodeSnapshot = storage.EpisodeSnapshot

	// EpisodeStoreSnapshot is the serializable state of an episode store.
	EpisodeStoreSnapshot = storage.EpisodeStoreSnapshot

	// TimeRangeSnapshot is the serializable state of a time range.
	TimeRangeSnapshot = storage.TimeRangeSnapshot
)

// NewDatabaseSnapshot creates a new empty database snapshot.
func NewDatabaseSnapshot() *DatabaseSnapshot {
	return storage.NewDatabaseSnapshot()
}

// NewCollectionSnapshot creates a new empty collection snapshot.
func NewCollectionSnapshot(name string, dimension int, distanceType floats.DistanceType) *CollectionSnapshot {
	return storage.NewCollectionSnapshot(name, dimension, distanceType)
}
