package veclite

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestTextDocumentSearchExplainAndStreamSkipTextOnly(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	coll, err := db.CreateCollection("evidence",
		WithDimension(3),
		WithHNSW(8, 50),
		WithTextIndex("kind"),
	)
	if err != nil {
		t.Fatal(err)
	}

	textID, err := coll.InsertTextDocument(
		"keyword-only checkout failure",
		map[string]any{"kind": "frame"},
	)
	if err != nil {
		t.Fatal(err)
	}

	explanation, err := coll.SearchExplain([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(explanation.Results) != 0 {
		t.Fatalf("SearchExplain returned %d results for text-only HNSW collection, want 0", len(explanation.Results))
	}

	vectorID, err := coll.InsertDocument(
		[]float32{1, 0, 0},
		"semantic checkout failure",
		map[string]any{"kind": "clip"},
	)
	if err != nil {
		t.Fatal(err)
	}

	var streamed []uint64
	if err := coll.SearchStream([]float32{1, 0, 0}, func(r Result) bool {
		streamed = append(streamed, r.Record.ID)
		return true
	}, TopK(10)); err != nil {
		t.Fatal(err)
	}
	if len(streamed) != 1 || streamed[0] != vectorID {
		t.Fatalf("SearchStream returned IDs %v, want only vector-backed ID %d", streamed, vectorID)
	}

	assertTextResultID(t, coll, "keyword-only", textID)
}

func TestTextDocumentPersistenceAndHNSWPromotion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.veclite")

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	coll, err := db.CreateCollection("evidence",
		WithHNSW(8, 50),
		WithTextIndex("kind"),
	)
	if err != nil {
		t.Fatal(err)
	}

	id, err := coll.InsertTextDocument(
		"transcript says checkout failed",
		map[string]any{"kind": "transcript", "source": "video.mp4"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	coll, err = db.GetCollection("evidence")
	if err != nil {
		t.Fatal(err)
	}

	stats := coll.Stats()
	if stats.Count != 1 || stats.VectorCount != 0 || stats.TextOnlyCount != 1 || stats.Dimension != 0 {
		t.Fatalf("Stats after reload = %#v, want one dimensionless text-only record", stats)
	}
	assertTextResultID(t, coll, "checkout failed", id)

	results, err := coll.Search([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("Search returned %d results before vector promotion, want 0", len(results))
	}

	explanation, err := coll.SearchExplain([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(explanation.Results) != 0 {
		t.Fatalf("SearchExplain returned %d results before vector promotion, want 0", len(explanation.Results))
	}

	if err := coll.UpdateVector(id, []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if got := coll.Dimension(); got != 3 {
		t.Fatalf("Dimension after vector promotion = %d, want 3", got)
	}
	if stats := coll.IndexStats(); stats == nil || stats.NodeCount != 1 {
		t.Fatalf("IndexStats after vector promotion = %#v, want one indexed vector", stats)
	}

	results, err = coll.Search([]float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Record.ID != id {
		t.Fatalf("Search after vector promotion returned %#v, want ID %d", results, id)
	}
}

func TestTextDocumentWritesRespectReadOnlyAndClosedDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly.veclite")

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	coll, err := db.CreateCollection("docs", WithTextIndex("kind"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := coll.InsertTextDocument("initial document", map[string]any{"kind": "note"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(path, WithReadOnly(true))
	if err != nil {
		t.Fatal(err)
	}
	coll, err = db.GetCollection("docs")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := coll.InsertTextDocument("blocked", map[string]any{"kind": "note"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("InsertTextDocument read-only error = %v, want ErrReadOnly", err)
	}
	if _, err := coll.UpsertTextDocument(id, "blocked", map[string]any{"kind": "note"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("UpsertTextDocument read-only error = %v, want ErrReadOnly", err)
	}
	if _, _, err := coll.UpsertTextDocumentByKey("kind", "note", "blocked", map[string]any{"kind": "note"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("UpsertTextDocumentByKey read-only error = %v, want ErrReadOnly", err)
	}
	if err := coll.UpdateDocument(id, "blocked", map[string]any{"kind": "note"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("UpdateDocument read-only error = %v, want ErrReadOnly", err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := coll.InsertTextDocument("blocked", map[string]any{"kind": "note"}); !errors.Is(err, ErrDatabaseClosed) {
		t.Fatalf("InsertTextDocument closed-db error = %v, want ErrDatabaseClosed", err)
	}
}

func TestInsertTextDocumentWithOptions(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	coll, err := db.CreateCollection("docs", WithTextIndex("kind"))
	if err != nil {
		t.Fatal(err)
	}

	id, err := coll.InsertTextDocumentWithOptions(
		"important retained keyword",
		map[string]any{"kind": "note"},
		WithTTL(time.Hour),
		WithImportance(0.8),
	)
	if err != nil {
		t.Fatal(err)
	}

	record, err := coll.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !record.HasTTL() {
		t.Fatal("text document has no TTL, want TTL from insert option")
	}
	if record.TTL() <= 0 {
		t.Fatalf("text document TTL = %v, want positive duration", record.TTL())
	}
	if record.Importance != 0.8 {
		t.Fatalf("text document importance = %v, want 0.8", record.Importance)
	}
	assertTextResultID(t, coll, "retained keyword", id)

	expiredID, err := coll.InsertTextDocumentWithOptions(
		"expiredtoken",
		map[string]any{"kind": "note"},
		WithExpiresAt(time.Now().Add(-time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := coll.CleanupExpired()
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("CleanupExpired deleted %d records, want 1", deleted)
	}
	if _, err := coll.Get(expiredID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get expired text document error = %v, want ErrNotFound", err)
	}
	assertNoTextResults(t, coll, "expiredtoken")
}
