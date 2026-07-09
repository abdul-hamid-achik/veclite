package main

import (
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/veclite"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		msg      string
		wantCode string
	}{
		{"vector dimension mismatch: expected 768, got 1536", "DIMENSION_MISMATCH"},
		{"Auto-embedding failed: connection refused", "EMBEDDER_UNAVAILABLE"},
		{"Either 'query' vector or 'text' is required", "INVALID_INPUT"},
		{"Record not found: no record with id 42", "NOT_FOUND"},
		{"database file is locked by another process", "LOCK_HELD"},
		{"This operation requires confirm: true", "CONFIRMATION_REQUIRED"},
		{"something entirely unexpected", "OPERATION_FAILED"},
	}
	for _, c := range cases {
		code, _ := classifyError(c.msg)
		if code != c.wantCode {
			t.Errorf("classifyError(%q) = %q, want %q", c.msg, code, c.wantCode)
		}
	}
}

func TestTruncateContent(t *testing.T) {
	short := "hello"
	if got := truncateContent(short, 2000); got != short {
		t.Errorf("short content should pass through unchanged, got %q", got)
	}

	long := strings.Repeat("x", 3000)
	got := truncateContent(long, 2000)
	if len(got) <= 2000 {
		// truncated body + suffix note; body must be exactly max
		t.Errorf("expected truncated content with suffix, got len %d", len(got))
	}
	if !strings.HasPrefix(got, strings.Repeat("x", 2000)) {
		t.Error("truncated content should preserve the first max chars")
	}
	if !strings.Contains(got, "truncated 1000 of 3000 chars") {
		t.Errorf("truncation note missing or wrong: %q", got[2000:])
	}
}

func TestSearchEnvelope(t *testing.T) {
	db, err := veclite.Open(t.TempDir() + "/env.veclite")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	coll, err := db.CreateCollection("test", veclite.WithDimension(3))
	if err != nil {
		t.Fatal(err)
	}

	// Empty collection, no results → hint says collection is empty.
	env := searchEnvelope(coll, nil, "vector")
	if env["total"] != 0 {
		t.Errorf("total = %v, want 0", env["total"])
	}
	hint, _ := env["hint"].(string)
	if !strings.Contains(hint, "collection is empty") {
		t.Errorf("empty-collection hint missing, got %q", hint)
	}

	// Non-empty collection, no results → mode-specific guidance.
	if _, err := coll.Insert([]float32{1, 0, 0}, nil); err != nil {
		t.Fatal(err)
	}
	env = searchEnvelope(coll, nil, "text")
	hint, _ = env["hint"].(string)
	if !strings.Contains(hint, "BM25") {
		t.Errorf("text-mode hint should mention BM25 fallback, got %q", hint)
	}

	// Results present with a good score → no hint.
	results := []veclite.Result{{Record: &veclite.Record{ID: 1}, Score: 0.9}}
	env = searchEnvelope(coll, results, "vector")
	if _, ok := env["hint"]; ok {
		t.Errorf("no hint expected for healthy results, got %v", env["hint"])
	}
	if env["total"] != 1 {
		t.Errorf("total = %v, want 1", env["total"])
	}

	// Low top score → weak-match hint.
	results[0].Score = 0.1
	env = searchEnvelope(coll, results, "vector")
	hint, _ = env["hint"].(string)
	if !strings.Contains(hint, "low") {
		t.Errorf("low-score hint missing, got %q", hint)
	}
}
