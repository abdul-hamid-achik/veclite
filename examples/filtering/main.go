// Example filtering demonstrates VecLite's rich filter expressions.
package main

import (
	"fmt"
	"log"

	"github.com/abdul-hamid-achik/veclite"
)

func main() {
	db, err := veclite.Open(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("docs")

	// Insert documents with varied metadata
	docs := []struct {
		vector  []float32
		payload map[string]any
	}{
		{[]float32{0.1, 0.2, 0.3}, map[string]any{"file": "main.go", "type": "code", "lang": "go", "lines": 150}},
		{[]float32{0.2, 0.3, 0.4}, map[string]any{"file": "utils.go", "type": "code", "lang": "go", "lines": 80}},
		{[]float32{0.3, 0.4, 0.5}, map[string]any{"file": "index.ts", "type": "code", "lang": "typescript", "lines": 200}},
		{[]float32{0.4, 0.5, 0.6}, map[string]any{"file": "README.md", "type": "docs", "lang": "markdown", "lines": 50}},
		{[]float32{0.5, 0.6, 0.7}, map[string]any{"file": "test_main.py", "type": "test", "lang": "python", "lines": 120}},
		{[]float32{0.6, 0.7, 0.8}, map[string]any{"file": "config.yaml", "type": "config", "lines": 30}},
	}

	for _, doc := range docs {
		if _, err := coll.Insert(doc.vector, doc.payload); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("Inserted %d documents\n\n", len(docs))

	// Exact match
	fmt.Println("=== Equal filter: type=code ===")
	records, _ := coll.Find(veclite.Equal("type", "code"))
	for _, r := range records {
		fmt.Printf("  %s (%s)\n", r.Payload["file"], r.Payload["lang"])
	}

	// Glob pattern
	fmt.Println("\n=== Glob filter: file=*.go ===")
	records, _ = coll.Find(veclite.Glob("file", "*.go"))
	for _, r := range records {
		fmt.Printf("  %s\n", r.Payload["file"])
	}

	// Prefix
	fmt.Println("\n=== Prefix filter: file starts with 'test' ===")
	records, _ = coll.Find(veclite.Prefix("file", "test"))
	for _, r := range records {
		fmt.Printf("  %s\n", r.Payload["file"])
	}

	// Numeric comparison
	fmt.Println("\n=== GreaterThan filter: lines > 100 ===")
	records, _ = coll.Find(veclite.GT("lines", 100))
	for _, r := range records {
		fmt.Printf("  %s (%v lines)\n", r.Payload["file"], r.Payload["lines"])
	}

	// Between
	fmt.Println("\n=== Between filter: 50 <= lines <= 120 ===")
	records, _ = coll.Find(veclite.Between("lines", 50, 120))
	for _, r := range records {
		fmt.Printf("  %s (%v lines)\n", r.Payload["file"], r.Payload["lines"])
	}

	// Field existence
	fmt.Println("\n=== Exists filter: has 'lang' field ===")
	records, _ = coll.Find(veclite.Exists("lang"))
	for _, r := range records {
		fmt.Printf("  %s (lang=%v)\n", r.Payload["file"], r.Payload["lang"])
	}

	// Composite: AND
	fmt.Println("\n=== AND: type=code AND lang=go ===")
	records, _ = coll.Find(
		veclite.Equal("type", "code"),
		veclite.Equal("lang", "go"),
	)
	for _, r := range records {
		fmt.Printf("  %s\n", r.Payload["file"])
	}

	// OR filter
	fmt.Println("\n=== OR: type=code OR type=test ===")
	records, _ = coll.Find(
		veclite.Or(
			veclite.Equal("type", "code"),
			veclite.Equal("type", "test"),
		),
	)
	for _, r := range records {
		fmt.Printf("  %s (type=%v)\n", r.Payload["file"], r.Payload["type"])
	}

	// NOT filter
	fmt.Println("\n=== NOT: type != code ===")
	records, _ = coll.Find(veclite.Not(veclite.Equal("type", "code")))
	for _, r := range records {
		fmt.Printf("  %s (type=%v)\n", r.Payload["file"], r.Payload["type"])
	}

	// Search with filters
	fmt.Println("\n=== Search with filter: type=code, top 2 ===")
	results, _ := coll.Search(
		[]float32{0.15, 0.25, 0.35},
		veclite.TopK(2),
		veclite.WithFilter(veclite.Equal("type", "code")),
	)
	for i, r := range results {
		fmt.Printf("  %d. %s (score=%.4f)\n", i+1, r.Record.Payload["file"], r.Score)
	}
}
