package veclite

import (
	"fmt"
	"math/rand"
	"testing"
)

// generateRandomVector creates a random unit vector of the given dimension.
func generateRandomVector(dim int) []float32 {
	vec := make([]float32, dim)
	var sum float32
	for i := range vec {
		vec[i] = rand.Float32()*2 - 1
		sum += vec[i] * vec[i]
	}
	// Normalize to unit length
	if sum > 0 {
		scale := 1.0 / float32(sum)
		for i := range vec {
			vec[i] *= scale
		}
	}
	return vec
}

// setupBenchCollection creates a collection with n vectors of given dimension.
func setupBenchCollection(b *testing.B, n, dim int, useHNSW bool) *Collection {
	b.Helper()

	db, err := Open(":memory:")
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })

	var coll *Collection
	if useHNSW {
		coll, err = db.CreateCollection("bench",
			WithDimension(dim),
			WithDistanceType(DistanceCosine),
			WithHNSW(16, 100),
		)
	} else {
		coll, err = db.CreateCollection("bench",
			WithDimension(dim),
			WithDistanceType(DistanceCosine),
		)
	}
	if err != nil {
		b.Fatalf("Failed to create collection: %v", err)
	}

	// Insert vectors in batches
	batchSize := 1000
	for i := 0; i < n; i += batchSize {
		end := i + batchSize
		if end > n {
			end = n
		}
		count := end - i

		vectors := make([][]float32, count)
		for j := 0; j < count; j++ {
			vectors[j] = generateRandomVector(dim)
		}

		_, err := coll.InsertBatch(vectors, nil)
		if err != nil {
			b.Fatalf("Failed to insert vectors: %v", err)
		}
	}

	return coll
}

// BenchmarkSearchScales benchmarks vector search performance at various scales.
func BenchmarkSearchScales(b *testing.B) {
	dim := 128
	sizes := []int{1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("brute_%d", size), func(b *testing.B) {
			coll := setupBenchCollection(b, size, dim, false)
			query := generateRandomVector(dim)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := coll.Search(query, TopK(10))
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("hnsw_%d", size), func(b *testing.B) {
			coll := setupBenchCollection(b, size, dim, true)
			query := generateRandomVector(dim)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := coll.Search(query, TopK(10))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSearchWithFilters benchmarks search with payload filters.
func BenchmarkSearchWithFilters(b *testing.B) {
	dim := 128
	size := 10000

	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	coll, _ := db.CreateCollection("bench",
		WithDimension(dim),
		WithDistanceType(DistanceCosine),
		WithHNSW(16, 100),
	)

	// Insert with categories
	categories := []string{"cat_a", "cat_b", "cat_c", "cat_d", "cat_e"}
	for i := 0; i < size; i++ {
		vec := generateRandomVector(dim)
		payload := map[string]any{
			"category": categories[i%len(categories)],
			"score":    float64(i % 100),
		}
		_, _ = coll.Insert(vec, payload)
	}

	query := generateRandomVector(dim)

	b.Run("no_filter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.Search(query, TopK(10))
		}
	})

	b.Run("one_filter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.Search(query, TopK(10), WithFilter(Equal("category", "cat_a")))
		}
	})

	b.Run("two_filters", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.Search(query, TopK(10),
				WithFilter(Equal("category", "cat_a")),
				WithFilter(GreaterThan("score", 50.0)),
			)
		}
	})
}

// BenchmarkInsertSingle benchmarks single insertion performance.
func BenchmarkInsertSingle(b *testing.B) {
	dim := 128

	b.Run("brute", func(b *testing.B) {
		db, _ := Open(":memory:")
		defer func() { _ = db.Close() }()
		coll, _ := db.CreateCollection("bench", WithDimension(dim))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vec := generateRandomVector(dim)
			_, _ = coll.Insert(vec, nil)
		}
	})

	b.Run("hnsw", func(b *testing.B) {
		db, _ := Open(":memory:")
		defer func() { _ = db.Close() }()
		coll, _ := db.CreateCollection("bench",
			WithDimension(dim),
			WithHNSW(16, 100),
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vec := generateRandomVector(dim)
			_, _ = coll.Insert(vec, nil)
		}
	})
}

// BenchmarkBatchInsert benchmarks batch insertion performance.
func BenchmarkBatchInsert(b *testing.B) {
	dim := 128
	batchSizes := []int{100, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("brute_%d", batchSize), func(b *testing.B) {
			vectors := make([][]float32, batchSize)
			for i := range vectors {
				vectors[i] = generateRandomVector(dim)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				db, _ := Open(":memory:")
				coll, _ := db.CreateCollection("bench", WithDimension(dim))
				_, _ = coll.InsertBatch(vectors, nil)
				_ = db.Close()
			}
		})

		b.Run(fmt.Sprintf("hnsw_%d", batchSize), func(b *testing.B) {
			vectors := make([][]float32, batchSize)
			for i := range vectors {
				vectors[i] = generateRandomVector(dim)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				db, _ := Open(":memory:")
				coll, _ := db.CreateCollection("bench",
					WithDimension(dim),
					WithHNSW(16, 100),
				)
				_, _ = coll.InsertBatch(vectors, nil)
				_ = db.Close()
			}
		})
	}
}

// BenchmarkHybridSearch benchmarks combined vector + text search.
func BenchmarkHybridSearch(b *testing.B) {
	dim := 128
	size := 5000

	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	coll, _ := db.CreateCollection("bench",
		WithDimension(dim),
		WithDistanceType(DistanceCosine),
		WithHNSW(16, 100),
		WithTextIndex("content"),
	)

	// Insert documents with text content
	words := []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog", "lorem", "ipsum"}
	for i := 0; i < size; i++ {
		vec := generateRandomVector(dim)
		// Generate random content
		content := ""
		for j := 0; j < 10; j++ {
			content += words[rand.Intn(len(words))] + " "
		}
		_, _ = coll.InsertDocument(vec, content, nil)
	}

	query := generateRandomVector(dim)
	textQuery := "quick brown fox"

	b.Run("vector_only", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.Search(query, TopK(10))
		}
	})

	b.Run("text_only", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.TextSearch(textQuery, TopK(10))
		}
	})

	b.Run("hybrid", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.HybridSearch(query, textQuery, TopK(10))
		}
	})
}

// BenchmarkGraphTraverse benchmarks knowledge graph traversal.
func BenchmarkGraphTraverse(b *testing.B) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	kg, _ := db.CreateKnowledgeGraph("bench")

	// Create a graph with entities and relationships
	numEntities := 500
	for i := 0; i < numEntities; i++ {
		_ = kg.AddEntity(Entity{
			ID:   fmt.Sprintf("e%d", i),
			Type: "entity",
			Name: fmt.Sprintf("Entity %d", i),
		})
	}

	// Add relationships (each entity connects to ~5 others)
	relID := 0
	for i := 0; i < numEntities; i++ {
		for j := 0; j < 5; j++ {
			target := rand.Intn(numEntities)
			if target != i {
				_ = kg.AddRelationship(Relationship{
					ID:       fmt.Sprintf("r%d", relID),
					SourceID: fmt.Sprintf("e%d", i),
					TargetID: fmt.Sprintf("e%d", target),
					Type:     "related",
					Weight:   0.5,
				})
				relID++
			}
		}
	}

	startIDs := []string{"e0"}

	b.Run("depth_1", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = kg.Traverse(startIDs, TraversalConfig{MaxDepth: 1, MaxNodes: 100})
		}
	})

	b.Run("depth_2", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = kg.Traverse(startIDs, TraversalConfig{MaxDepth: 2, MaxNodes: 100})
		}
	})

	b.Run("depth_3", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = kg.Traverse(startIDs, TraversalConfig{MaxDepth: 3, MaxNodes: 100})
		}
	})
}

// BenchmarkMemoryOperations benchmarks agent memory operations.
func BenchmarkMemoryOperations(b *testing.B) {
	dim := 384 // Typical embedding dimension

	b.Run("remember", func(b *testing.B) {
		db, _ := Open(":memory:")
		defer func() { _ = db.Close() }()
		coll, _ := db.CreateCollection("memories",
			WithDimension(dim),
			WithHNSW(16, 100),
			WithTextIndex("content"),
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vec := generateRandomVector(dim)
			_, _ = coll.InsertWithOptions(vec, nil, WithContentOption("test memory content"), WithImportance(0.5))
		}
	})

	b.Run("recall", func(b *testing.B) {
		db, _ := Open(":memory:")
		defer func() { _ = db.Close() }()
		coll, _ := db.CreateCollection("memories",
			WithDimension(dim),
			WithHNSW(16, 100),
			WithTextIndex("content"),
		)

		// Pre-populate
		for i := 0; i < 5000; i++ {
			vec := generateRandomVector(dim)
			_, _ = coll.InsertWithOptions(vec, nil, WithContentOption("test memory"), WithImportance(rand.Float32()))
		}

		query := generateRandomVector(dim)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.Search(query, TopK(10), WithFilter(ImportanceAbove(0.5)))
		}
	})
}

// BenchmarkConsolidation benchmarks memory cluster detection.
func BenchmarkConsolidation(b *testing.B) {
	dim := 128
	size := 500

	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	coll, _ := db.CreateCollection("memories",
		WithDimension(dim),
		WithDistanceType(DistanceCosine),
	)

	// Insert clustered data (groups of similar vectors)
	for i := 0; i < size; i++ {
		base := generateRandomVector(dim)
		// Add noise to create clusters
		for j := 0; j < 3; j++ {
			vec := make([]float32, dim)
			for k := range vec {
				vec[k] = base[k] + (rand.Float32()-0.5)*0.1
			}
			_, _ = coll.Insert(vec, map[string]any{"group": i})
		}
	}

	b.Run("find_clusters", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = coll.FindSimilarClusters(ConsolidationConfig{
				SimilarityThreshold: 0.9,
				MinGroupSize:        2,
				MaxGroupSize:        10,
			})
		}
	})
}

// BenchmarkEpisodeDetection benchmarks episode detection performance.
func BenchmarkEpisodeDetection(b *testing.B) {
	dim := 128
	size := 500

	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	coll, _ := db.CreateCollection("memories",
		WithDimension(dim),
	)

	// Insert records
	for i := 0; i < size; i++ {
		vec := generateRandomVector(dim)
		_, _ = coll.Insert(vec, nil)
	}

	es, _ := db.CreateEpisodeStore("memories")

	b.Run("detect", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = es.DetectEpisodes(EpisodeConfig{
				MinRecords: 2,
			})
		}
	})
}

// BenchmarkInsertDurability compares the cost of one durable insert across
// the three durability modes: none (memory until Sync), WAL append, and
// full-snapshot save per write.
func BenchmarkInsertDurability(b *testing.B) {
	const dim = 128
	const preload = 1000

	modes := []struct {
		name string
		opts []Option
	}{
		{"none", nil},
		{"wal", []Option{WithWAL(true)}},
		{"syncOnWrite", []Option{WithSyncOnWrite(true)}},
	}

	for _, mode := range modes {
		b.Run(mode.name, func(b *testing.B) {
			path := b.TempDir() + "/bench.veclite"
			db, err := Open(path, mode.opts...)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = db.Close() }()

			coll, err := db.CreateCollection("bench", WithDimension(dim), WithHNSW(16, 200))
			if err != nil {
				b.Fatal(err)
			}
			// Preload so syncOnWrite pays a realistic snapshot cost.
			for i := 0; i < preload; i++ {
				if _, err := coll.Insert(generateRandomVector(dim), map[string]any{"i": i}); err != nil {
					b.Fatal(err)
				}
			}
			if err := db.Sync(); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := coll.Insert(generateRandomVector(dim), map[string]any{"i": i}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
