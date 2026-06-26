// Package client provides a thin Go client for a VecLite HTTP server.
//
// It mirrors the embedded [github.com/abdul-hamid-achik/veclite] library's API
// surface so that Go consumers can swap between embedding the library directly
// (single-process) and talking to a remote server (multi-process) with minimal
// code change.
//
// # Quick start
//
//	// Instead of:   db, _ := veclite.Open("data.veclite")
//	// Use:          db, _ := client.Open("http://localhost:8080")
//
//	db, err := client.Open("http://localhost:8080")
//	if err != nil { ... }
//	defer db.Close()
//
//	coll, err := db.CreateCollection("docs",
//		client.WithDimension(384),
//		client.WithDistanceType(client.DistanceCosine),
//	)
//	id, err := coll.Insert([]float32{...}, map[string]any{"source": "wiki"})
//	results, err := coll.Search([]float32{...}, client.TopK(10))
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DistanceType is the distance metric for a collection.
type DistanceType string

const (
	DistanceCosine    DistanceType = "cosine"
	DistanceDot        DistanceType = "dot"
	DistanceEuclidean  DistanceType = "euclidean"
)

// Result is a search result returned by the client.
type Result struct {
	ID      uint64         `json:"id"`
	Score   float32        `json:"score"`
	Payload map[string]any `json:"payload,omitempty"`
	Vector  []float32      `json:"vector,omitempty"`
}

// Record is a record returned by Get/List operations.
type Record struct {
	ID      uint64         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload,omitempty"`
	Content string         `json:"content,omitempty"`
}

// CollectionStats contains statistics about a collection.
type CollectionStats struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Dimension int    `json:"dimension"`
	Distance  string `json:"distance"`
	IndexType string `json:"index_type"`
}

// DatabaseStats contains statistics about the database.
type DatabaseStats struct {
	Path         string            `json:"path"`
	Collections  int               `json:"collections"`
	TotalRecords int               `json:"total_records"`
	Version      string            `json:"version"`
}

// CollectionOption configures collection creation.
type CollectionOption func(*collectionConfig)

type collectionConfig struct {
	dimension   int
	distance    DistanceType
	hnsw        bool
	hnswM       int
	hnswEfCon   int
	textIndex   string
}

// WithDimension sets the vector dimension.
func WithDimension(d int) CollectionOption {
	return func(c *collectionConfig) { c.dimension = d }
}

// WithDistanceType sets the distance metric.
func WithDistanceType(d DistanceType) CollectionOption {
	return func(c *collectionConfig) { c.distance = d }
}

// WithHNSW enables an HNSW index with the given parameters.
func WithHNSW(m, efConstruction int) CollectionOption {
	return func(c *collectionConfig) {
		c.hnsw = true
		c.hnswM = m
		c.hnswEfCon = efConstruction
	}
}

// WithTextIndex enables BM25 text indexing on the given payload field.
func WithTextIndex(field string) CollectionOption {
	return func(c *collectionConfig) { c.textIndex = field }
}

// SearchOption configures search behavior.
type SearchOption func(*searchConfig)

type searchConfig struct {
	topK      int
	threshold *float32
	filters   []Filter
}

// TopK sets the maximum number of results.
func TopK(k int) SearchOption {
	return func(c *searchConfig) { c.topK = k }
}

// Threshold sets the minimum similarity score.
func Threshold(t float32) SearchOption {
	return func(c *searchConfig) { c.threshold = &t }
}

// WithFilter adds a filter to the search.
func WithFilter(f Filter) SearchOption {
	return func(c *searchConfig) { c.filters = append(c.filters, f) }
}

// WithFilters adds multiple filters to the search.
func WithFilters(filters ...Filter) SearchOption {
	return func(c *searchConfig) { c.filters = append(c.filters, filters...) }
}

// Filter is a metadata filter for search/find operations.
type Filter struct {
	Key   string `json:"key"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// Equal creates an equality filter.
func Equal(key string, value any) Filter { return Filter{Key: key, Op: "eq", Value: value} }

// NotEqual creates a not-equal filter.
func NotEqual(key string, value any) Filter { return Filter{Key: key, Op: "neq", Value: value} }

// GT creates a greater-than filter (numeric).
func GT(key string, value float64) Filter { return Filter{Key: key, Op: "gt", Value: value} }

// GTE creates a greater-than-or-equal filter (numeric).
func GTE(key string, value float64) Filter { return Filter{Key: key, Op: "gte", Value: value} }

// LT creates a less-than filter (numeric).
func LT(key string, value float64) Filter { return Filter{Key: key, Op: "lt", Value: value} }

// LTE creates a less-than-or-equal filter (numeric).
func LTE(key string, value float64) Filter { return Filter{Key: key, Op: "lte", Value: value} }

// Glob creates a glob-match filter (string).
func Glob(key, pattern string) Filter { return Filter{Key: key, Op: "glob", Value: pattern} }

// Prefix creates a prefix-match filter (string).
func Prefix(key, value string) Filter { return Filter{Key: key, Op: "prefix", Value: value} }

// Suffix creates a suffix-match filter (string).
func Suffix(key, value string) Filter { return Filter{Key: key, Op: "suffix", Value: value} }

// Contains creates a contains filter (string).
func Contains(key, value string) Filter { return Filter{Key: key, Op: "contains", Value: value} }

// Exists creates an exists filter (checks the key is present).
func Exists(key string) Filter { return Filter{Key: key, Op: "exists", Value: nil} }

// DB is a client connection to a VecLite HTTP server.
type DB struct {
	baseURL string
	http    *http.Client
}

// Open creates a new client connection to a VecLite server at the given base URL.
// The baseURL should include the scheme and host (e.g. "http://localhost:8080").
func Open(baseURL string) (*DB, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("client: base URL is required")
	}
	return &DB{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Close releases client resources. There is no server-side session to close;
// this is provided for API symmetry with the embedded library.
func (db *DB) Close() error { return nil }

// --- HTTP helpers ---

func (db *DB) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("client: marshal body: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, db.baseURL+path, rdr)
	if err != nil {
		return 0, nil, fmt.Errorf("client: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := db.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("client: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("client: read body: %w", err)
	}
	return resp.StatusCode, data, nil
}

func (db *DB) doJSON(ctx context.Context, method, path string, reqBody, respBody any) error {
	status, data, err := db.do(ctx, method, path, reqBody)
	if err != nil {
		return err
	}
	if status >= 400 {
		var apiErr struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.Unmarshal(data, &apiErr)
		if apiErr.Error != "" {
			return fmt.Errorf("client: %s: %s", apiErr.Code, apiErr.Error)
		}
		return fmt.Errorf("client: HTTP %d: %s", status, string(data))
	}
	if respBody != nil && len(data) > 0 {
		if err := json.Unmarshal(data, respBody); err != nil {
			return fmt.Errorf("client: decode response: %w", err)
		}
	}
	return nil
}

// --- DB-level operations ---

// CreateCollection creates a new collection on the server.
func (db *DB) CreateCollection(name string, opts ...CollectionOption) (*Collection, error) {
	cfg := &collectionConfig{distance: DistanceCosine}
	for _, o := range opts {
		o(cfg)
	}
	req := map[string]any{
		"name":     name,
		"distance": string(cfg.distance),
	}
	if cfg.dimension > 0 {
		req["dimension"] = cfg.dimension
	}
	if cfg.hnsw {
		m := cfg.hnswM
		if m == 0 {
			m = 16
		}
		ef := cfg.hnswEfCon
		if ef == 0 {
			ef = 200
		}
		req["hnsw"] = true
		req["hnsw_m"] = m
		req["hnsw_ef"] = ef
	}
	if err := db.doJSON(context.Background(), http.MethodPost, "/collections", req, nil); err != nil {
		return nil, err
	}
	return &Collection{db: db, name: name}, nil
}

// Collection returns a handle for the named collection. It does not verify the
// collection exists; use GetCollection if you need an error on missing collections.
func (db *DB) Collection(name string) *Collection {
	return &Collection{db: db, name: name}
}

// GetCollection returns a handle for the named collection, returning an error
// if the collection does not exist on the server.
func (db *DB) GetCollection(name string) (*Collection, error) {
	var stats CollectionStats
	if err := db.doJSON(context.Background(), http.MethodGet, "/collections/"+name, nil, &stats); err != nil {
		return nil, err
	}
	return &Collection{db: db, name: name}, nil
}

// DropCollection deletes a collection from the server.
func (db *DB) DropCollection(name string) error {
	return db.doJSON(context.Background(), http.MethodDelete, "/collections/"+name, nil, nil)
}

// Collections returns the list of collection names on the server.
func (db *DB) Collections() ([]string, error) {
	var colls []CollectionStats
	if err := db.doJSON(context.Background(), http.MethodGet, "/collections", nil, &colls); err != nil {
		return nil, err
	}
	names := make([]string, len(colls))
	for i, c := range colls {
		names[i] = c.Name
	}
	return names, nil
}

// Stats returns database-level statistics.
func (db *DB) Stats() (*DatabaseStats, error) {
	var stats DatabaseStats
	if err := db.doJSON(context.Background(), http.MethodGet, "/info", nil, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// Sync forces a sync to disk on the server.
func (db *DB) Sync() error {
	return db.doJSON(context.Background(), http.MethodPost, "/sync", nil, nil)
}

// Reload reloads the database from disk on the server, picking up changes
// written by another process.
func (db *DB) Reload() error {
	return db.doJSON(context.Background(), http.MethodPost, "/reload", nil, nil)
}

// --- Collection operations ---

// Collection is a handle to a collection on the remote server.
type Collection struct {
	db   *DB
	name string
}

// Name returns the collection name.
func (c *Collection) Name() string { return c.name }

// Stats returns collection statistics.
func (c *Collection) Stats() (*CollectionStats, error) {
	var stats CollectionStats
	if err := c.db.doJSON(context.Background(), http.MethodGet, "/collections/"+c.name, nil, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// Insert adds a single vector with optional payload to the collection.
func (c *Collection) Insert(vector []float32, payload map[string]any) (uint64, error) {
	req := map[string]any{
		"vector":  toFloat64(vector),
		"payload": payload,
	}
	var resp struct {
		ID     uint64 `json:"id"`
		Status string `json:"status"`
	}
	if err := c.db.doJSON(context.Background(), http.MethodPost, "/collections/"+c.name+"/vectors", req, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// InsertBatch adds multiple vectors with optional payloads.
func (c *Collection) InsertBatch(vectors [][]float32, payloads []map[string]any) ([]uint64, error) {
	v64 := make([][]float64, len(vectors))
	for i, v := range vectors {
		v64[i] = toFloat64(v)
	}
	req := map[string]any{
		"vectors":  v64,
		"payloads": payloads,
	}
	var resp struct {
		IDs    []uint64 `json:"ids"`
		Count  int      `json:"count"`
		Status string   `json:"status"`
	}
	if err := c.db.doJSON(context.Background(), http.MethodPost, "/collections/"+c.name+"/vectors", req, &resp); err != nil {
		return nil, err
	}
	return resp.IDs, nil
}

// Get retrieves a record by ID.
func (c *Collection) Get(id uint64) (*Record, error) {
	var resp Record
	if err := c.db.doJSON(context.Background(), http.MethodGet,
		fmt.Sprintf("/collections/%s/vectors/%d", c.name, id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Delete removes a record by ID.
func (c *Collection) Delete(id uint64) error {
	return c.db.doJSON(context.Background(), http.MethodDelete,
		fmt.Sprintf("/collections/%s/vectors/%d", c.name, id), nil, nil)
}

// UpdateVector replaces the vector for a record.
func (c *Collection) UpdateVector(id uint64, vector []float32) error {
	req := map[string]any{"vector": toFloat64(vector)}
	return c.db.doJSON(context.Background(), http.MethodPut,
		fmt.Sprintf("/collections/%s/vectors/%d", c.name, id), req, nil)
}

// Update replaces the payload for a record.
func (c *Collection) Update(id uint64, payload map[string]any) error {
	req := map[string]any{"payload": payload}
	return c.db.doJSON(context.Background(), http.MethodPut,
		fmt.Sprintf("/collections/%s/vectors/%d", c.name, id), req, nil)
}

// Upsert inserts or updates a record by ID.
func (c *Collection) Upsert(id uint64, vector []float32, payload map[string]any) (uint64, error) {
	req := map[string]any{
		"id":      id,
		"vector":  toFloat64(vector),
		"payload": payload,
	}
	var resp struct {
		ID     uint64 `json:"id"`
		Status string `json:"status"`
	}
	if err := c.db.doJSON(context.Background(), http.MethodPost, "/collections/"+c.name+"/upsert", req, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// UpsertByKey inserts or updates a record by a payload key field.
func (c *Collection) UpsertByKey(keyField string, keyValue any, vector []float32, payload map[string]any) (uint64, bool, error) {
	req := map[string]any{
		"key_field": keyField,
		"key_value": keyValue,
		"vector":     toFloat64(vector),
		"payload":    payload,
	}
	var resp struct {
		ID       uint64 `json:"id"`
		Inserted bool   `json:"inserted"`
		Status   string `json:"status"`
	}
	if err := c.db.doJSON(context.Background(), http.MethodPost, "/collections/"+c.name+"/upsert", req, &resp); err != nil {
		return 0, false, err
	}
	return resp.ID, resp.Inserted, nil
}

// Search performs a vector similarity search.
func (c *Collection) Search(query []float32, opts ...SearchOption) ([]Result, error) {
	cfg := &searchConfig{topK: 10}
	for _, o := range opts {
		o(cfg)
	}
	req := map[string]any{
		"query": toFloat64(query),
		"top_k": cfg.topK,
	}
	if cfg.threshold != nil {
		req["threshold"] = float64(*cfg.threshold)
	}
	if len(cfg.filters) > 0 {
		req["filters"] = cfg.filters
	}
	var resp struct {
		Results []Result `json:"results"`
		Count   int      `json:"count"`
	}
	if err := c.db.doJSON(context.Background(), http.MethodPost, "/collections/"+c.name+"/search", req, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// Find retrieves records matching the given filters.
func (c *Collection) Find(filters ...Filter) ([]Record, error) {
	req := map[string]any{"filters": filters}
	var resp struct {
		Results []Record `json:"results"`
		Count   int      `json:"count"`
	}
	if err := c.db.doJSON(context.Background(), http.MethodPost, "/collections/"+c.name+"/find", req, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// DeleteWhere deletes records matching the given filters.
func (c *Collection) DeleteWhere(filters ...Filter) (int, error) {
	req := map[string]any{"filters": filters}
	var resp struct {
		Deleted int    `json:"deleted"`
		Status  string `json:"status"`
	}
	if err := c.db.doJSON(context.Background(), http.MethodDelete, "/collections/"+c.name+"/vectors", req, &resp); err != nil {
		return 0, err
	}
	return resp.Deleted, nil
}

// --- helpers ---

func toFloat64(in []float32) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}