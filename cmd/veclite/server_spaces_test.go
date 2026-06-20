package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abdul-hamid-achik/veclite"
)

// newTestServer returns a Server backed by an in-memory database with one
// dimension-2 collection named "items".
func newTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := veclite.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.CreateCollection("items", veclite.WithDimension(2)); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	return &Server{db: db, dbPath: ":memory:"}
}

// do sends a request through the collection router and returns the recorder.
func do(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	rec := httptest.NewRecorder()
	s.handleCollection(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestHTTPNamedSpacesLifecycle(t *testing.T) {
	s := newTestServer(t)

	// Declare a named image space.
	rec := do(t, s, http.MethodPost, "/collections/items/spaces",
		map[string]any{"name": "image", "dimension": 2, "hnsw": true})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add space: status %d body %s", rec.Code, rec.Body.String())
	}

	// Insert two multi-space records.
	for _, r := range []map[string]any{
		{"payload": map[string]any{"label": "a"}, "vectors": map[string][]float64{"default": {1, 0}, "image": {1, 0}}},
		{"payload": map[string]any{"label": "b"}, "vectors": map[string][]float64{"default": {0, 1}, "image": {0, 1}}},
	} {
		rec := do(t, s, http.MethodPost, "/collections/items/records", r)
		if rec.Code != http.StatusCreated {
			t.Fatalf("insert record: status %d body %s", rec.Code, rec.Body.String())
		}
	}

	// List spaces: default + image (hnsw).
	rec = do(t, s, http.MethodGet, "/collections/items/spaces", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list spaces: status %d", rec.Code)
	}
	out := decode(t, rec)
	spaces, _ := out["spaces"].([]any)
	if len(spaces) != 2 {
		t.Fatalf("expected 2 spaces, got %v", out["spaces"])
	}
	foundImageHNSW := false
	for _, sp := range spaces {
		m := sp.(map[string]any)
		if m["name"] == "image" && m["index_type"] == "hnsw" {
			foundImageHNSW = true
		}
	}
	if !foundImageHNSW {
		t.Errorf("image space not reported as hnsw: %v", spaces)
	}

	// Search the image space: query nearest "b".
	rec = do(t, s, http.MethodPost, "/collections/items/search-space",
		map[string]any{"space": "image", "query": []float64{0, 1}, "top_k": 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("search-space: status %d body %s", rec.Code, rec.Body.String())
	}
	res := decode(t, rec)["results"].([]any)
	if len(res) != 1 || res[0].(map[string]any)["payload"].(map[string]any)["label"] != "b" {
		t.Fatalf("image search should return 'b', got %v", res)
	}

	// Fuse both spaces toward "a": a is top in both, so it wins.
	rec = do(t, s, http.MethodPost, "/collections/items/fuse-search",
		map[string]any{"queries": map[string][]float64{"default": {1, 0}, "image": {1, 0}}, "top_k": 2})
	if rec.Code != http.StatusOK {
		t.Fatalf("fuse-search: status %d body %s", rec.Code, rec.Body.String())
	}
	fused := decode(t, rec)["results"].([]any)
	if len(fused) == 0 || fused[0].(map[string]any)["payload"].(map[string]any)["label"] != "a" {
		t.Fatalf("fusion should rank 'a' first, got %v", fused)
	}
}

func TestParseFilterRequestNumericOps(t *testing.T) {
	rec := &veclite.Record{Payload: map[string]any{"n": 5.0}}
	cases := []struct {
		op   string
		val  float64
		want bool
	}{
		{"gt", 3, true}, {"gt", 5, false}, {">", 4, true},
		{"gte", 5, true}, {"gte", 6, false},
		{"lt", 9, true}, {"lt", 5, false}, {"<", 6, true},
		{"lte", 5, true}, {"lte", 4, false},
	}
	for _, c := range cases {
		f := parseFilterRequest(filterRequest{Key: "n", Op: c.op, Value: c.val})
		if f == nil {
			t.Fatalf("op %q returned a nil filter (operator not wired up?)", c.op)
		}
		if got := f.Match(rec); got != c.want {
			t.Errorf("op %q val %v: Match=%v, want %v", c.op, c.val, got, c.want)
		}
	}
}

func TestHTTPSearchUnknownSpaceFails(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/collections/items/search-space",
		map[string]any{"space": "nope", "query": []float64{1, 0}, "top_k": 1})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown space, got %d body %s", rec.Code, rec.Body.String())
	}
}
