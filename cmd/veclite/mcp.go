package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/abdul-hamid-achik/veclite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer handles MCP protocol using the official Go SDK.
type MCPServer struct {
	db     *veclite.DB
	server *mcp.Server
}

func cmdMCP(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: veclite mcp <db-path>")
		os.Exit(1)
	}

	dbPath := args[0]
	db, err := veclite.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	srv := &MCPServer{db: db}
	srv.run()
}

func (s *MCPServer) run() {
	// Create MCP server with implementation info
	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "veclite",
		Version: veclite.Version,
	}, nil)

	// Register tools
	s.registerTools()

	// Run with stdio transport
	transport := &mcp.StdioTransport{}
	if err := s.server.Run(context.Background(), transport); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

// Input/output types for tools

type emptyInput struct{}

type searchInput struct {
	Collection string    `json:"collection" jsonschema:"description=Collection name,required"`
	Query      []float64 `json:"query" jsonschema:"description=Query vector,required"`
	TopK       int       `json:"top_k,omitempty" jsonschema:"description=Number of results (default 10)"`
}

type textSearchInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	Query      string `json:"query" jsonschema:"description=Text query,required"`
	TopK       int    `json:"top_k,omitempty" jsonschema:"description=Number of results (default 10)"`
}

type hybridSearchInput struct {
	Collection   string    `json:"collection" jsonschema:"description=Collection name,required"`
	Vector       []float64 `json:"vector" jsonschema:"description=Query vector,required"`
	Text         string    `json:"text" jsonschema:"description=Text query,required"`
	TopK         int       `json:"top_k,omitempty" jsonschema:"description=Number of results (default 10)"`
	VectorWeight float64   `json:"vector_weight,omitempty" jsonschema:"description=Weight for vector results (default 1.0)"`
	TextWeight   float64   `json:"text_weight,omitempty" jsonschema:"description=Weight for text results (default 1.0)"`
}

type findInput struct {
	Collection string          `json:"collection" jsonschema:"description=Collection name,required"`
	Filters    []filterRequest `json:"filters,omitempty" jsonschema:"description=Filter conditions"`
	Limit      int             `json:"limit,omitempty" jsonschema:"description=Maximum results"`
}

type insertInput struct {
	Collection string         `json:"collection" jsonschema:"description=Collection name,required"`
	Vector     []float64      `json:"vector" jsonschema:"description=Vector to insert,required"`
	Payload    map[string]any `json:"payload,omitempty" jsonschema:"description=Optional metadata"`
	Content    string         `json:"content,omitempty" jsonschema:"description=Optional text content"`
}

type memoryRememberInput struct {
	Text       string         `json:"text" jsonschema:"description=The text content to remember,required"`
	Vector     []float64      `json:"vector" jsonschema:"description=Vector embedding of the text,required"`
	Importance float64        `json:"importance,omitempty" jsonschema:"description=Importance score from 0.0 to 1.0 (default 0.5)"`
	Tags       []string       `json:"tags,omitempty" jsonschema:"description=Optional tags for categorization"`
	TTLHours   float64        `json:"ttl_hours,omitempty" jsonschema:"description=Optional time-to-live in hours (0 = never expires)"`
	Metadata   map[string]any `json:"metadata,omitempty" jsonschema:"description=Additional metadata to store"`
}

type memoryRecallInput struct {
	Query          []float64 `json:"query" jsonschema:"description=Query vector for semantic search,required"`
	Limit          int       `json:"limit,omitempty" jsonschema:"description=Maximum number of memories to return (default 10)"`
	MinImportance  float64   `json:"min_importance,omitempty" jsonschema:"description=Minimum importance score (0.0-1.0)"`
	Tags           []string  `json:"tags,omitempty" jsonschema:"description=Filter by tags (any match)"`
	SinceHours     float64   `json:"since_hours,omitempty" jsonschema:"description=Only memories created within this many hours"`
	IncludeExpired bool      `json:"include_expired,omitempty" jsonschema:"description=Include expired memories (default false)"`
}

type memoryForgetInput struct {
	OlderThanHours  float64  `json:"older_than_hours,omitempty" jsonschema:"description=Delete memories older than this many hours"`
	Tags            []string `json:"tags,omitempty" jsonschema:"description=Delete memories with any of these tags"`
	ExpiredOnly     bool     `json:"expired_only,omitempty" jsonschema:"description=Only delete expired memories"`
	BelowImportance float64  `json:"below_importance,omitempty" jsonschema:"description=Delete memories with importance below this threshold"`
}

func (s *MCPServer) registerTools() {
	// veclite_collections - List all collections
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_collections",
		Description: "List all collections in the database with their stats",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, any, error) {
		return s.toolCollections()
	})

	// veclite_stats - Get database statistics
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_stats",
		Description: "Get database statistics including collection details",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, any, error) {
		return s.toolStats()
	})

	// veclite_search - Vector similarity search
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_search",
		Description: "Search for similar vectors in a collection",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchInput) (*mcp.CallToolResult, any, error) {
		return s.toolSearch(input)
	})

	// veclite_text_search - BM25 text search
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_text_search",
		Description: "Search for records by text using BM25 full-text search",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input textSearchInput) (*mcp.CallToolResult, any, error) {
		return s.toolTextSearch(input)
	})

	// veclite_hybrid_search - Combined vector + text search
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_hybrid_search",
		Description: "Search using both vector similarity and text matching with RRF fusion",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input hybridSearchInput) (*mcp.CallToolResult, any, error) {
		return s.toolHybridSearch(input)
	})

	// veclite_find - Filter-based record retrieval
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_find",
		Description: "Find records matching filters (no vector needed)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input findInput) (*mcp.CallToolResult, any, error) {
		return s.toolFind(input)
	})

	// veclite_insert - Insert a vector
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_insert",
		Description: "Insert a vector with optional payload and content into a collection",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input insertInput) (*mcp.CallToolResult, any, error) {
		return s.toolInsert(input)
	})

	// Memory tools

	// memory_remember - Store a memory
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_remember",
		Description: "Store a memory with text content, importance score, tags, and optional TTL. Uses the 'memories' collection for agent memory.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryRememberInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryRemember(input)
	})

	// memory_recall - Recall memories via semantic search
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_recall",
		Description: "Recall memories using semantic search with optional filters for time, tags, and importance",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryRecallInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryRecall(input)
	})

	// memory_forget - Remove memories
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_forget",
		Description: "Remove memories by age, tags, or expired status",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryForgetInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryForget(input)
	})
}

// Tool implementations

func (s *MCPServer) toolCollections() (*mcp.CallToolResult, any, error) {
	names := s.db.Collections()
	type collInfo struct {
		Name      string `json:"name"`
		Count     int    `json:"count"`
		Dimension int    `json:"dimension"`
		Distance  string `json:"distance"`
		IndexType string `json:"index_type"`
	}

	result := make([]collInfo, 0, len(names))
	for _, name := range names {
		coll, err := s.db.GetCollection(name)
		if err != nil {
			continue
		}
		stats := coll.Stats()
		result = append(result, collInfo{
			Name:      name,
			Count:     stats.Count,
			Dimension: stats.Dimension,
			Distance:  stats.DistanceType,
			IndexType: stats.IndexType,
		})
	}
	return textResult(result)
}

func (s *MCPServer) toolStats() (*mcp.CallToolResult, any, error) {
	return textResult(s.db.Stats())
}

func (s *MCPServer) toolSearch(input searchInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return errorResult("Collection not found: " + input.Collection)
	}

	queryVec := float64ToFloat32(input.Query)

	opts := []veclite.SearchOption{}
	if input.TopK > 0 {
		opts = append(opts, veclite.TopK(input.TopK))
	}

	results, err := coll.Search(queryVec, opts...)
	if err != nil {
		return errorResult("Search error: " + err.Error())
	}

	return textResult(formatResults(results))
}

func (s *MCPServer) toolTextSearch(input textSearchInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return errorResult("Collection not found: " + input.Collection)
	}

	opts := []veclite.SearchOption{}
	if input.TopK > 0 {
		opts = append(opts, veclite.TopK(input.TopK))
	}

	results, err := coll.TextSearch(input.Query, opts...)
	if err != nil {
		return errorResult("Text search error: " + err.Error())
	}

	return textResult(formatResults(results))
}

func (s *MCPServer) toolHybridSearch(input hybridSearchInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return errorResult("Collection not found: " + input.Collection)
	}

	queryVec := float64ToFloat32(input.Vector)

	opts := []veclite.SearchOption{}
	if input.TopK > 0 {
		opts = append(opts, veclite.TopK(input.TopK))
	}
	if input.VectorWeight > 0 {
		opts = append(opts, veclite.WithVectorWeight(input.VectorWeight))
	}
	if input.TextWeight > 0 {
		opts = append(opts, veclite.WithTextWeight(input.TextWeight))
	}

	results, err := coll.HybridSearch(queryVec, input.Text, opts...)
	if err != nil {
		return errorResult("Hybrid search error: " + err.Error())
	}

	return textResult(formatResults(results))
}

func (s *MCPServer) toolFind(input findInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return errorResult("Collection not found: " + input.Collection)
	}

	var vecliteFilters []veclite.Filter
	for _, f := range input.Filters {
		filter := parseFilterRequest(f)
		if filter != nil {
			vecliteFilters = append(vecliteFilters, filter)
		}
	}

	records, err := coll.Find(vecliteFilters...)
	if err != nil {
		return errorResult("Find error: " + err.Error())
	}

	if input.Limit > 0 && len(records) > input.Limit {
		records = records[:input.Limit]
	}

	type recordOut struct {
		ID      uint64         `json:"id"`
		Payload map[string]any `json:"payload,omitempty"`
		Content string         `json:"content,omitempty"`
	}
	out := make([]recordOut, len(records))
	for i, r := range records {
		out[i] = recordOut{ID: r.ID, Payload: r.Payload, Content: r.Content}
	}

	return textResult(out)
}

func (s *MCPServer) toolInsert(input insertInput) (*mcp.CallToolResult, any, error) {
	coll := s.db.Collection(input.Collection)

	vec := float64ToFloat32(input.Vector)

	var id uint64
	var err error
	if input.Content != "" {
		id, err = coll.InsertDocument(vec, input.Content, input.Payload)
	} else {
		id, err = coll.Insert(vec, input.Payload)
	}
	if err != nil {
		return errorResult("Insert error: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{"id": id, "status": "inserted"})
}

// Memory tools implementations

const memoriesCollection = "memories"

func (s *MCPServer) toolMemoryRemember(input memoryRememberInput) (*mcp.CallToolResult, any, error) {
	if input.Text == "" {
		return errorResult("Text is required")
	}
	if len(input.Vector) == 0 {
		return errorResult("Vector is required")
	}

	coll := s.db.Collection(memoriesCollection)
	vec := float64ToFloat32(input.Vector)

	// Build payload
	payload := make(map[string]any)
	if input.Metadata != nil {
		for k, v := range input.Metadata {
			payload[k] = v
		}
	}
	if len(input.Tags) > 0 {
		payload["_tags"] = input.Tags
	}

	// Build insert options
	opts := []veclite.InsertOption{
		veclite.WithContentOption(input.Text),
	}

	// Set importance (default 0.5)
	imp := float32(0.5)
	if input.Importance > 0 {
		imp = float32(input.Importance)
	}
	opts = append(opts, veclite.WithImportance(imp))

	// Set TTL if specified
	if input.TTLHours > 0 {
		ttl := time.Duration(input.TTLHours * float64(time.Hour))
		opts = append(opts, veclite.WithTTL(ttl))
	}

	id, err := coll.InsertWithOptions(vec, payload, opts...)
	if err != nil {
		return errorResult("Failed to store memory: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"id":         id,
		"status":     "remembered",
		"importance": imp,
		"has_ttl":    input.TTLHours > 0,
	})
}

func (s *MCPServer) toolMemoryRecall(input memoryRecallInput) (*mcp.CallToolResult, any, error) {
	if len(input.Query) == 0 {
		return errorResult("Query vector is required")
	}

	coll, err := s.db.GetCollection(memoriesCollection)
	if err != nil {
		return errorResult("Memories collection not found. Use memory_remember first.")
	}

	queryVec := float64ToFloat32(input.Query)

	// Build search options
	limit := 10
	if input.Limit > 0 {
		limit = input.Limit
	}
	opts := []veclite.SearchOption{veclite.TopK(limit)}

	// Build filters
	var filters []veclite.Filter

	if input.MinImportance > 0 {
		filters = append(filters, veclite.ImportanceAbove(float32(input.MinImportance)))
	}

	if input.SinceHours > 0 {
		since := time.Duration(input.SinceHours * float64(time.Hour))
		filters = append(filters, veclite.AgeNewerThan(since))
	}

	if !input.IncludeExpired {
		filters = append(filters, veclite.NotExpired())
	}

	for _, f := range filters {
		opts = append(opts, veclite.WithFilter(f))
	}

	results, err := coll.Search(queryVec, opts...)
	if err != nil {
		return errorResult("Recall error: " + err.Error())
	}

	// Post-filter by tags if specified
	if len(input.Tags) > 0 {
		filtered := make([]veclite.Result, 0, len(results))
		for _, r := range results {
			if hasAnyTag(r.Record.Payload, input.Tags) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Format results with memory-specific fields
	memories := make([]map[string]any, len(results))
	for i, r := range results {
		memory := map[string]any{
			"id":         r.Record.ID,
			"score":      r.Score,
			"text":       r.Record.Content,
			"importance": r.Record.Importance,
			"created_at": r.Record.CreatedAt.Format(time.RFC3339),
		}
		if r.Record.Payload != nil {
			if t, ok := r.Record.Payload["_tags"]; ok {
				memory["tags"] = t
			}
			for k, v := range r.Record.Payload {
				if k != "_tags" {
					memory[k] = v
				}
			}
		}
		if r.Record.HasTTL() {
			memory["expires_at"] = r.Record.ExpiresAt.Format(time.RFC3339)
			memory["ttl_remaining"] = r.Record.TTL().String()
		}
		memories[i] = memory
	}

	return textResult(map[string]any{
		"count":    len(memories),
		"memories": memories,
	})
}

func (s *MCPServer) toolMemoryForget(input memoryForgetInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(memoriesCollection)
	if err != nil {
		return errorResult("Memories collection not found")
	}

	var filters []veclite.Filter

	if input.ExpiredOnly {
		filters = append(filters, veclite.FilterFunc(func(r *veclite.Record) bool {
			return r.IsExpired()
		}))
	}

	if input.OlderThanHours > 0 {
		age := time.Duration(input.OlderThanHours * float64(time.Hour))
		filters = append(filters, veclite.AgeOlderThan(age))
	}

	if input.BelowImportance > 0 {
		filters = append(filters, veclite.ImportanceBelow(float32(input.BelowImportance)))
	}

	if len(input.Tags) > 0 {
		filters = append(filters, veclite.FilterFunc(func(r *veclite.Record) bool {
			return hasAnyTag(r.Payload, input.Tags)
		}))
	}

	if len(filters) == 0 {
		return errorResult("At least one filter criteria is required to forget memories")
	}

	deleted, err := coll.DeleteWhere(filters...)
	if err != nil {
		return errorResult("Forget error: " + err.Error())
	}

	if deleted > 0 {
		_ = s.db.Sync()
	}

	return textResult(map[string]any{
		"status":  "forgotten",
		"deleted": deleted,
	})
}

// Helper functions

func textResult(data any) (*mcp.CallToolResult, any, error) {
	b, _ := json.MarshalIndent(data, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, nil, nil
}

func errorResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
		IsError: true,
	}, nil, nil
}

func float64ToFloat32(v []float64) []float32 {
	result := make([]float32, len(v))
	for i, f := range v {
		result[i] = float32(f)
	}
	return result
}

func formatResults(results []veclite.Result) []map[string]any {
	out := make([]map[string]any, len(results))
	for i, r := range results {
		entry := map[string]any{
			"id":      r.Record.ID,
			"score":   r.Score,
			"payload": r.Record.Payload,
		}
		if r.Record.Content != "" {
			entry["content"] = r.Record.Content
		}
		out[i] = entry
	}
	return out
}

func hasAnyTag(payload map[string]any, tags []string) bool {
	if payload == nil {
		return false
	}
	tagsVal, ok := payload["_tags"]
	if !ok {
		return false
	}

	switch t := tagsVal.(type) {
	case []string:
		for _, pt := range t {
			for _, tag := range tags {
				if pt == tag {
					return true
				}
			}
		}
	case []any:
		for _, pt := range t {
			if ptStr, ok := pt.(string); ok {
				for _, tag := range tags {
					if ptStr == tag {
						return true
					}
				}
			}
		}
	}
	return false
}
