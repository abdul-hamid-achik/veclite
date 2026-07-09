package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/veclite"
)

// newTestServer starts an HTTP server backed by a file-based VecLite DB
// and returns the client base URL plus the underlying DB for direct writes.
func newTestServer(t *testing.T) (*DB, *veclite.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.veclite")

	db, err := veclite.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	server := &struct {
		db *veclite.DB
	}{db: db}

	mux := http.NewServeMux()

	// Reuse the real server handlers by wrapping the veclite server.
	// We can't import cmd/veclite (it's package main), so we wire minimal
	// handlers that call into the DB directly.

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": veclite.Version})
	})
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		stats := db.Stats()
		writeJSON(w, http.StatusOK, map[string]any{
			"path":          path,
			"collections":   stats.Collections,
			"total_records": stats.TotalRecords,
			"version":       veclite.Version,
		})
	})
	mux.HandleFunc("/collections", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			colls := db.Collections()
			type collInfo struct {
				Name      string `json:"name"`
				Count     int    `json:"count"`
				Dimension int    `json:"dimension"`
				Distance  string `json:"distance"`
				IndexType string `json:"index_type"`
			}
			result := make([]collInfo, 0, len(colls))
			for _, name := range colls {
				c, _ := db.GetCollection(name)
				s := c.Stats()
				result = append(result, collInfo{
					Name: name, Count: s.Count, Dimension: s.Dimension,
					Distance: s.DistanceType, IndexType: s.IndexType,
				})
			}
			writeJSON(w, http.StatusOK, result)
		case "POST":
			var req struct {
				Name      string `json:"name"`
				Dimension int    `json:"dimension"`
				Distance  string `json:"distance"`
				HNSW      bool   `json:"hnsw"`
				HNSWM     int    `json:"hnsw_m"`
				HNSWEf    int    `json:"hnsw_ef"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
				return
			}
			opts := []veclite.CollectionOption{veclite.WithDistanceType(veclite.DistanceCosine)}
			if req.Dimension > 0 {
				opts = append(opts, veclite.WithDimension(req.Dimension))
			}
			if req.HNSW {
				m := req.HNSWM
				if m == 0 {
					m = 16
				}
				ef := req.HNSWEf
				if ef == 0 {
					ef = 200
				}
				opts = append(opts, veclite.WithHNSW(m, ef))
			}
			_, err := db.CreateCollection(req.Name, opts...)
			if err != nil {
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"status": "created", "collection": req.Name})
		}
	})
	mux.HandleFunc("/collections/", func(w http.ResponseWriter, r *http.Request) {
		// Minimal router for test purposes
		path := r.URL.Path[len("/collections/"):]
		// ...delegate to the full handler set
		_ = path
		// For the test, we just need search and vectors endpoints
		handleCollectionRoute(w, r, db, path)
	})
	mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Sync(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "synced"})
	})
	mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Reload(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "reloaded"})
	})

	_ = server

	httpServer := httptest.NewServer(mux)

	cli, err := Open(httpServer.URL)
	if err != nil {
		t.Fatalf("open client: %v", err)
	}

	cleanup := func() {
		httpServer.Close()
		_ = db.Close()
	}
	return cli, db, cleanup
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

// handleCollectionRoute is a minimal per-collection router for tests.
func handleCollectionRoute(w http.ResponseWriter, r *http.Request, db *veclite.DB, path string) {
	// Parse: {name}/vectors, {name}/vectors/{id}, {name}/search, {name}/upsert, {name}/find
	parts := splitPath(path)
	if len(parts) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing name"})
		return
	}
	name := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case "GET":
			c, err := db.GetCollection(name)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			s := c.Stats()
			writeJSON(w, http.StatusOK, map[string]any{
				"name": s.Name, "count": s.Count, "dimension": s.Dimension,
				"distance": s.DistanceType, "index_type": s.IndexType,
			})
		case "DELETE":
			if err := db.DropCollection(name); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "dropped", "collection": name})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		}
		return
	}

	coll, collErr := db.GetCollection(name)

	switch parts[1] {
	case "vectors":
		if len(parts) == 2 {
			switch r.Method {
			case "POST":
				// POST vectors auto-creates the collection (matches real server behavior).
				coll = db.Collection(name)
				var req struct {
					Vector   []float64        `json:"vector"`
					Vectors  [][]float64      `json:"vectors"`
					Payload  map[string]any   `json:"payload"`
					Payloads []map[string]any `json:"payloads"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if req.Vector != nil {
					v := toFloat32(req.Vector)
					id, err := coll.Insert(v, req.Payload)
					if err != nil {
						writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
						return
					}
					writeJSON(w, http.StatusCreated, map[string]any{"status": "inserted", "id": id})
					return
				}
				if req.Vectors != nil {
					vs := make([][]float32, len(req.Vectors))
					for i, v := range req.Vectors {
						vs[i] = toFloat32(v)
					}
					ids, err := coll.InsertBatch(vs, req.Payloads)
					if err != nil {
						writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
						return
					}
					writeJSON(w, http.StatusCreated, map[string]any{"status": "inserted", "count": len(ids), "ids": ids})
					return
				}
			}
			return
		}
		if len(parts) == 3 {
			// /collections/{name}/vectors/{id}
			if collErr != nil {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			id := parseUint(parts[2])
			switch r.Method {
			case "GET":
				rec, err := coll.Get(id)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"id": rec.ID, "vector": rec.Vector, "payload": rec.Payload,
				})
			case "DELETE":
				_ = coll.Delete(id)
				writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": id})
			case "PUT":
				var req struct {
					Vector  []float64      `json:"vector"`
					Payload map[string]any `json:"payload"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if req.Vector != nil {
					_ = coll.UpdateVector(id, toFloat32(req.Vector))
				}
				if req.Payload != nil {
					_ = coll.Update(id, req.Payload)
				}
				writeJSON(w, http.StatusOK, map[string]any{"status": "updated", "id": id})
			}
			return
		}
	case "search":
		if collErr != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		if r.Method == "POST" {
			var req struct {
				Query     []float64    `json:"query"`
				TopK      int          `json:"top_k"`
				Threshold *float64     `json:"threshold"`
				Filters   []clientFilt `json:"filters"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			opts := []veclite.SearchOption{veclite.TopK(req.TopK)}
			if req.Threshold != nil {
				opts = append(opts, veclite.Threshold(float32(*req.Threshold)))
			}
			for _, f := range req.Filters {
				opts = append(opts, veclite.WithFilter(veclite.Equal(f.Key, f.Value)))
			}
			results, err := coll.Search(toFloat32(req.Query), opts...)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			type out struct {
				ID      uint64         `json:"id"`
				Score   float32        `json:"score"`
				Payload map[string]any `json:"payload,omitempty"`
			}
			outs := make([]out, len(results))
			for i, r := range results {
				outs[i] = out{ID: r.Record.ID, Score: r.Score, Payload: r.Record.Payload}
			}
			writeJSON(w, http.StatusOK, map[string]any{"results": outs, "count": len(outs)})
			return
		}
	case "upsert":
		if collErr != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		if r.Method == "POST" {
			var req struct {
				ID       uint64         `json:"id"`
				Vector   []float64      `json:"vector"`
				Payload  map[string]any `json:"payload"`
				KeyField string         `json:"key_field"`
				KeyValue any            `json:"key_value"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.KeyField != "" {
				id, inserted, err := coll.UpsertByKey(req.KeyField, req.KeyValue, toFloat32(req.Vector), req.Payload)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
					return
				}
				status := "updated"
				if inserted {
					status = "inserted"
				}
				writeJSON(w, http.StatusOK, map[string]any{"status": status, "id": id, "inserted": inserted})
				return
			}
			id, err := coll.Upsert(req.ID, toFloat32(req.Vector), req.Payload)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "upserted", "id": id})
			return
		}
	case "find":
		if collErr != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		if r.Method == "POST" {
			var req struct {
				Filters []clientFilt `json:"filters"`
				Limit   int          `json:"limit"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			var filters []veclite.Filter
			for _, f := range req.Filters {
				filters = append(filters, veclite.Equal(f.Key, f.Value))
			}
			records, err := coll.Find(filters...)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			type out struct {
				ID      uint64         `json:"id"`
				Payload map[string]any `json:"payload,omitempty"`
			}
			outs := make([]out, len(records))
			for i, rec := range records {
				outs[i] = out{ID: rec.ID, Payload: rec.Payload}
			}
			writeJSON(w, http.StatusOK, map[string]any{"results": outs, "count": len(outs)})
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
}

type clientFilt struct {
	Key   string `json:"key"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

func splitPath(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + uint64(c-'0')
	}
	return n
}

func toFloat32(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}

// --- Tests ---

func TestClientCreateCollectionAndInsert(t *testing.T) {
	cli, db, cleanup := newTestServer(t)
	defer cleanup()
	_ = db

	coll, err := cli.CreateCollection("items", WithDimension(3), WithDistanceType(DistanceCosine))
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	id, err := coll.Insert([]float32{1, 0, 0}, map[string]any{"label": "a"})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if id == 0 {
		t.Errorf("expected non-zero ID")
	}
}

func TestClientSearch(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	coll, err := cli.CreateCollection("docs", WithDimension(2), WithDistanceType(DistanceCosine))
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	_, err = coll.Insert([]float32{1, 0}, map[string]any{"label": "a"})
	if err != nil {
		t.Fatalf("Insert a: %v", err)
	}
	_, err = coll.Insert([]float32{0, 1}, map[string]any{"label": "b"})
	if err != nil {
		t.Fatalf("Insert b: %v", err)
	}

	results, err := coll.Search([]float32{1, 0}, TopK(1))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Payload["label"] != "a" {
		t.Errorf("expected label 'a', got %v", results[0].Payload["label"])
	}
}

func TestClientGetAndDelete(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	coll, err := cli.CreateCollection("things", WithDimension(2), WithDistanceType(DistanceCosine))
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	id, err := coll.Insert([]float32{1, 0}, map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	rec, err := coll.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Payload["k"] != "v" {
		t.Errorf("expected k=v, got %v", rec.Payload["k"])
	}

	if err := coll.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = coll.Get(id)
	if err == nil {
		t.Errorf("expected error after delete, got nil")
	}
}

func TestClientUpdateVectorAndPayload(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	coll, err := cli.CreateCollection("upd", WithDimension(2), WithDistanceType(DistanceCosine))
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	id, err := coll.Insert([]float32{1, 0}, map[string]any{"label": "a"})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if err := coll.UpdateVector(id, []float32{0, 1}); err != nil {
		t.Fatalf("UpdateVector: %v", err)
	}

	if err := coll.Update(id, map[string]any{"label": "b"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	rec, err := coll.Get(id)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if rec.Payload["label"] != "b" {
		t.Errorf("expected label 'b', got %v", rec.Payload["label"])
	}
}

func TestClientUpsertByKey(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	coll, err := cli.CreateCollection("ups", WithDimension(2), WithDistanceType(DistanceCosine))
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	id1, inserted, err := coll.UpsertByKey("key", "a", []float32{1, 0}, map[string]any{"key": "a", "label": "first"})
	if err != nil {
		t.Fatalf("UpsertByKey (insert): %v", err)
	}
	if !inserted {
		t.Errorf("expected inserted=true on first upsert")
	}

	id2, inserted, err := coll.UpsertByKey("key", "a", []float32{0, 1}, map[string]any{"key": "a", "label": "second"})
	if err != nil {
		t.Fatalf("UpsertByKey (replace): %v", err)
	}
	if inserted {
		t.Errorf("expected inserted=false on replace")
	}
	if id1 != id2 {
		t.Errorf("expected same ID on replace, got %d then %d", id1, id2)
	}
}

func TestClientCollectionsList(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	_, _ = cli.CreateCollection("c1", WithDimension(2))
	_, _ = cli.CreateCollection("c2", WithDimension(3))

	names, err := cli.Collections()
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 collections, got %d: %v", len(names), names)
	}
}

func TestClientStats(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	_, _ = cli.CreateCollection("st", WithDimension(2))
	_, _ = cli.CreateCollection("st2", WithDimension(2))

	stats, err := cli.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Collections != 2 {
		t.Errorf("expected 2 collections, got %d", stats.Collections)
	}
}

func TestClientSyncAndReload(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	_, _ = cli.CreateCollection("sr", WithDimension(2))

	if err := cli.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if err := cli.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
}

func TestClientDropCollection(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	_, _ = cli.CreateCollection("drop", WithDimension(2))

	if err := cli.DropCollection("drop"); err != nil {
		t.Fatalf("DropCollection: %v", err)
	}

	_, err := cli.GetCollection("drop")
	if err == nil {
		t.Errorf("expected error after drop, got nil")
	}
}

func TestClientFindWithFilter(t *testing.T) {
	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	coll, err := cli.CreateCollection("find", WithDimension(2), WithDistanceType(DistanceCosine))
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	_, _ = coll.Insert([]float32{1, 0}, map[string]any{"group": "a"})
	_, _ = coll.Insert([]float32{0, 1}, map[string]any{"group": "b"})
	_, _ = coll.Insert([]float32{1, 1}, map[string]any{"group": "a"})

	records, err := coll.Find(Equal("group", "a"))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records with group=a, got %d", len(records))
	}
}

func TestClientContextUsed(t *testing.T) {
	// Verify the client uses context for requests
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	cli, _, cleanup := newTestServer(t)
	defer cleanup()

	_, _ = cli.CreateCollection("ctx", WithDimension(2))

	// Use doJSON with a cancelled context — should fail
	err := cli.doJSON(ctx, http.MethodGet, "/collections", nil, nil)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}
