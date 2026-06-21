// Example hnsw demonstrates HNSW index configuration and performance.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/abdul-hamid-achik/veclite"
)

func main() {
	db, err := veclite.Open(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Create a collection with HNSW indexing
	coll, err := db.CreateCollection("vectors",
		veclite.WithDimension(128),
		veclite.WithDistanceType(veclite.DistanceCosine),
		veclite.WithHNSW(16, 200), // M=16, efConstruction=200
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Created HNSW-indexed collection (dim=128, M=16, efConstruction=200)")

	// Insert random vectors
	rng := rand.New(rand.NewSource(42))
	count := 5000
	fmt.Printf("Inserting %d vectors...\n", count)

	start := time.Now()
	for i := 0; i < count; i++ {
		vec := make([]float32, 128)
		for j := range vec {
			vec[j] = rng.Float32()
		}
		_, err := coll.Insert(vec, map[string]any{"index": i})
		if err != nil {
			log.Fatal(err)
		}
	}
	insertTime := time.Since(start)
	fmt.Printf("Insert time: %v (%.0f vectors/sec)\n", insertTime, float64(count)/insertTime.Seconds())

	// Search benchmark
	queries := 100
	query := make([]float32, 128)
	for j := range query {
		query[j] = rng.Float32()
	}

	// Default ef search
	start = time.Now()
	for i := 0; i < queries; i++ {
		_, err := coll.Search(query, veclite.TopK(10))
		if err != nil {
			log.Fatal(err)
		}
	}
	defaultTime := time.Since(start)
	fmt.Printf("\nDefault efSearch: %v for %d queries (%.0f QPS)\n",
		defaultTime, queries, float64(queries)/defaultTime.Seconds())

	// Higher ef search (better recall)
	start = time.Now()
	for i := 0; i < queries; i++ {
		_, err := coll.Search(query, veclite.TopK(10), veclite.WithEfSearch(200))
		if err != nil {
			log.Fatal(err)
		}
	}
	highEfTime := time.Since(start)
	fmt.Printf("efSearch=200:     %v for %d queries (%.0f QPS)\n",
		highEfTime, queries, float64(queries)/highEfTime.Seconds())

	// Show stats
	stats := coll.Stats()
	fmt.Printf("\nCollection: %d records, index=%s\n", stats.Count, stats.IndexType)
}
