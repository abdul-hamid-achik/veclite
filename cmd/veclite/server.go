package main

import (
	"context"
	"encoding/json"
	"fmt"
	"flag"
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
	db       *veclite.DB
	dbPath   string
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

func cmdServe(args []string) {
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
		fmt.Println("  GET    /collections                     List collections")
		fmt.Println("  POST   /collections                     Create collection")
		fmt.Println("  GET    /collections/{name}              Collection info")
		fmt.Println("  DELETE /collections/{name}              Drop collection")
		fmt.Println("  POST   /collections/{name}/vectors      Insert vector(s)")
		fmt.Println("  GET    /collections/{name}/vectors/{id} Get vector by ID")
		fmt.Println("  DELETE /collections/{name}/vectors/{id} Delete vector")
		fmt.Println("  POST   /collections/{name}/search       Search vectors")
		fmt.Println("  POST   /sync                            Force sync to disk")
		fmt.Println("\nExamples:")
		fmt.Println("  veclite serve data.veclite --port=8080")
		fmt.Println("  veclite serve data.veclite --host=0.0.0.0 --port=3000 --cors")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	// Open database
	db, err := veclite.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	server := &Server{
		db:          db,
		dbPath:      path,
		corsEnabled: *cors,
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/info", server.handleInfo)
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

		if err := db.Close(); err != nil {
			log.Printf("Database close error: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}

	<-done
	fmt.Println("Server stopped")
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
		Name           string `json:"name"`
		Dimension      int    `json:"dimension"`
		Distance       string `json:"distance"`
		HNSW           bool   `json:"hnsw"`
		HNSWM          int    `json:"hnsw_m"`
		HNSWEf         int    `json:"hnsw_ef"`
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
			if r.Method == "POST" {
				s.insertVectors(w, r, collName)
			} else {
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
			case "DELETE":
				s.deleteVector(w, r, collName, id)
			default:
				writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		}
	}

	if parts[1] == "search" && len(parts) == 2 {
		if r.Method == "POST" {
			s.searchVectors(w, r, collName)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		}
		return
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

	if req.Query == nil || len(req.Query) == 0 {
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
