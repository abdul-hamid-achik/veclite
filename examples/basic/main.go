// Example basic demonstrates the core VecLite operations:
// open a database, insert vectors, search, and close.
package main

import (
	"fmt"
	"log"

	"github.com/abdul-hamid-achik/veclite"
)

func main() {
	// Open an in-memory database (use a file path for persistence)
	db, err := veclite.Open(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Get or create a collection
	coll := db.Collection("embeddings")

	// Insert vectors with metadata
	id1, err := coll.Insert(
		[]float32{0.1, 0.2, 0.3, 0.4},
		map[string]any{"file": "main.go", "type": "code"},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Inserted record %d\n", id1)

	id2, err := coll.Insert(
		[]float32{0.2, 0.3, 0.4, 0.5},
		map[string]any{"file": "utils.go", "type": "code"},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Inserted record %d\n", id2)

	id3, err := coll.Insert(
		[]float32{0.9, 0.8, 0.7, 0.6},
		map[string]any{"file": "README.md", "type": "docs"},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Inserted record %d\n", id3)

	// Search for similar vectors
	results, err := coll.Search(
		[]float32{0.15, 0.25, 0.35, 0.45},
		veclite.TopK(2),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nSearch results (top %d):\n", len(results))
	for i, r := range results {
		fmt.Printf("  %d. ID=%d score=%.4f payload=%v\n",
			i+1, r.Record.ID, r.Score, r.Record.Payload)
	}

	// Get a specific record
	record, err := coll.Get(id1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nRecord %d: vector=%v payload=%v\n", record.ID, record.Vector, record.Payload)

	// Collection stats
	stats := coll.Stats()
	fmt.Printf("\nCollection '%s': %d records, dimension=%d\n",
		stats.Name, stats.Count, stats.Dimension)
}
