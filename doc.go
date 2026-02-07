// Package veclite provides an embeddable vector database for Go applications.
//
// VecLite stores vectors with metadata in a single file, supports HNSW indexing
// for fast approximate nearest-neighbor search, and requires zero external
// dependencies for its core functionality.
//
// # Quick Start
//
// Open a database, create a collection, insert vectors, and search:
//
//	db, err := veclite.Open("data.veclite")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
//
//	coll := db.Collection("embeddings")
//	id, err := coll.Insert(vector, map[string]any{"text": "hello world"})
//	results, err := coll.Search(queryVector, veclite.TopK(10))
//
// # Collections
//
// Collections are namespaced containers for vectors. Create them with options:
//
//	coll, err := db.CreateCollection("docs",
//	    veclite.WithDimension(384),
//	    veclite.WithHNSW(16, 200),
//	    veclite.WithDistanceType(veclite.DistanceCosine),
//	)
//
// # Search
//
// Vector search supports filtering, pagination, and hybrid vector+text search:
//
//	results, err := coll.Search(query,
//	    veclite.TopK(20),
//	    veclite.WithFilter(veclite.Equal("category", "science")),
//	    veclite.Threshold(0.7),
//	)
//
// # Persistence
//
// Data is persisted to a single file using gob encoding with atomic writes.
// Use ":memory:" for an in-memory database:
//
//	db, err := veclite.Open(":memory:")
//
// # Thread Safety
//
// All operations on DB and Collection are safe for concurrent use.
// Multiple goroutines can read and write simultaneously.
package veclite
