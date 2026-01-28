// Example batch demonstrates batch operations, upsert, and iteration.
package main

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/abdul-hamid-achik/veclite"
)

func main() {
	db, err := veclite.Open(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("vectors")

	// Batch insert
	fmt.Println("=== Batch Insert ===")
	rng := rand.New(rand.NewSource(42))
	batchSize := 100
	vectors := make([][]float32, batchSize)
	payloads := make([]map[string]any, batchSize)
	for i := 0; i < batchSize; i++ {
		vectors[i] = make([]float32, 8)
		for j := range vectors[i] {
			vectors[i][j] = rng.Float32()
		}
		payloads[i] = map[string]any{
			"index": i,
			"group": fmt.Sprintf("group-%d", i%5),
		}
	}

	ids, err := coll.InsertBatch(vectors, payloads)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Inserted %d vectors (IDs: %d-%d)\n", len(ids), ids[0], ids[len(ids)-1])

	// Upsert by ID (update existing)
	fmt.Println("\n=== Upsert by ID ===")
	newVec := make([]float32, 8)
	for j := range newVec {
		newVec[j] = rng.Float32()
	}
	upsertID, err := coll.Upsert(ids[0], newVec, map[string]any{"index": 0, "updated": true})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Upserted record %d\n", upsertID)

	// Upsert by key field
	fmt.Println("\n=== Upsert by Key ===")
	keyVec := make([]float32, 8)
	for j := range keyVec {
		keyVec[j] = rng.Float32()
	}
	keyID, inserted, err := coll.UpsertByKey("group", "group-0",
		keyVec, map[string]any{"group": "group-0", "key_upserted": true})
	if err != nil {
		log.Fatal(err)
	}
	action := "updated"
	if inserted {
		action = "inserted"
	}
	fmt.Printf("Upserted by key: ID=%d (%s)\n", keyID, action)

	// Iterate with pagination
	fmt.Println("\n=== Iterate (offset=0, limit=5) ===")
	it := coll.Iterate(veclite.IterOffset(0), veclite.IterLimit(5))
	for {
		rec, ok := it.Next()
		if !ok {
			break
		}
		fmt.Printf("  ID=%d group=%v\n", rec.ID, rec.Payload["group"])
	}
	it.Close()

	// ForEach (stop after 3)
	fmt.Println("\n=== ForEach (stop after 3) ===")
	count := 0
	coll.ForEach(func(r *veclite.Record) bool {
		fmt.Printf("  ID=%d group=%v\n", r.ID, r.Payload["group"])
		count++
		return count < 3
	})

	// Delete where
	fmt.Println("\n=== Delete Where group=group-4 ===")
	deleted, err := coll.DeleteWhere(veclite.Equal("group", "group-4"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Deleted %d records\n", deleted)
	fmt.Printf("Remaining: %d records\n", coll.Count())

	// Search with pagination
	fmt.Println("\n=== Search with Offset/Limit ===")
	query := make([]float32, 8)
	for j := range query {
		query[j] = rng.Float32()
	}

	// First page
	results, err := coll.Search(query, veclite.TopK(3))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Page 1 (top 3):")
	for _, r := range results {
		fmt.Printf("  ID=%d score=%.4f\n", r.Record.ID, r.Score)
	}

	// Second page
	results, err = coll.Search(query, veclite.TopK(3), veclite.WithOffset(3))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Page 2 (offset=3, top 3):")
	for _, r := range results {
		fmt.Printf("  ID=%d score=%.4f\n", r.Record.ID, r.Score)
	}
}
