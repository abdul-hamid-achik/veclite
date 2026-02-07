package veclite

import (
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

	// InvertedIndexSnapshot is the serializable state of the BM25 inverted index.
	InvertedIndexSnapshot = storage.InvertedIndexSnapshot
)

// NewDatabaseSnapshot creates a new empty database snapshot.
var NewDatabaseSnapshot = storage.NewDatabaseSnapshot

// NewCollectionSnapshot creates a new empty collection snapshot.
var NewCollectionSnapshot = storage.NewCollectionSnapshot
