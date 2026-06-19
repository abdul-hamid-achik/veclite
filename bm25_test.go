package veclite

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"hello", "world"}},
		{"  spaces  between  ", []string{"spaces", "between"}},
		{"punctuation! marks? here.", []string{"punctuation", "marks", "here"}},
		{"", nil},
		{"UPPER lower MiXeD", []string{"upper", "lower", "mixed"}},
		{"code: main.go", []string{"code", "main.go"}},
	}

	for _, tt := range tests {
		result := tokenize(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestBM25Scorer(t *testing.T) {
	scorer := newBM25Scorer()
	// BM25 score should be positive for matching terms
	score := scorer.score(1, 10, 10.0, 100, 5)
	if score <= 0 {
		t.Errorf("BM25 score = %v, want > 0", score)
	}

	// Higher TF should give higher score
	scoreLow := scorer.score(1, 10, 10.0, 100, 5)
	scoreHigh := scorer.score(5, 10, 10.0, 100, 5)
	if scoreHigh <= scoreLow {
		t.Errorf("Higher TF should give higher score: %v <= %v", scoreHigh, scoreLow)
	}

	// Rarer terms (lower DF) should have higher IDF
	scoreCommon := scorer.score(1, 10, 10.0, 100, 50)
	scoreRare := scorer.score(1, 10, 10.0, 100, 5)
	if scoreRare <= scoreCommon {
		t.Errorf("Rarer terms should score higher: %v <= %v", scoreRare, scoreCommon)
	}
}

func TestInvertedIndex(t *testing.T) {
	idx := newInvertedIndex([]string{"title", "body"})

	// Index some documents
	idx.indexRecord(1, map[string]any{"title": "Go programming", "body": "Learn Go language"}, "")
	idx.indexRecord(2, map[string]any{"title": "Python tutorial", "body": "Python for beginners"}, "")
	idx.indexRecord(3, map[string]any{"title": "Go patterns", "body": "Advanced Go design patterns"}, "")

	if idx.docCount != 3 {
		t.Errorf("docCount = %d, want 3", idx.docCount)
	}

	// Search for "Go"
	results := idx.search("Go", 10)
	if len(results) != 2 {
		t.Errorf("search('Go') returned %d results, want 2", len(results))
	}
	// Results should be sorted by score (descending)
	if len(results) >= 2 && results[0].score < results[1].score {
		t.Error("Results should be sorted by score descending")
	}

	// Search for "Python"
	results = idx.search("Python", 10)
	if len(results) != 1 {
		t.Errorf("search('Python') returned %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].id != 2 {
		t.Errorf("search('Python') returned ID %d, want 2", results[0].id)
	}

	// Search with topK limit
	results = idx.search("Go", 1)
	if len(results) != 1 {
		t.Errorf("search('Go', topK=1) returned %d results, want 1", len(results))
	}

	// Remove a document
	idx.removeRecord(1)
	if idx.docCount != 2 {
		t.Errorf("docCount after removal = %d, want 2", idx.docCount)
	}

	results = idx.search("Go", 10)
	if len(results) != 1 {
		t.Errorf("search('Go') after removal returned %d results, want 1", len(results))
	}
}

func TestInvertedIndexContent(t *testing.T) {
	idx := newInvertedIndex([]string{})

	// Index with content field instead of payload
	idx.indexRecord(1, nil, "Go programming language is great")
	idx.indexRecord(2, nil, "Python is also a great language")

	results := idx.search("Go programming", 10)
	if len(results) != 1 {
		t.Errorf("search('Go programming') returned %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].id != 1 {
		t.Errorf("expected ID 1, got %d", results[0].id)
	}

	// "great" appears in both
	results = idx.search("great", 10)
	if len(results) != 2 {
		t.Errorf("search('great') returned %d results, want 2", len(results))
	}
}

func TestBM25TermFrequency(t *testing.T) {
	idx := newInvertedIndex([]string{})

	// Doc 1: "go" appears once
	idx.indexRecord(1, nil, "go is a language")
	// Doc 2: "go" appears three times (higher TF)
	idx.indexRecord(2, nil, "go go go is great")

	results := idx.search("go", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// The document with higher TF for "go" should score higher
	if results[0].id != 2 {
		t.Errorf("expected doc 2 (higher TF) to rank first, got doc %d with score %v, doc %d has score %v",
			results[0].id, results[0].score, results[1].id, results[1].score)
	}
	if results[0].score <= results[1].score {
		t.Errorf("higher TF should produce higher score: doc2=%v, doc1=%v", results[0].score, results[1].score)
	}
}

func TestInvertedIndexSnapshot(t *testing.T) {
	idx := newInvertedIndex([]string{"title"})
	idx.indexRecord(1, map[string]any{"title": "Hello World"}, "")
	idx.indexRecord(2, map[string]any{"title": "Goodbye World"}, "")

	// Take snapshot
	snap := idx.snapshot()
	if snap == nil {
		t.Fatal("snapshot is nil")
	}

	// Restore from snapshot
	restored := loadInvertedIndexFromSnapshot(snap)
	if restored.docCount != 2 {
		t.Errorf("restored docCount = %d, want 2", restored.docCount)
	}

	// Verify search works on restored index
	results := restored.search("Hello", 10)
	if len(results) != 1 {
		t.Errorf("search('Hello') on restored index returned %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].id != 1 {
		t.Errorf("expected ID 1, got %d", results[0].id)
	}
}

func TestCollectionTextSearch(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("docs",
		WithDimension(3),
		WithTextIndex("title", "body"),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Insert documents
	_, err = coll.InsertDocument(
		[]float32{0.1, 0.2, 0.3},
		"Go is a great programming language",
		map[string]any{"title": "Go Language", "body": "Fast and efficient"},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertDocument(
		[]float32{0.4, 0.5, 0.6},
		"Python is popular for data science",
		map[string]any{"title": "Python", "body": "Data science and ML"},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.InsertDocument(
		[]float32{0.7, 0.8, 0.9},
		"JavaScript powers the web",
		map[string]any{"title": "JavaScript", "body": "Web development"},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Text search
	results, err := coll.TextSearch("Go programming", TopK(2))
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("TextSearch returned no results")
	}

	// First result should be the Go document
	if results[0].Record.Content != "Go is a great programming language" {
		t.Errorf("Expected Go doc, got: %s", results[0].Record.Content)
	}
}

func TestCollectionTextSearchNoIndex(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll := db.Collection("no-text")
	_, err = coll.Insert([]float32{0.1, 0.2, 0.3}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coll.TextSearch("test")
	if err == nil {
		t.Error("Expected error for TextSearch on collection without text index")
	}
}

func TestTextDocumentSearchAndVectorSearchSkip(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("evidence",
		WithDimension(3),
		WithTextIndex("frame"),
	)
	if err != nil {
		t.Fatal(err)
	}

	vectorID, err := coll.InsertDocument(
		[]float32{1, 0, 0},
		"vector backed document",
		map[string]any{"frame": "frames/frame_0001.png"},
	)
	if err != nil {
		t.Fatal(err)
	}

	textID, err := coll.InsertTextDocument(
		"checkout failed after clicking the submit button",
		map[string]any{"frame": "frames/frame_0002.png"},
	)
	if err != nil {
		t.Fatal(err)
	}

	stats := coll.Stats()
	if stats.Count != 2 || stats.VectorCount != 1 || stats.TextOnlyCount != 1 {
		t.Fatalf("Stats = %#v, want count=2 vector=1 text-only=1", stats)
	}
	if coll.Dimension() != 3 {
		t.Fatalf("Dimension = %d, want 3", coll.Dimension())
	}

	textResults, err := coll.TextSearch("checkout submit", TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(textResults) == 0 || textResults[0].Record.ID != textID {
		t.Fatalf("TextSearch returned %#v, want first ID %d", textResults, textID)
	}

	vectorResults, err := coll.Search([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(vectorResults) != 1 {
		t.Fatalf("Search returned %d results, want 1 vector-backed result", len(vectorResults))
	}
	if vectorResults[0].Record.ID != vectorID {
		t.Fatalf("Search returned ID %d, want %d", vectorResults[0].Record.ID, vectorID)
	}

	hybridResults, err := coll.HybridSearch([]float32{1, 0, 0}, "checkout submit", TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if !hasResultID(hybridResults, textID) {
		t.Fatalf("HybridSearch did not include text-only result ID %d: %#v", textID, hybridResults)
	}
}

func TestTextDocumentUpsertReindexesAndRemovesVector(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("docs",
		WithDimension(3),
		WithTextIndex("doc_key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	id, err := coll.InsertDocument(
		[]float32{1, 0, 0},
		"oldtoken",
		map[string]any{"doc_key": "a"},
	)
	if err != nil {
		t.Fatal(err)
	}

	upsertID, inserted, err := coll.UpsertTextDocumentByKey(
		"doc_key",
		"a",
		"newtoken",
		map[string]any{"doc_key": "a"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("expected UpsertTextDocumentByKey to update existing record")
	}
	if upsertID != id {
		t.Fatalf("updated ID = %d, want %d", upsertID, id)
	}

	assertNoTextResults(t, coll, "oldtoken")
	assertTextResultID(t, coll, "newtoken", id)

	rec, err := coll.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Vector) != 0 {
		t.Fatalf("updated text document vector len = %d, want 0", len(rec.Vector))
	}

	vectorResults, err := coll.Search([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(vectorResults) != 0 {
		t.Fatalf("Search returned %d results for text-only record, want 0", len(vectorResults))
	}

	if _, err := coll.UpsertTextDocument(id, "finaltoken", map[string]any{"doc_key": "a"}); err != nil {
		t.Fatal(err)
	}
	assertNoTextResults(t, coll, "newtoken")
	assertTextResultID(t, coll, "finaltoken", id)
}

func TestTextDocumentCanReceiveVectorLater(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("evidence",
		WithHNSW(16, 200),
		WithTextIndex("doc_key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	id, err := coll.InsertTextDocument("keyword only first", map[string]any{"doc_key": "a"})
	if err != nil {
		t.Fatal(err)
	}

	results, err := coll.Search([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("Search returned %d results before vector promotion, want 0", len(results))
	}

	if err := coll.UpdateVector(id, []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if coll.Dimension() != 3 {
		t.Fatalf("Dimension = %d, want 3", coll.Dimension())
	}
	if stats := coll.IndexStats(); stats == nil || stats.NodeCount != 1 {
		t.Fatalf("IndexStats = %#v, want one indexed vector", stats)
	}

	results, err = coll.Search([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Record.ID != id {
		t.Fatalf("Search after UpdateVector returned %#v, want ID %d", results, id)
	}

	if _, err := coll.UpsertTextDocument(id, "keyword only again", map[string]any{"doc_key": "a"}); err != nil {
		t.Fatal(err)
	}
	results, err = coll.Search([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("Search returned %d results after demotion to text-only, want 0", len(results))
	}

	updatedID, inserted, err := coll.UpsertByKey("doc_key", "a", []float32{0, 1, 0}, map[string]any{"doc_key": "a"})
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("expected UpsertByKey to update text-only record")
	}
	if updatedID != id {
		t.Fatalf("updated ID = %d, want %d", updatedID, id)
	}

	results, err = coll.Search([]float32{0, 1, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Record.ID != id {
		t.Fatalf("Search after UpsertByKey returned %#v, want ID %d", results, id)
	}
}

func TestTextIndexReindexedOnPayloadMutation(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	coll, err := db.CreateCollection("docs",
		WithDimension(3),
		WithTextIndex("title", "doc"),
	)
	if err != nil {
		t.Fatal(err)
	}

	id, err := coll.InsertDocument(
		[]float32{0.1, 0.2, 0.3},
		"body text",
		map[string]any{"title": "oldtoken"},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := coll.Update(id, map[string]any{"title": "newtoken"}); err != nil {
		t.Fatal(err)
	}
	assertNoTextResults(t, coll, "oldtoken")
	assertTextResultID(t, coll, "newtoken", id)

	upsertID, err := coll.Upsert(0, []float32{0.2, 0.3, 0.4}, map[string]any{"title": "alphatoken"})
	if err != nil {
		t.Fatal(err)
	}
	assertTextResultID(t, coll, "alphatoken", upsertID)

	_, err = coll.Upsert(upsertID, []float32{0.3, 0.4, 0.5}, map[string]any{"title": "betatoken"})
	if err != nil {
		t.Fatal(err)
	}
	assertNoTextResults(t, coll, "alphatoken")
	assertTextResultID(t, coll, "betatoken", upsertID)

	keyID, inserted, err := coll.UpsertByKey("doc", "key1", []float32{0.4, 0.5, 0.6}, map[string]any{
		"doc":   "key1",
		"title": "gammatoken",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("expected initial UpsertByKey to insert")
	}
	assertTextResultID(t, coll, "gammatoken", keyID)

	updatedID, inserted, err := coll.UpsertByKey("doc", "key1", []float32{0.5, 0.6, 0.7}, map[string]any{
		"doc":   "key1",
		"title": "deltatoken",
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("expected second UpsertByKey to update")
	}
	if updatedID != keyID {
		t.Fatalf("updated ID = %d, want %d", updatedID, keyID)
	}
	assertNoTextResults(t, coll, "gammatoken")
	assertTextResultID(t, coll, "deltatoken", keyID)
}

func hasResultID(results []Result, id uint64) bool {
	for _, result := range results {
		if result.Record.ID == id {
			return true
		}
	}
	return false
}

func assertNoTextResults(t *testing.T, coll *Collection, query string) {
	t.Helper()
	results, err := coll.TextSearch(query, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("TextSearch(%q) returned %d results, want 0", query, len(results))
	}
}

func assertTextResultID(t *testing.T, coll *Collection, query string, id uint64) {
	t.Helper()
	results, err := coll.TextSearch(query, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatalf("TextSearch(%q) returned no results, want ID %d", query, id)
	}
	if results[0].Record.ID != id {
		t.Fatalf("TextSearch(%q) returned ID %d, want %d", query, results[0].Record.ID, id)
	}
}
