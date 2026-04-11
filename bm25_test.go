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
