package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/abdul-hamid-achik/veclite"
)

// toFloat32 converts a JSON numeric array to []float32.
func toFloat32(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}

// listVectorSpaces handles GET /collections/{name}/spaces
func (s *Server) listVectorSpaces(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	type spaceOut struct {
		Name        string `json:"name"`
		Dimension   int    `json:"dimension"`
		Distance    string `json:"distance"`
		Modality    string `json:"modality,omitempty"`
		Provider    string `json:"provider,omitempty"`
		Model       string `json:"model,omitempty"`
		IndexType   string `json:"index_type"`
		VectorCount int    `json:"vector_count"`
	}

	spaces := coll.VectorSpaces()
	out := make([]spaceOut, len(spaces))
	for i, sp := range spaces {
		out[i] = spaceOut{
			Name:        sp.Name,
			Dimension:   sp.Dimension,
			Distance:    string(sp.Distance),
			Modality:    sp.Modality,
			Provider:    sp.Provider,
			Model:       sp.Model,
			IndexType:   sp.IndexType,
			VectorCount: sp.VectorCount,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"spaces": out, "count": len(out)})
}

// addVectorSpace handles POST /collections/{name}/spaces
func (s *Server) addVectorSpace(w http.ResponseWriter, r *http.Request, collName string) {
	coll := s.db.Collection(collName)

	var req struct {
		Name      string `json:"name"`
		Dimension int    `json:"dimension"`
		Distance  string `json:"distance"`
		Modality  string `json:"modality"`
		Provider  string `json:"provider"`
		Model     string `json:"model"`
		HNSW      bool   `json:"hnsw"`
		HNSWM     int    `json:"hnsw_m"`
		HNSWEf    int    `json:"hnsw_ef"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Vector space name is required", "MISSING_NAME")
		return
	}

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

	cfg := veclite.VectorSpaceConfig{
		Name:      req.Name,
		Dimension: req.Dimension,
		Distance:  distType,
		Modality:  req.Modality,
		Provider:  req.Provider,
		Model:     req.Model,
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
		cfg.HNSW = &veclite.HNSWConfig{M: m, EfConstruction: ef, EfSearch: 100, UseHeuristic: true}
	}

	if err := coll.AddVectorSpace(cfg); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "VECTOR_SPACE_ERROR")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "created",
		"collection": collName,
		"space":      req.Name,
	})
}

// insertRecord handles POST /collections/{name}/records
func (s *Server) insertRecord(w http.ResponseWriter, r *http.Request, collName string) {
	coll := s.db.Collection(collName)

	var req struct {
		ID      uint64               `json:"id"`
		Content string               `json:"content"`
		Payload map[string]any       `json:"payload"`
		Vectors map[string][]float64 `json:"vectors"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}
	if len(req.Vectors) == 0 && req.Content == "" {
		writeError(w, http.StatusBadRequest, "At least one of 'vectors' or 'content' is required", "MISSING_VECTORS")
		return
	}

	vectors := make(map[string][]float32, len(req.Vectors))
	for name, vec := range req.Vectors {
		vectors[name] = toFloat32(vec)
	}

	id, err := coll.InsertRecord(veclite.RecordInput{
		ID:      req.ID,
		Content: req.Content,
		Payload: req.Payload,
		Vectors: vectors,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INSERT_ERROR")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "inserted", "id": id})
}

// searchVectorSpace handles POST /collections/{name}/search-space
func (s *Server) searchVectorSpace(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	var req struct {
		Space     string          `json:"space"`
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

	opts := buildSearchOpts(req.TopK, req.Threshold, req.Filters)
	results, err := coll.SearchSpace(req.Space, toFloat32(req.Query), opts...)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "SEARCH_ERROR")
		return
	}
	writeSearchResults(w, results)
}

// fuseSearch handles POST /collections/{name}/fuse-search
func (s *Server) fuseSearch(w http.ResponseWriter, r *http.Request, collName string) {
	coll, err := s.db.GetCollection(collName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Collection not found", "NOT_FOUND")
		return
	}

	var req struct {
		Queries map[string][]float64 `json:"queries"`
		TopK    int                  `json:"top_k"`
		Filters []filterRequest      `json:"filters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}
	if len(req.Queries) == 0 {
		writeError(w, http.StatusBadRequest, "At least one query is required", "MISSING_QUERIES")
		return
	}

	queries := make(map[string][]float32, len(req.Queries))
	for name, vec := range req.Queries {
		queries[name] = toFloat32(vec)
	}

	opts := buildSearchOpts(req.TopK, nil, req.Filters)
	results, err := coll.MultiSpaceSearch(queries, opts...)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "SEARCH_ERROR")
		return
	}
	writeSearchResults(w, results)
}

// buildSearchOpts assembles common search options from request fields.
func buildSearchOpts(topK int, threshold *float64, filters []filterRequest) []veclite.SearchOption {
	opts := []veclite.SearchOption{}
	if topK > 0 {
		opts = append(opts, veclite.TopK(topK))
	}
	if threshold != nil {
		opts = append(opts, veclite.Threshold(float32(*threshold)))
	}
	for _, f := range filters {
		if filter := parseFilterRequest(f); filter != nil {
			opts = append(opts, veclite.WithFilter(filter))
		}
	}
	return opts
}

// writeSearchResults renders search results in the standard envelope.
func writeSearchResults(w http.ResponseWriter, results []veclite.Result) {
	type resultOutput struct {
		ID      uint64         `json:"id"`
		Score   float32        `json:"score"`
		Payload map[string]any `json:"payload,omitempty"`
	}
	out := make([]resultOutput, len(results))
	for i, r := range results {
		out[i] = resultOutput{ID: r.Record.ID, Score: r.Score, Payload: r.Record.Payload}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out, "count": len(out)})
}
