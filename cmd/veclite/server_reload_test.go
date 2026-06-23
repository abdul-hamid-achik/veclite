package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/abdul-hamid-achik/veclite"
)

// newFileTestServer creates a server backed by a file-based DB for reload tests.
func newFileTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/test.veclite"
	db, err := veclite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s := &Server{db: db, dbPath: path}
	return s, func() { _ = db.Close() }
}

func TestHTTPReload(t *testing.T) {
	s, cleanup := newFileTestServer(t)
	defer cleanup()

	// Create a collection and insert a record via the collection router.
	rec := do(t, s, http.MethodPost, "/collections/test/vectors",
		map[string]any{"vector": []float64{1, 0}, "payload": map[string]any{"label": "a"}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("insert: status %d body %s", rec.Code, rec.Body.String())
	}

	// Sync to disk via the full mux router.
	rec = doRequest(t, s, http.MethodPost, "/sync", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: status %d", rec.Code)
	}

	// Reload via the full mux router.
	rec = doRequest(t, s, http.MethodPost, "/reload", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reload: status %d body %s", rec.Code, rec.Body.String())
	}
	out := decode(t, rec)
	if out["status"] != "reloaded" {
		t.Errorf("expected status 'reloaded', got %v", out["status"])
	}

	// Verify the data is still there after reload.
	rec = do(t, s, http.MethodPost, "/collections/test/search",
		map[string]any{"query": []float64{1, 0}, "top_k": 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("search after reload: status %d", rec.Code)
	}
	results := decode(t, rec)["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result after reload, got %d", len(results))
	}
}

func TestHTTPReloadMethodNotAllowed(t *testing.T) {
	s, cleanup := newFileTestServer(t)
	defer cleanup()

	rec := doRequest(t, s, http.MethodGet, "/reload", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHTTPConcurrentMultiClient(t *testing.T) {
	s := newTestServer(t)

	// Insert initial data.
	for i := 0; i < 20; i++ {
		v := float64(i)
		rec := do(t, s, http.MethodPost, "/collections/items/vectors",
			map[string]any{"vector": []float64{v, float64(i % 3)}, "payload": map[string]any{"idx": i}})
		if rec.Code != http.StatusCreated {
			t.Fatalf("insert %d: status %d", i, rec.Code)
		}
	}

	// Spin up a real HTTP server for concurrent access.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/info", s.handleInfo)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/collections", s.handleCollections)
	mux.HandleFunc("/collections/", s.handleCollection)
	mux.HandleFunc("/sync", s.handleSync)
	mux.HandleFunc("/reload", s.handleReload)

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	// 50 concurrent readers.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"query": []float64{1, 0}, "top_k": 5})
			resp, err := http.Post(
				httpServer.URL+"/collections/items/search",
				"application/json",
				strings.NewReader(string(body)),
			)
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- &testErr{"reader " + strconv.Itoa(n) + ": status " + resp.Status}
			}
		}(i)
	}

	// 50 concurrent writers.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{
				"vector":  []float64{float64(n), float64(n % 3)},
				"payload": map[string]any{"batch": n},
			})
			resp, err := http.Post(
				httpServer.URL+"/collections/items/vectors",
				"application/json",
				strings.NewReader(string(body)),
			)
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				errs <- &testErr{"writer " + strconv.Itoa(n) + ": status " + resp.Status}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

// doRequest sends a request through the full mux (not just handleCollection).
// Unlike do() which only routes to handleCollection, this routes to the
// correct top-level handler based on the path.
func doRequest(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()

	// Route to the correct handler — mirrors cmdServe's mux setup.
	switch {
	case path == "/health":
		s.handleHealth(rec, req)
	case path == "/info":
		s.handleInfo(rec, req)
	case path == "/metrics":
		s.handleMetrics(rec, req)
	case path == "/sync":
		s.handleSync(rec, req)
	case path == "/reload":
		s.handleReload(rec, req)
	case path == "/collections":
		s.handleCollections(rec, req)
	case strings.HasPrefix(path, "/collections/"):
		s.handleCollection(rec, req)
	default:
		rec.Code = http.StatusNotFound
	}
	return rec
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }