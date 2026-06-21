package veclite

import "testing"

func TestWithContentControlsSearchResultContent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	coll, err := db.CreateCollection("docs",
		WithDimension(3),
		WithHNSW(8, 50),
		WithTextIndex("title"),
	)
	if err != nil {
		t.Fatal(err)
	}

	id, err := coll.InsertDocument(
		[]float32{1, 0, 0},
		"persistent searchable content",
		map[string]any{"title": "content option"},
	)
	if err != nil {
		t.Fatal(err)
	}

	vectorResults, err := coll.Search([]float32{1, 0, 0}, TopK(1))
	if err != nil {
		t.Fatal(err)
	}
	requireResultContent(t, vectorResults, "persistent searchable content")

	vectorResults, err = coll.Search([]float32{1, 0, 0}, TopK(1), WithContent(false))
	if err != nil {
		t.Fatal(err)
	}
	requireResultContent(t, vectorResults, "")

	vectorResults, err = coll.Search([]float32{1, 0, 0}, TopK(1), WithContent(true))
	if err != nil {
		t.Fatal(err)
	}
	requireResultContent(t, vectorResults, "persistent searchable content")

	textResults, err := coll.TextSearch("searchable content", TopK(1), WithContent(false))
	if err != nil {
		t.Fatal(err)
	}
	requireResultContent(t, textResults, "")

	hybridResults, err := coll.HybridSearch([]float32{1, 0, 0}, "searchable content", TopK(1), WithContent(false))
	if err != nil {
		t.Fatal(err)
	}
	requireResultContent(t, hybridResults, "")

	explanation, err := coll.SearchExplain([]float32{1, 0, 0}, TopK(1), WithContent(false))
	if err != nil {
		t.Fatal(err)
	}
	requireResultContent(t, explanation.Results, "")

	var streamed []Result
	if err := coll.SearchStream([]float32{1, 0, 0}, func(r Result) bool {
		streamed = append(streamed, r)
		return true
	}, TopK(1), WithContent(false)); err != nil {
		t.Fatal(err)
	}
	requireResultContent(t, streamed, "")

	stored, err := coll.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Content != "persistent searchable content" {
		t.Fatalf("stored content = %q, want original content", stored.Content)
	}
}

func TestSearchExplainAppliesPaginationOptions(t *testing.T) {
	tests := []struct {
		name string
		opts []CollectionOption
	}{
		{name: "bruteforce", opts: []CollectionOption{WithDimension(2)}},
		{name: "hnsw", opts: []CollectionOption{WithDimension(2), WithHNSW(8, 50)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Open(":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()

			coll, err := db.CreateCollection("docs", tt.opts...)
			if err != nil {
				t.Fatal(err)
			}
			for _, vector := range [][]float32{
				{1, 0},
				{0.9, 0.1},
				{0, 1},
			} {
				if _, err := coll.Insert(vector, nil); err != nil {
					t.Fatal(err)
				}
			}

			searchResults, err := coll.Search([]float32{1, 0}, TopK(1), WithOffset(1))
			if err != nil {
				t.Fatal(err)
			}
			explanation, err := coll.SearchExplain([]float32{1, 0}, TopK(1), WithOffset(1))
			if err != nil {
				t.Fatal(err)
			}

			if len(searchResults) != 1 {
				t.Fatalf("Search returned %d results, want 1", len(searchResults))
			}
			if len(explanation.Results) != 1 {
				t.Fatalf("SearchExplain returned %d results, want 1", len(explanation.Results))
			}
			if explanation.Results[0].Record.ID != searchResults[0].Record.ID {
				t.Fatalf("SearchExplain ID = %d, want Search ID %d",
					explanation.Results[0].Record.ID,
					searchResults[0].Record.ID,
				)
			}
		})
	}
}

func requireResultContent(t *testing.T, results []Result, want string) {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Record.Content != want {
		t.Fatalf("result content = %q, want %q", results[0].Record.Content, want)
	}
}
