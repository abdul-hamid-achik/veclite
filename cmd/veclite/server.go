package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/veclite"
)

// Server holds the HTTP server state.
type Server struct {
	db          *veclite.DB
	dbPath      string
	corsEnabled bool
}

// APIError represents an error response.
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, APIError{Error: message, Code: code})
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "HTTP port to listen on")
	host := fs.String("host", "127.0.0.1", "Host to bind to")
	cors := fs.Bool("cors", false, "Enable CORS headers for cross-origin requests")
	fs.Usage = func() {
		fmt.Println("Usage: veclite serve [options] <file>")
		fmt.Println("\nStart an HTTP server for multi-client access to the database.")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		fmt.Println("\nEndpoints:")
		fmt.Println("  GET    /health                          Health check")
		fmt.Println("  GET    /info                            Database info")
		fmt.Println("  GET    /metrics                         Database metrics")
		fmt.Println("  GET    /collections                     List collections")
		fmt.Println("  POST   /collections                     Create collection")
		fmt.Println("  GET    /collections/{name}              Collection info")
		fmt.Println("  DELETE /collections/{name}              Drop collection")
		fmt.Println("  GET    /collections/{name}/vectors      List all vectors (with pagination)")
		fmt.Println("  POST   /collections/{name}/vectors      Insert vector(s)")
		fmt.Println("  GET    /collections/{name}/vectors/{id} Get vector by ID")
		fmt.Println("  PUT    /collections/{name}/vectors/{id} Update vector and/or payload")
		fmt.Println("  DELETE /collections/{name}/vectors/{id} Delete vector")
		fmt.Println("  POST   /collections/{name}/search       Search vectors")
		fmt.Println("  GET    /collections/{name}/spaces       List vector spaces")
		fmt.Println("  POST   /collections/{name}/spaces       Add a named vector space")
		fmt.Println("  POST   /collections/{name}/records      Insert a multi-space record")
		fmt.Println("  POST   /collections/{name}/search-space Search one named vector space")
		fmt.Println("  POST   /collections/{name}/fuse-search  Fuse search across vector spaces")
		fmt.Println("  POST   /collections/{name}/upsert       Upsert vector")
		fmt.Println("  POST   /collections/{name}/find         Find records by filter")
		fmt.Println("  DELETE /collections/{name}/vectors       Delete vectors by filter")
		fmt.Println("  POST   /collections/{name}/compact      Compact collection")
		fmt.Println("  POST   /collections/{name}/validate     Validate collection integrity")
		fmt.Println("  POST   /sync                            Force sync to disk")
		fmt.Println("\nExamples:")
		fmt.Println("  veclite serve data.veclite --port=8080")
		fmt.Println("  veclite serve data.veclite --host=0.0.0.0 --port=3000 --cors")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing required argument: file")
	}

	path := fs.Arg(0)

	// Open database
	db, err := veclite.Open(path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	server := &Server{
		db:          db,
		dbPath:      path,
		corsEnabled: *cors,
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/info", server.handleInfo)
	mux.HandleFunc("/metrics", server.handleMetrics)
	mux.HandleFunc("/collections", server.handleCollections)
	mux.HandleFunc("/collections/", server.handleCollection)
	mux.HandleFunc("/sync", server.handleSync)

	// Wrap with CORS middleware if enabled
	var handler http.Handler = mux
	if *cors {
		handler = server.corsMiddleware(mux)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v\n", err)
		}

		close(done)
	}()

	fmt.Printf("Starting VecLite server on %s\n", addr)
	fmt.Printf("Database: %s\n", path)
	if *cors {
		fmt.Println("CORS: enabled")
	}
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	<-done
	fmt.Println("Server stopped")
	return nil
}

// corsMiddleware adds CORS headers.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": veclite.Version,
	})
}

// handleInfo handles GET /info
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	stats := s.db.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"path":          s.dbPath,
		"collections":   stats.Collections,
		"total_records": stats.TotalRecords,
		"version":       veclite.Version,
	})
}

// handleCollections handles /collections
func (s *Server) handleCollections(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.listCollections(w, r)
	case "POST":
		s.createCollection(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
	}
}

// listCollections handles GET /collections
func (s *Server) listCollections(w http.ResponseWriter, r *http.Request) {
	collections := s.db.Collections()

	type collInfo struct {
		Name      string `json:"name"`
		Count     int    `json:"count"`
		Dimension int    `json:"dimension"`
		Distance  string `json:"distance"`
		IndexType string `json:"index_type"`
	}

	result := make([]collInfo, 0, len(collections))
	for _, name := range collections {
		coll, _ := s.db.GetCollection(name)
		stats := coll.Stats()
		result = append(result, collInfo{
			Name:      name,
			Count:     stats.Count,
			Dimension: stats.Dimension,
			Distance:  stats.DistanceType,
			IndexType: stats.IndexType,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// createCollection handles POST /collections
func (s *Server) createCollection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Dimension int    `json:"dimension"`
		Distance  string `json:"distance"`
		HNSW      bool   `json:"hnsw"`
		HNSWM     int    `json:"hnsw_m"`
		HNSWEf    int    `json:"hnsw_ef"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Collection name is required", "MISSING_NAME")
		return
	}

	// Parse distance type
	var distType veclite.DistanceType
	switch strings.ToLower(req.Distance) {
	case "", "cosine":
		distType = veclite.DistanceCosine
	case "dot":
		distType = veclite.DistanceDot
	case "euclidean":
		distType = veclite.DistanceEuclidean
	default:
		writeError(w, http.StatusBadRequest, "Invalid distance type", "INVALID_DISTANCE")
		return
	}

	// Build options
	opts := []veclite.CollectionOption{
		veclite.WithDistanceType(distType),
	}
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

	_, err := s.db.CreateCollection(req.Name, opts...)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error(), "COLLECTION_EXISTS")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "created",
		"collection": req.Name,
	})
}

// handleCollection routes /collections/{name}/*
func (s *Server) handleCollection(w http.ResponseWriter, r *http.Request) {
	// Parse path: /collections/{name} or /collections/{name}/vectors or /collections/{name}/vectors/{id} or /collections/{name}/search
	path := strings.TrimPrefix(r.URL.Path, "/collections/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "Collection name required", "MISSING_NAME")
		return
	}

	collName := parts[0]

	if len(parts) == 1 {
		// /collections/{name}
		switch r.Method {
		case "GET":
			s.getCollectionInfo(w, r, collName)
		case "DELETE":
			s.dropCollection(w, r, collName)
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		}
		return
	}

	if parts[1] == "vectors" {
		if len(parts) == 2 {
			// /collections/{name}/vectors
			switch r.Method {
			case "GET":
				s.listVectors(w, r, collName)
			case "POST":
				s.insertVectors(w, r, collName)
			case "DELETE":
				s.deleteWhere(w, r, collName)
			default:
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		}

		if len(parts) == 3 {
			// /collections/{name}/vectors/{id}
			id, err := strconv.ParseUint(parts[2], 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid vector ID", "INVALID_ID")
				return
			}

			switch r.Method {
			case "GET":
				s.getVector(w, r, collName, id)
			case "PUT":
				s.updateVector(w, r, collName, id)
			case "DELETE":
				s.deleteVector(w, r, collName, id)
			default:
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		}
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "spaces":
			switch r.Method {
			case "GET":
				s.listVectorSpaces(w, r, collName)
			case "POST":
				s.addVectorSpace(w, r, collName)
			default:
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "records":
			if r.Method == "POST" {
				s.insertRecord(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "search-space":
			if r.Method == "POST" {
				s.searchVectorSpace(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "fuse-search":
			if r.Method == "POST" {
				s.fuseSearch(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "search":
			if r.Method == "POST" {
				s.searchVectors(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "upsert":
			if r.Method == "POST" {
				s.upsertVector(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "find":
			if r.Method == "POST" {
				s.findRecords(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "compact":
			if r.Method == "POST" {
				s.compactCollection(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		case "validate":
			if r.Method == "POST" {
				s.validateCollection(w, r, collName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		}
	}

	writeError(w, http.StatusNotFound, "Not found", "NOT_FOUND")
}

// getCollectionInfo handles GET /collections/{name}
func (s *Server) getCollectionInfo(w http.ResponseWriter, r *http.Request, name string) {
	coll, err := s.db.GetCollection(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	stats := coll.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"name":       stats.Name,
		"count":      stats.Count,
		"dimension":  stats.Dimension,
		"distance":   stats.DistanceType,
		"index_type": stats.IndexType,
	})
}

// dropCollection handles DELETE /collections/{name}
func (s *Server) dropCollection(w http.ResponseWriter, r *http.Request, name string) {
	err := s.db.DropCollection(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "dropped",
		"collection": name,
	})
}

// insertVectors handles POST /collections/{name}/vectors
func (s *Server) insertVectors(w http.ResponseWriter, r *http.Request, collName string) {
	coll := s.db.Collection(collName)

	var req struct {
		Vector   []float64        `json:"vector"`
		Vectors  [][]float64      `json:"vectors"`
		Payload  map[string]any   `json:"payload"`
		Payloads []map[string]any `json:"payloads"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	// Single vector insert
	if req.Vector != nil {
		vector := make([]float32, len(req.Vector))
		for i, v := range req.Vector {
			vector[i] = float32(v)
		}

		id, err := coll.Insert(vector, req.Payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "INSERT_ERROR")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"status": "inserted",
			"id":     id,
		})
		return
	}

	// Batch insert
	if req.Vectors != nil {
		vectors := make([][]float32, len(req.Vectors))
		for i, v := range req.Vectors {
			vectors[i] = make([]float32, len(v))
			for j, val := range v {
				vectors[i][j] = float32(val)
			}
		}

		ids, err := coll.InsertBatch(vectors, req.Payloads)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "INSERT_ERROR")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"status": "inserted",
			"count":  len(ids),
			"ids":    ids,
		})
		return
	}

	writeError(w, http.StatusBadRequest, "Either 'vector' or 'vectors' is required", "MISSING_VECTOR")
}

// getVector handles GET /collections/{name}/vectors/{id}
func (s *Server) getVector(w http.ResponseWriter, r *http.Request, collName string, id uint64) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	record, err := coll.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vector not found", "NOT_FOUND")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         record.ID,
		"vector":     record.Vector,
		"payload":    record.Payload,
		"created_at": record.CreatedAt,
		"updated_at": record.UpdatedAt,
	})
}

// deleteVector handles DELETE /collections/{name}/vectors/{id}
func (s *Server) deleteVector(w http.ResponseWriter, r *http.Request, collName string, id uint64) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	err = coll.Delete(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vector not found", "NOT_FOUND")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "deleted",
		"id":     id,
	})
}

// searchVectors handles POST /collections/{name}/search
func (s *Server) searchVectors(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	var req struct {
		Query     []float64       `json:"query"`
		TopK      int             `json:"top_k"`
		Threshold *float64        `json:"threshold"`
		Filters   []filterRequest `json:"filters"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	if len(req.Query) == 0 {
		writeError(w, http.StatusBadRequest, "Query vector is required", "MISSING_QUERY")
		return
	}

	query := make([]float32, len(req.Query))
	for i, v := range req.Query {
		query[i] = float32(v)
	}

	// Build search options
	searchOpts := []veclite.SearchOption{}
	if req.TopK > 0 {
		searchOpts = append(searchOpts, veclite.TopK(req.TopK))
	}
	if req.Threshold != nil {
		searchOpts = append(searchOpts, veclite.Threshold(float32(*req.Threshold)))
	}

	// Apply filters
	for _, f := range req.Filters {
		filter := parseFilterRequest(f)
		if filter != nil {
			searchOpts = append(searchOpts, veclite.WithFilter(filter))
		}
	}

	results, err := coll.Search(query, searchOpts...)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "SEARCH_ERROR")
		return
	}

	type resultOutput struct {
		ID      uint64         `json:"id"`
		Score   float32        `json:"score"`
		Payload map[string]any `json:"payload,omitempty"`
		Vector  []float32      `json:"vector,omitempty"`
	}

	output := make([]resultOutput, len(results))
	for i, r := range results {
		output[i] = resultOutput{
			ID:      r.Record.ID,
			Score:   r.Score,
			Payload: r.Record.Payload,
		}
	}

	// Check for NDJSON streaming request
	if r.Header.Get("Accept") == "application/x-ndjson" {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		for _, o := range output {
			_ = enc.Encode(o)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": output,
		"count":   len(output),
	})
}

// filterRequest represents a filter in the API.
type filterRequest struct {
	Key   string `json:"key"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// parseFilterRequest converts an API filter to a veclite filter.
func parseFilterRequest(f filterRequest) veclite.Filter {
	switch f.Op {
	case "", "eq", "=":
		return veclite.Equal(f.Key, f.Value)
	case "neq", "!=":
		return veclite.NotEqual(f.Key, f.Value)
	case "gt", ">":
		if n, ok := filterNumber(f.Value); ok {
			return veclite.GT(f.Key, n)
		}
	case "gte", ">=":
		if n, ok := filterNumber(f.Value); ok {
			return veclite.GTE(f.Key, n)
		}
	case "lt", "<":
		if n, ok := filterNumber(f.Value); ok {
			return veclite.LT(f.Key, n)
		}
	case "lte", "<=":
		if n, ok := filterNumber(f.Value); ok {
			return veclite.LTE(f.Key, n)
		}
	case "glob":
		if s, ok := f.Value.(string); ok {
			return veclite.Glob(f.Key, s)
		}
	case "prefix":
		if s, ok := f.Value.(string); ok {
			return veclite.Prefix(f.Key, s)
		}
	case "suffix":
		if s, ok := f.Value.(string); ok {
			return veclite.Suffix(f.Key, s)
		}
	case "contains":
		if s, ok := f.Value.(string); ok {
			return veclite.Contains(f.Key, s)
		}
	case "exists":
		return veclite.Exists(f.Key)
	}
	return nil
}

// filterNumber coerces a JSON filter value to float64 for numeric comparison
// operators (JSON numbers decode to float64).
func filterNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// handleMetrics handles GET /metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	m := s.db.Metrics()
	writeJSON(w, http.StatusOK, map[string]any{
		"search_count":       m.SearchCount,
		"insert_count":       m.InsertCount,
		"delete_count":       m.DeleteCount,
		"avg_search_time_ns": int64(m.AvgSearchTime),
	})
}

// listVectors handles GET /collections/{name}/vectors
func (s *Server) listVectors(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	// Parse pagination params
	offset := 0
	limit := 100
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	it := coll.Iterate(veclite.IterOffset(offset), veclite.IterLimit(limit))
	defer it.Close()

	type recordOutput struct {
		ID      uint64         `json:"id"`
		Vector  []float32      `json:"vector"`
		Payload map[string]any `json:"payload,omitempty"`
		Content string         `json:"content,omitempty"`
	}

	records := make([]recordOutput, 0)
	for {
		rec, ok := it.Next()
		if !ok {
			break
		}
		records = append(records, recordOutput{
			ID:      rec.ID,
			Vector:  rec.Vector,
			Payload: rec.Payload,
			Content: rec.Content,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"records": records,
		"count":   len(records),
		"offset":  offset,
		"limit":   limit,
	})
}

// updateVector handles PUT /collections/{name}/vectors/{id}
func (s *Server) updateVector(w http.ResponseWriter, r *http.Request, collName string, id uint64) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	var req struct {
		Vector  []float64      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	if req.Vector != nil {
		vector := make([]float32, len(req.Vector))
		for i, v := range req.Vector {
			vector[i] = float32(v)
		}
		if err := coll.UpdateVector(id, vector); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "UPDATE_ERROR")
			return
		}
	}

	if req.Payload != nil {
		if err := coll.Update(id, req.Payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "UPDATE_ERROR")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "updated",
		"id":     id,
	})
}

// upsertVector handles POST /collections/{name}/upsert
func (s *Server) upsertVector(w http.ResponseWriter, r *http.Request, collName string) {
	coll := s.db.Collection(collName)

	var req struct {
		ID       uint64         `json:"id"`
		Vector   []float64      `json:"vector"`
		Payload  map[string]any `json:"payload"`
		KeyField string         `json:"key_field"`
		KeyValue any            `json:"key_value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	if req.Vector == nil {
		writeError(w, http.StatusBadRequest, "Vector is required", "MISSING_VECTOR")
		return
	}

	vector := make([]float32, len(req.Vector))
	for i, v := range req.Vector {
		vector[i] = float32(v)
	}

	if req.KeyField != "" {
		id, inserted, err := coll.UpsertByKey(req.KeyField, req.KeyValue, vector, req.Payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "UPSERT_ERROR")
			return
		}
		action := "updated"
		if inserted {
			action = "inserted"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": action,
			"id":     id,
		})
		return
	}

	id, err := coll.Upsert(req.ID, vector, req.Payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "UPSERT_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "upserted",
		"id":     id,
	})
}

// findRecords handles POST /collections/{name}/find
func (s *Server) findRecords(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	var req struct {
		Filters []filterRequest `json:"filters"`
		Limit   int             `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	var filters []veclite.Filter
	for _, f := range req.Filters {
		filter := parseFilterRequest(f)
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	records, err := coll.Find(filters...)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "FIND_ERROR")
		return
	}

	if req.Limit > 0 && len(records) > req.Limit {
		records = records[:req.Limit]
	}

	type recordOutput struct {
		ID      uint64         `json:"id"`
		Payload map[string]any `json:"payload,omitempty"`
		Content string         `json:"content,omitempty"`
	}

	output := make([]recordOutput, len(records))
	for i, rec := range records {
		output[i] = recordOutput{
			ID:      rec.ID,
			Payload: rec.Payload,
			Content: rec.Content,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": output,
		"count":   len(output),
	})
}

// deleteWhere handles DELETE /collections/{name}/vectors with body
func (s *Server) deleteWhere(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	var req struct {
		Filters []filterRequest `json:"filters"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	if len(req.Filters) == 0 {
		writeError(w, http.StatusBadRequest, "At least one filter is required", "MISSING_FILTERS")
		return
	}

	var filters []veclite.Filter
	for _, f := range req.Filters {
		filter := parseFilterRequest(f)
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	deleted, err := coll.DeleteWhere(filters...)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "DELETE_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "deleted",
		"deleted": deleted,
	})
}

// compactCollection handles POST /collections/{name}/compact
func (s *Server) compactCollection(w http.ResponseWriter, r *http.Request, collName string) {
	_, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	if err := s.db.Sync(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "COMPACT_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "compacted",
		"collection": collName,
	})
}

// validateCollection handles POST /collections/{name}/validate
func (s *Server) validateCollection(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	issues := make([]string, 0)
	records := coll.All()
	stats := coll.Stats()

	if len(records) > 0 {
		expectedDim := len(records[0].Vector)
		for _, rec := range records[1:] {
			if len(rec.Vector) != expectedDim {
				issues = append(issues, fmt.Sprintf("record %d has dimension %d, expected %d",
					rec.ID, len(rec.Vector), expectedDim))
			}
		}
		if stats.Dimension != 0 && stats.Dimension != expectedDim {
			issues = append(issues, fmt.Sprintf("stats dimension (%d) doesn't match actual (%d)",
				stats.Dimension, expectedDim))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"collection": collName,
		"valid":      len(issues) == 0,
		"issues":     issues,
		"count":      stats.Count,
	})
}

// handleSync handles POST /sync
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	if err := s.db.Sync(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "SYNC_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "synced",
	})
}
