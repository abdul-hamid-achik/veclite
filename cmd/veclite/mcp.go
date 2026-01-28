package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/abdul-hamid-achik/veclite"
)

// MCP JSON-RPC types
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Result  any         `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol types
type mcpToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPServer handles MCP protocol over stdio.
type MCPServer struct {
	db *veclite.DB
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

	server := &MCPServer{db: db}
	server.run()
}

func (s *MCPServer) run() {
	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer size for large messages
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &rpcError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		resp := s.handleRequest(req)
		_ = enc.Encode(resp)
	}
}

func (s *MCPServer) handleRequest(req jsonrpcRequest) jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "veclite",
					"version": veclite.Version,
				},
			},
		}

	case "notifications/initialized":
		// No response needed for notifications
		return jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}

	case "tools/list":
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": s.listTools()},
		}

	case "tools/call":
		return s.handleToolCall(req)

	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func (s *MCPServer) listTools() []mcpToolInfo {
	return []mcpToolInfo{
		{
			Name:        "veclite_collections",
			Description: "List all collections in the database with their stats",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "veclite_stats",
			Description: "Get database statistics including collection details",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "veclite_search",
			Description: "Search for similar vectors in a collection",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "Collection name"},
					"query":      map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Query vector"},
					"top_k":      map[string]any{"type": "integer", "description": "Number of results (default 10)"},
				},
				"required": []string{"collection", "query"},
			},
		},
		{
			Name:        "veclite_text_search",
			Description: "Search for records by text using BM25 full-text search",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "Collection name"},
					"query":      map[string]any{"type": "string", "description": "Text query"},
					"top_k":      map[string]any{"type": "integer", "description": "Number of results (default 10)"},
				},
				"required": []string{"collection", "query"},
			},
		},
		{
			Name:        "veclite_hybrid_search",
			Description: "Search using both vector similarity and text matching with RRF fusion",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection":    map[string]any{"type": "string", "description": "Collection name"},
					"vector":        map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Query vector"},
					"text":          map[string]any{"type": "string", "description": "Text query"},
					"top_k":         map[string]any{"type": "integer", "description": "Number of results (default 10)"},
					"vector_weight": map[string]any{"type": "number", "description": "Weight for vector results (default 1.0)"},
					"text_weight":   map[string]any{"type": "number", "description": "Weight for text results (default 1.0)"},
				},
				"required": []string{"collection", "vector", "text"},
			},
		},
		{
			Name:        "veclite_find",
			Description: "Find records matching filters (no vector needed)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "Collection name"},
					"filters": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"key":   map[string]any{"type": "string"},
								"op":    map[string]any{"type": "string", "enum": []string{"eq", "neq", "glob", "prefix", "suffix", "contains", "exists"}},
								"value": map[string]any{},
							},
						},
						"description": "Filter conditions",
					},
					"limit": map[string]any{"type": "integer", "description": "Maximum results"},
				},
				"required": []string{"collection"},
			},
		},
		{
			Name:        "veclite_insert",
			Description: "Insert a vector with optional payload and content into a collection",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "Collection name"},
					"vector":     map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Vector to insert"},
					"payload":    map[string]any{"type": "object", "description": "Optional metadata"},
					"content":    map[string]any{"type": "string", "description": "Optional text content"},
				},
				"required": []string{"collection", "vector"},
			},
		},
	}
}

func (s *MCPServer) handleToolCall(req jsonrpcRequest) jsonrpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "Invalid params"},
		}
	}

	result := s.executeTool(params.Name, params.Arguments)
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *MCPServer) executeTool(name string, argsJSON json.RawMessage) mcpToolResult {
	switch name {
	case "veclite_collections":
		return s.toolCollections()
	case "veclite_stats":
		return s.toolStats()
	case "veclite_search":
		return s.toolSearch(argsJSON)
	case "veclite_text_search":
		return s.toolTextSearch(argsJSON)
	case "veclite_hybrid_search":
		return s.toolHybridSearch(argsJSON)
	case "veclite_find":
		return s.toolFind(argsJSON)
	case "veclite_insert":
		return s.toolInsert(argsJSON)
	default:
		return mcpToolResult{
			Content: []mcpContent{{Type: "text", Text: "Unknown tool: " + name}},
			IsError: true,
		}
	}
}

func (s *MCPServer) textResult(data any) mcpToolResult {
	b, _ := json.MarshalIndent(data, "", "  ")
	return mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: string(b)}},
	}
}

func (s *MCPServer) errorResult(msg string) mcpToolResult {
	return mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func (s *MCPServer) toolCollections() mcpToolResult {
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
	return s.textResult(result)
}

func (s *MCPServer) toolStats() mcpToolResult {
	return s.textResult(s.db.Stats())
}

func (s *MCPServer) toolSearch(argsJSON json.RawMessage) mcpToolResult {
	var args struct {
		Collection string    `json:"collection"`
		Query      []float64 `json:"query"`
		TopK       int       `json:"top_k"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return s.errorResult("Invalid arguments: " + err.Error())
	}

	coll, err := s.db.GetCollection(args.Collection)
	if err != nil {
		return s.errorResult("Collection not found: " + args.Collection)
	}

	query := make([]float32, len(args.Query))
	for i, v := range args.Query {
		query[i] = float32(v)
	}

	opts := []veclite.SearchOption{}
	if args.TopK > 0 {
		opts = append(opts, veclite.TopK(args.TopK))
	}

	results, err := coll.Search(query, opts...)
	if err != nil {
		return s.errorResult("Search error: " + err.Error())
	}

	return s.textResult(formatResults(results))
}

func (s *MCPServer) toolTextSearch(argsJSON json.RawMessage) mcpToolResult {
	var args struct {
		Collection string `json:"collection"`
		Query      string `json:"query"`
		TopK       int    `json:"top_k"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return s.errorResult("Invalid arguments: " + err.Error())
	}

	coll, err := s.db.GetCollection(args.Collection)
	if err != nil {
		return s.errorResult("Collection not found: " + args.Collection)
	}

	opts := []veclite.SearchOption{}
	if args.TopK > 0 {
		opts = append(opts, veclite.TopK(args.TopK))
	}

	results, err := coll.TextSearch(args.Query, opts...)
	if err != nil {
		return s.errorResult("Text search error: " + err.Error())
	}

	return s.textResult(formatResults(results))
}

func (s *MCPServer) toolHybridSearch(argsJSON json.RawMessage) mcpToolResult {
	var args struct {
		Collection   string    `json:"collection"`
		Vector       []float64 `json:"vector"`
		Text         string    `json:"text"`
		TopK         int       `json:"top_k"`
		VectorWeight float64   `json:"vector_weight"`
		TextWeight   float64   `json:"text_weight"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return s.errorResult("Invalid arguments: " + err.Error())
	}

	coll, err := s.db.GetCollection(args.Collection)
	if err != nil {
		return s.errorResult("Collection not found: " + args.Collection)
	}

	query := make([]float32, len(args.Vector))
	for i, v := range args.Vector {
		query[i] = float32(v)
	}

	opts := []veclite.SearchOption{}
	if args.TopK > 0 {
		opts = append(opts, veclite.TopK(args.TopK))
	}
	if args.VectorWeight > 0 {
		opts = append(opts, veclite.WithVectorWeight(args.VectorWeight))
	}
	if args.TextWeight > 0 {
		opts = append(opts, veclite.WithTextWeight(args.TextWeight))
	}

	results, err := coll.HybridSearch(query, args.Text, opts...)
	if err != nil {
		return s.errorResult("Hybrid search error: " + err.Error())
	}

	return s.textResult(formatResults(results))
}

func (s *MCPServer) toolFind(argsJSON json.RawMessage) mcpToolResult {
	var args struct {
		Collection string          `json:"collection"`
		Filters    []filterRequest `json:"filters"`
		Limit      int             `json:"limit"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return s.errorResult("Invalid arguments: " + err.Error())
	}

	coll, err := s.db.GetCollection(args.Collection)
	if err != nil {
		return s.errorResult("Collection not found: " + args.Collection)
	}

	var filters []veclite.Filter
	for _, f := range args.Filters {
		filter := parseFilterRequest(f)
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	records, err := coll.Find(filters...)
	if err != nil {
		return s.errorResult("Find error: " + err.Error())
	}

	if args.Limit > 0 && len(records) > args.Limit {
		records = records[:args.Limit]
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

	return s.textResult(out)
}

func (s *MCPServer) toolInsert(argsJSON json.RawMessage) mcpToolResult {
	var args struct {
		Collection string         `json:"collection"`
		Vector     []float64      `json:"vector"`
		Payload    map[string]any `json:"payload"`
		Content    string         `json:"content"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return s.errorResult("Invalid arguments: " + err.Error())
	}

	coll := s.db.Collection(args.Collection)

	vector := make([]float32, len(args.Vector))
	for i, v := range args.Vector {
		vector[i] = float32(v)
	}

	var id uint64
	var err error
	if args.Content != "" {
		id, err = coll.InsertDocument(vector, args.Content, args.Payload)
	} else {
		id, err = coll.Insert(vector, args.Payload)
	}
	if err != nil {
		return s.errorResult("Insert error: " + err.Error())
	}

	_ = s.db.Sync()

	return s.textResult(map[string]any{"id": id, "status": "inserted"})
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
