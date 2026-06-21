package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/veclite"
	"github.com/abdul-hamid-achik/veclite/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer handles MCP protocol using the official Go SDK.
type MCPServer struct {
	db       *veclite.DB
	server   *mcp.Server
	embedder veclite.Embedder
	// graphStore caches created KnowledgeGraph instances by name
	graphStore map[string]*veclite.KnowledgeGraph
	// episodeStore caches EpisodeStore instances by collection name
	episodeStore map[string]*veclite.EpisodeStore
}

func cmdMCP(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: veclite mcp <db-path>")
	}

	dbPath := args[0]
	db, err := veclite.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	srv := &MCPServer{
		db:           db,
		graphStore:   make(map[string]*veclite.KnowledgeGraph),
		episodeStore: make(map[string]*veclite.EpisodeStore),
	}

	// Try to load embedder if available
	srv.initEmbedder()

	return srv.run()
}

// initEmbedder tries to initialize an embedder from config or environment variables.
// It checks VECLITE_EMBEDDER env var for quick setup, or loads from veclite.yaml.
func (s *MCPServer) initEmbedder() {
	// Check for environment variable quick setup
	provider := os.Getenv("VECLITE_EMBEDDER")
	if provider == "" {
		// Try loading from config file
		cfg, err := config.LoadConfig("")
		if err == nil && cfg.Embedder.Provider != "" {
			provider = cfg.Embedder.Provider
		}
	}

	if provider == "" {
		return
	}

	var embedderCfg veclite.EmbedderConfig
	switch provider {
	case "openai":
		apiKey := os.Getenv("VECLITE_OPENAI_API_KEY")
		baseURL := os.Getenv("VECLITE_OPENAI_BASE_URL")
		model := os.Getenv("VECLITE_OPENAI_MODEL")
		if apiKey == "" {
			// Try loading from config file
			cfg, err := config.LoadConfig("")
			if err == nil {
				apiKey = cfg.Embedder.OpenAI.APIKey
				if baseURL == "" {
					baseURL = cfg.Embedder.OpenAI.BaseURL
				}
				if model == "" {
					model = cfg.Embedder.OpenAI.Model
				}
			}
		}
		embedderCfg = veclite.EmbedderConfig{
			Provider: "openai",
			OpenAI: veclite.OpenAIConfig{
				APIKey:  apiKey,
				BaseURL: baseURL,
				Model:   model,
			},
		}
	case "ollama":
		baseURL := os.Getenv("VECLITE_OLLAMA_BASE_URL")
		model := os.Getenv("VECLITE_OLLAMA_MODEL")
		if baseURL == "" || model == "" {
			cfg, err := config.LoadConfig("")
			if err == nil {
				if baseURL == "" {
					baseURL = cfg.Embedder.Ollama.BaseURL
				}
				if model == "" {
					model = cfg.Embedder.Ollama.Model
				}
			}
		}
		embedderCfg = veclite.EmbedderConfig{
			Provider: "ollama",
			Ollama: veclite.OllamaConfig{
				BaseURL: baseURL,
				Model:   model,
			},
		}
	case "onnx":
		modelDir := os.Getenv("VECLITE_MODEL_DIR")
		model := os.Getenv("VECLITE_ONNX_MODEL")
		cfg, err := config.LoadConfig("")
		if err == nil {
			if modelDir == "" {
				modelDir = cfg.Embedder.ONNX.ModelDir
			}
			if model == "" {
				model = cfg.Embedder.ONNX.Model
			}
		}
		embedderCfg = veclite.EmbedderConfig{
			Provider: "onnx",
			ONNX: veclite.ONNXConfig{
				ModelDir: modelDir,
				Model:    model,
			},
		}
	default:
		return
	}

	embedder, err := veclite.NewEmbedderFromConfig(embedderCfg)
	if err != nil {
		// Log the error but don't fail - MCP server can still work without embedding
		fmt.Fprintf(os.Stderr, "veclite: failed to initialize embedder: %v\n", err)
		return
	}
	s.embedder = embedder
}

// getOrCreateGraph returns or creates a knowledge graph by name.
func (s *MCPServer) getOrCreateGraph(name string) (*veclite.KnowledgeGraph, error) {
	if kg, ok := s.graphStore[name]; ok {
		return kg, nil
	}
	kg, err := s.db.CreateKnowledgeGraph(name)
	if err != nil {
		return nil, err
	}
	s.graphStore[name] = kg
	return kg, nil
}

// getOrCreateEpisodeStore returns or creates an episode store for a collection.
func (s *MCPServer) getOrCreateEpisodeStore(collectionName string) (*veclite.EpisodeStore, error) {
	if es, ok := s.episodeStore[collectionName]; ok {
		return es, nil
	}
	es, err := s.db.CreateEpisodeStore(collectionName)
	if err != nil {
		return nil, err
	}
	s.episodeStore[collectionName] = es
	return es, nil
}

func (s *MCPServer) run() error {
	// Create MCP server with implementation info
	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "veclite",
		Version: veclite.Version,
	}, nil)

	// Register tools
	s.registerTools()

	// Run with stdio transport and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		cancel()
	}()

	transport := &mcp.StdioTransport{}
	if err := s.server.Run(ctx, transport); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}
	return nil
}

// Input/output types for tools

type emptyInput struct{}

type collectionSchemaInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
}

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
	Vector     []float64      `json:"vector,omitempty" jsonschema:"description=Vector embedding of the text. Optional if embedder is configured."`
	Importance float64        `json:"importance,omitempty" jsonschema:"description=Importance score from 0.0 to 1.0 (default 0.5)"`
	Tags       []string       `json:"tags,omitempty" jsonschema:"description=Optional tags for categorization"`
	TTLHours   float64        `json:"ttl_hours,omitempty" jsonschema:"description=Optional time-to-live in hours (0 = never expires)"`
	Metadata   map[string]any `json:"metadata,omitempty" jsonschema:"description=Additional metadata to store"`
}

type memoryRecallInput struct {
	Query          []float64 `json:"query,omitempty" jsonschema:"description=Query vector for semantic search. Optional if text and embedder are configured."`
	Text           string    `json:"text,omitempty" jsonschema:"description=Text to search for. Will be auto-embedded if embedder is configured."`
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

type embedInput struct {
	Text  string   `json:"text,omitempty" jsonschema:"description=Text to embed"`
	Texts []string `json:"texts,omitempty" jsonschema:"description=Multiple texts to embed in batch"`
}

// Graph tool inputs

type graphAddEntityInput struct {
	Graph      string         `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	ID         string         `json:"id" jsonschema:"description=Unique entity ID,required"`
	Type       string         `json:"type,omitempty" jsonschema:"description=Entity type (e.g. person/company/concept)"`
	Name       string         `json:"name,omitempty" jsonschema:"description=Human-readable name"`
	Vector     []float64      `json:"vector,omitempty" jsonschema:"description=Optional embedding vector"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"description=Additional properties"`
}

type graphAddRelationshipInput struct {
	Graph         string         `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	ID            string         `json:"id" jsonschema:"description=Unique relationship ID,required"`
	SourceID      string         `json:"source_id" jsonschema:"description=Source entity ID,required"`
	TargetID      string         `json:"target_id" jsonschema:"description=Target entity ID,required"`
	Type          string         `json:"type,omitempty" jsonschema:"description=Relationship type (e.g. works_at/knows)"`
	Weight        float64        `json:"weight,omitempty" jsonschema:"description=Relationship strength (0.0-1.0)"`
	Bidirectional bool           `json:"bidirectional,omitempty" jsonschema:"description=Whether relationship goes both ways"`
	Properties    map[string]any `json:"properties,omitempty" jsonschema:"description=Additional properties"`
}

type graphGetRelationshipsInput struct {
	Graph     string `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	EntityID  string `json:"entity_id" jsonschema:"description=Entity ID to get relationships for,required"`
	Direction string `json:"direction,omitempty" jsonschema:"description=Direction: outgoing/incoming/both (default both)"`
}

type graphTraverseInput struct {
	Graph             string   `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	StartIDs          []string `json:"start_ids" jsonschema:"description=Starting entity IDs,required"`
	MaxDepth          int      `json:"max_depth,omitempty" jsonschema:"description=Maximum traversal depth (default 3)"`
	MaxNodes          int      `json:"max_nodes,omitempty" jsonschema:"description=Maximum nodes to visit (default 100)"`
	MinWeight         float64  `json:"min_weight,omitempty" jsonschema:"description=Minimum relationship weight"`
	RelationshipTypes []string `json:"relationship_types,omitempty" jsonschema:"description=Filter by relationship types"`
	EntityTypes       []string `json:"entity_types,omitempty" jsonschema:"description=Filter by entity types"`
	Direction         string   `json:"direction,omitempty" jsonschema:"description=Direction: outgoing/incoming/both (default both)"`
}

type graphExpandedSearchInput struct {
	Graph       string    `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	Query       []float64 `json:"query,omitempty" jsonschema:"description=Query vector"`
	Text        string    `json:"text,omitempty" jsonschema:"description=Text to search (auto-embedded if embedder configured)"`
	TopK        int       `json:"top_k,omitempty" jsonschema:"description=Number of results (default 10)"`
	ExpandDepth int       `json:"expand_depth,omitempty" jsonschema:"description=Graph expansion depth (default 1)"`
}

// Conversation tool inputs

type conversationAddTurnInput struct {
	Collection    string         `json:"collection" jsonschema:"description=Collection name,required"`
	SessionID     string         `json:"session_id" jsonschema:"description=Session/conversation ID,required"`
	Role          string         `json:"role,omitempty" jsonschema:"description=Speaker role (user/assistant/system)"`
	Content       string         `json:"content" jsonschema:"description=Turn content text,required"`
	TurnNumber    int            `json:"turn_number,omitempty" jsonschema:"description=Sequential turn number (auto-increment if 0)"`
	ParentChunkID uint64         `json:"parent_chunk_id,omitempty" jsonschema:"description=Parent chunk ID for threaded conversations"`
	Vector        []float64      `json:"vector,omitempty" jsonschema:"description=Optional embedding vector (auto-embedded if not provided)"`
	Importance    float64        `json:"importance,omitempty" jsonschema:"description=Importance score (0.0-1.0)"`
	TTLHours      float64        `json:"ttl_hours,omitempty" jsonschema:"description=Time-to-live in hours"`
	Metadata      map[string]any `json:"metadata,omitempty" jsonschema:"description=Additional metadata"`
}

type conversationGetSessionInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	SessionID  string `json:"session_id" jsonschema:"description=Session ID to retrieve,required"`
}

type conversationSearchSessionInput struct {
	Collection string    `json:"collection" jsonschema:"description=Collection name,required"`
	SessionID  string    `json:"session_id" jsonschema:"description=Session ID to search within,required"`
	Query      []float64 `json:"query,omitempty" jsonschema:"description=Query vector"`
	Text       string    `json:"text,omitempty" jsonschema:"description=Text to search (auto-embedded if embedder configured)"`
	Limit      int       `json:"limit,omitempty" jsonschema:"description=Maximum results (default 10)"`
}

type conversationListSessionsInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
}

type conversationGetThreadInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	ChunkID    uint64 `json:"chunk_id" jsonschema:"description=Chunk ID to get thread for,required"`
}

// Episode tool inputs

type episodeDetectInput struct {
	Collection          string  `json:"collection" jsonschema:"description=Collection name,required"`
	TimeGapMinutes      float64 `json:"time_gap_minutes,omitempty" jsonschema:"description=Max time gap between records in same episode (default 30)"`
	MinRecords          int     `json:"min_records,omitempty" jsonschema:"description=Minimum records to form episode (default 2)"`
	SimilarityThreshold float64 `json:"similarity_threshold,omitempty" jsonschema:"description=Minimum similarity for grouping (0.0-1.0)"`
}

type episodeCreateInput struct {
	Collection string   `json:"collection" jsonschema:"description=Collection name,required"`
	RecordIDs  []uint64 `json:"record_ids" jsonschema:"description=Record IDs to include,required"`
	Title      string   `json:"title,omitempty" jsonschema:"description=Episode title/summary"`
}

type episodeGetInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	EpisodeID  string `json:"episode_id" jsonschema:"description=Episode ID to retrieve,required"`
}

type episodeListInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
}

type episodeSearchInput struct {
	Collection string    `json:"collection" jsonschema:"description=Collection name,required"`
	Query      []float64 `json:"query,omitempty" jsonschema:"description=Query vector"`
	Text       string    `json:"text,omitempty" jsonschema:"description=Text to search (auto-embedded if embedder configured)"`
	Limit      int       `json:"limit,omitempty" jsonschema:"description=Maximum results (default 10)"`
}

type episodeSearchExpandedInput struct {
	Collection string    `json:"collection" jsonschema:"description=Collection name,required"`
	Query      []float64 `json:"query,omitempty" jsonschema:"description=Query vector"`
	Text       string    `json:"text,omitempty" jsonschema:"description=Text to search (auto-embedded if embedder configured)"`
	TopK       int       `json:"top_k,omitempty" jsonschema:"description=Number of results (default 10)"`
}

// Consolidation tool inputs

type memoryFindClustersInput struct {
	Collection          string  `json:"collection" jsonschema:"description=Collection name (default: memories),required"`
	SimilarityThreshold float64 `json:"similarity_threshold,omitempty" jsonschema:"description=Minimum similarity for grouping (default 0.85)"`
	MinSize             int     `json:"min_size,omitempty" jsonschema:"description=Minimum cluster size (default 2)"`
	MaxSize             int     `json:"max_size,omitempty" jsonschema:"description=Maximum cluster size (default 10)"`
}

type memoryArchiveInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name (default: memories)"`
	RecordID   uint64 `json:"record_id" jsonschema:"description=Record ID to archive,required"`
}

type memoryUnarchiveInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name (default: memories)"`
	RecordID   uint64 `json:"record_id" jsonschema:"description=Record ID to unarchive,required"`
}

type memoryGetArchivedInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name (default: memories)"`
}

// Phase 1: Essential CRUD input types

type getInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	ID         uint64 `json:"id" jsonschema:"description=Record ID,required"`
}

type deleteInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	ID         uint64 `json:"id" jsonschema:"description=Record ID,required"`
}

type updateInput struct {
	Collection string         `json:"collection" jsonschema:"description=Collection name,required"`
	ID         uint64         `json:"id" jsonschema:"description=Record ID,required"`
	Payload    map[string]any `json:"payload" jsonschema:"description=New payload to set,required"`
}

type upsertInput struct {
	Collection string         `json:"collection" jsonschema:"description=Collection name,required"`
	ID         uint64         `json:"id,omitempty" jsonschema:"description=Record ID (0 for auto-generated)"`
	Vector     []float64      `json:"vector" jsonschema:"description=Vector to insert/update,required"`
	Payload    map[string]any `json:"payload,omitempty" jsonschema:"description=Optional metadata"`
}

type deleteWhereInput struct {
	Collection string          `json:"collection" jsonschema:"description=Collection name,required"`
	Filters    []filterRequest `json:"filters" jsonschema:"description=Filter conditions,required"`
}

type clearInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	Confirm    bool   `json:"confirm" jsonschema:"description=Must be true to confirm destructive operation,required"`
}

// Phase 2: Batch + Collection Management input types

type insertBatchInput struct {
	Collection string           `json:"collection" jsonschema:"description=Collection name,required"`
	Vectors    [][]float64      `json:"vectors" jsonschema:"description=Vectors to insert,required"`
	Payloads   []map[string]any `json:"payloads,omitempty" jsonschema:"description=Payloads for each vector (optional)"`
}

type upsertByKeyInput struct {
	Collection string         `json:"collection" jsonschema:"description=Collection name,required"`
	KeyField   string         `json:"key_field" jsonschema:"description=Payload field to match on,required"`
	KeyValue   any            `json:"key_value" jsonschema:"description=Value to match,required"`
	Vector     []float64      `json:"vector" jsonschema:"description=Vector to insert/update,required"`
	Payload    map[string]any `json:"payload,omitempty" jsonschema:"description=Optional metadata"`
}

type createCollectionInput struct {
	Name         string `json:"name" jsonschema:"description=Collection name,required"`
	Dimension    int    `json:"dimension,omitempty" jsonschema:"description=Vector dimension (auto-detected if not set)"`
	DistanceType string `json:"distance_type,omitempty" jsonschema:"description=Distance metric: cosine/euclidean/dot (default cosine)"`
	IndexType    string `json:"index_type,omitempty" jsonschema:"description=Index type: hnsw/flat (default hnsw)"`
}

type dropCollectionInput struct {
	Name    string `json:"name" jsonschema:"description=Collection name to drop,required"`
	Confirm bool   `json:"confirm" jsonschema:"description=Must be true to confirm destructive operation,required"`
}

type syncInput struct{}

type metricsInput struct{}

// Phase 3: Graph CRUD input types

type graphGetEntityInput struct {
	Graph    string `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	EntityID string `json:"entity_id" jsonschema:"description=Entity ID to retrieve,required"`
}

type graphUpdateEntityInput struct {
	Graph      string         `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	ID         string         `json:"id" jsonschema:"description=Entity ID,required"`
	Type       string         `json:"type,omitempty" jsonschema:"description=Entity type"`
	Name       string         `json:"name,omitempty" jsonschema:"description=Human-readable name"`
	Vector     []float64      `json:"vector,omitempty" jsonschema:"description=Optional embedding vector"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"description=Additional properties"`
}

type graphDeleteEntityInput struct {
	Graph    string `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	EntityID string `json:"entity_id" jsonschema:"description=Entity ID to delete,required"`
}

type graphDeleteRelationshipInput struct {
	Graph          string `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	RelationshipID string `json:"relationship_id" jsonschema:"description=Relationship ID to delete,required"`
}

type graphListEntitiesInput struct {
	Graph      string `json:"graph" jsonschema:"description=Knowledge graph name,required"`
	EntityType string `json:"entity_type,omitempty" jsonschema:"description=Filter by entity type (empty for all)"`
}

// Phase 4: Cleanup, Consolidation, and Conversation input types

type cleanupExpiredInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
}

type countExpiredInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
}

type memoryEnforceLimitInput struct {
	Collection        string `json:"collection" jsonschema:"description=Collection name (default: memories)"`
	MaxRecords        int    `json:"max_records" jsonschema:"description=Maximum number of records,required"`
	EvictionPolicy    string `json:"eviction_policy,omitempty" jsonschema:"description=Eviction policy: fifo/lru/importance (default fifo)"`
	EvictionBatchSize int    `json:"eviction_batch_size,omitempty" jsonschema:"description=Number of records to evict per batch"`
}

type memoryConsolidateInput struct {
	Collection          string  `json:"collection" jsonschema:"description=Collection name (default: memories)"`
	SimilarityThreshold float64 `json:"similarity_threshold,omitempty" jsonschema:"description=Minimum similarity for grouping (default 0.85)"`
	MinSize             int     `json:"min_size,omitempty" jsonschema:"description=Minimum cluster size (default 2)"`
	MaxSize             int     `json:"max_size,omitempty" jsonschema:"description=Maximum cluster size (default 10)"`
	ArchiveOriginals    bool    `json:"archive_originals,omitempty" jsonschema:"description=Archive original records after consolidation"`
}

type memoryExpandConsolidationInput struct {
	Collection      string `json:"collection" jsonschema:"description=Collection name (default: memories)"`
	ConsolidationID uint64 `json:"consolidation_id" jsonschema:"description=ID of the consolidation record,required"`
}

type conversationDeleteSessionInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	SessionID  string `json:"session_id" jsonschema:"description=Session ID to delete,required"`
	Confirm    bool   `json:"confirm" jsonschema:"description=Must be true to confirm destructive operation,required"`
}

type conversationGetStatsInput struct {
	Collection string `json:"collection" jsonschema:"description=Collection name,required"`
	SessionID  string `json:"session_id" jsonschema:"description=Session ID to get stats for,required"`
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

	// veclite_collection_schema - Discover the schema of a collection
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_collection_schema",
		Description: "Discover the schema of a collection: payload fields, types, vector dimension, index type, and content availability",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input collectionSchemaInput) (*mcp.CallToolResult, any, error) {
		return s.toolCollectionSchema(input)
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

	// veclite_embed - Embed text using ONNX
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_embed",
		Description: "Convert text to vector embedding using the configured ONNX embedder",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input embedInput) (*mcp.CallToolResult, any, error) {
		return s.toolEmbed(input)
	})

	// Graph tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_add_entity",
		Description: "Add an entity node to a knowledge graph with optional embedding vector",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphAddEntityInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphAddEntity(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_add_relationship",
		Description: "Add a relationship edge between two entities in a knowledge graph",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphAddRelationshipInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphAddRelationship(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_get_relationships",
		Description: "Get all relationships for an entity in a knowledge graph",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphGetRelationshipsInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphGetRelationships(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_traverse",
		Description: "Perform BFS traversal of a knowledge graph from starting entities",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphTraverseInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphTraverse(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_expanded_search",
		Description: "Search knowledge graph with vector similarity and expand results with graph context",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphExpandedSearchInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphExpandedSearch(input)
	})

	// Conversation tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "conversation_add_turn",
		Description: "Add a conversation turn with session tracking and optional threading",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input conversationAddTurnInput) (*mcp.CallToolResult, any, error) {
		return s.toolConversationAddTurn(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "conversation_get_session",
		Description: "Get all turns in a conversation session ordered by turn number",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input conversationGetSessionInput) (*mcp.CallToolResult, any, error) {
		return s.toolConversationGetSession(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "conversation_search_session",
		Description: "Search within a specific conversation session using vector similarity",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input conversationSearchSessionInput) (*mcp.CallToolResult, any, error) {
		return s.toolConversationSearchSession(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "conversation_list_sessions",
		Description: "List all conversation session IDs in a collection",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input conversationListSessionsInput) (*mcp.CallToolResult, any, error) {
		return s.toolConversationListSessions(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "conversation_get_thread",
		Description: "Get all records in a conversation thread starting from a chunk ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input conversationGetThreadInput) (*mcp.CallToolResult, any, error) {
		return s.toolConversationGetThread(input)
	})

	// Episode tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "episode_detect",
		Description: "Auto-detect episodes using temporal and similarity clustering",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input episodeDetectInput) (*mcp.CallToolResult, any, error) {
		return s.toolEpisodeDetect(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "episode_create",
		Description: "Manually create an episode from a set of record IDs",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input episodeCreateInput) (*mcp.CallToolResult, any, error) {
		return s.toolEpisodeCreate(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "episode_get",
		Description: "Get details of a specific episode including its records",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input episodeGetInput) (*mcp.CallToolResult, any, error) {
		return s.toolEpisodeGet(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "episode_list",
		Description: "List all detected episodes in a collection",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input episodeListInput) (*mcp.CallToolResult, any, error) {
		return s.toolEpisodeList(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "episode_search",
		Description: "Search episodes by their vector representation",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input episodeSearchInput) (*mcp.CallToolResult, any, error) {
		return s.toolEpisodeSearch(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "episode_search_expanded",
		Description: "Search with episode context expansion for richer results",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input episodeSearchExpandedInput) (*mcp.CallToolResult, any, error) {
		return s.toolEpisodeSearchExpanded(input)
	})

	// Consolidation tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_find_clusters",
		Description: "Find clusters of similar memories that could be consolidated",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryFindClustersInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryFindClusters(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_archive",
		Description: "Archive a memory record (excluded from searches but preserved)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryArchiveInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryArchive(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_unarchive",
		Description: "Restore an archived memory record to active status",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryUnarchiveInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryUnarchive(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_get_archived",
		Description: "List all archived memory records",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryGetArchivedInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryGetArchived(input)
	})

	// Phase 1: Essential CRUD tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_get",
		Description: "Get a record by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getInput) (*mcp.CallToolResult, any, error) {
		return s.toolGet(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_delete",
		Description: "Delete a record by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input deleteInput) (*mcp.CallToolResult, any, error) {
		return s.toolDelete(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_update",
		Description: "Update a record's payload by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input updateInput) (*mcp.CallToolResult, any, error) {
		return s.toolUpdate(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_upsert",
		Description: "Insert a new record or update an existing one by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input upsertInput) (*mcp.CallToolResult, any, error) {
		return s.toolUpsert(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_delete_where",
		Description: "Delete all records matching filter conditions",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input deleteWhereInput) (*mcp.CallToolResult, any, error) {
		return s.toolDeleteWhere(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_clear",
		Description: "Clear all records from a collection (requires confirm: true)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input clearInput) (*mcp.CallToolResult, any, error) {
		return s.toolClear(input)
	})

	// Phase 2: Batch + Collection Management tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_insert_batch",
		Description: "Insert multiple vectors in a single batch operation",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input insertBatchInput) (*mcp.CallToolResult, any, error) {
		return s.toolInsertBatch(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_upsert_by_key",
		Description: "Insert or update a record based on a key field in the payload",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input upsertByKeyInput) (*mcp.CallToolResult, any, error) {
		return s.toolUpsertByKey(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_create_collection",
		Description: "Create a new collection with specified options",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input createCollectionInput) (*mcp.CallToolResult, any, error) {
		return s.toolCreateCollection(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_drop_collection",
		Description: "Delete a collection and all its data (requires confirm: true)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input dropCollectionInput) (*mcp.CallToolResult, any, error) {
		return s.toolDropCollection(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_sync",
		Description: "Force persist all pending changes to disk",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input syncInput) (*mcp.CallToolResult, any, error) {
		return s.toolSync()
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_metrics",
		Description: "Get performance metrics for the database",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input metricsInput) (*mcp.CallToolResult, any, error) {
		return s.toolMetrics()
	})

	// Phase 3: Graph CRUD tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_get_entity",
		Description: "Get an entity from a knowledge graph by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphGetEntityInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphGetEntity(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_update_entity",
		Description: "Update an existing entity in a knowledge graph",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphUpdateEntityInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphUpdateEntity(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_delete_entity",
		Description: "Delete an entity and all its relationships from a knowledge graph",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphDeleteEntityInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphDeleteEntity(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_delete_relationship",
		Description: "Delete a relationship from a knowledge graph",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphDeleteRelationshipInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphDeleteRelationship(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "graph_list_entities",
		Description: "List all entities in a knowledge graph, optionally filtered by type",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input graphListEntitiesInput) (*mcp.CallToolResult, any, error) {
		return s.toolGraphListEntities(input)
	})

	// Phase 4: Cleanup, Consolidation, and Conversation tools

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_cleanup_expired",
		Description: "Remove all expired records from a collection",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input cleanupExpiredInput) (*mcp.CallToolResult, any, error) {
		return s.toolCleanupExpired(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "veclite_count_expired",
		Description: "Count the number of expired records in a collection",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input countExpiredInput) (*mcp.CallToolResult, any, error) {
		return s.toolCountExpired(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_enforce_limit",
		Description: "Enforce a memory limit by evicting records according to a policy",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryEnforceLimitInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryEnforceLimit(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_consolidate",
		Description: "Find and optionally consolidate similar memory clusters",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryConsolidateInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryConsolidate(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "memory_expand_consolidation",
		Description: "Get the original records that were merged into a consolidation record",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input memoryExpandConsolidationInput) (*mcp.CallToolResult, any, error) {
		return s.toolMemoryExpandConsolidation(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "conversation_delete_session",
		Description: "Delete all turns in a conversation session (requires confirm: true)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input conversationDeleteSessionInput) (*mcp.CallToolResult, any, error) {
		return s.toolConversationDeleteSession(input)
	})

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "conversation_get_stats",
		Description: "Get statistics about a conversation session",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input conversationGetStatsInput) (*mcp.CallToolResult, any, error) {
		return s.toolConversationGetStats(input)
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

func (s *MCPServer) toolCollectionSchema(input collectionSchemaInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	// Sample records to discover payload fields
	all := coll.All()
	sampleSize := len(all)
	if sampleSize > 50 {
		sampleSize = 50
	}

	fieldTypes := make(map[string]map[string]int)
	contentAvailable := 0
	hasVector := 0

	for i := 0; i < sampleSize; i++ {
		r := all[i]
		if r.Content != "" {
			contentAvailable++
		}
		if len(r.Vector) > 0 {
			hasVector++
		}
		for k, v := range r.Payload {
			if fieldTypes[k] == nil {
				fieldTypes[k] = make(map[string]int)
			}
			typeName := fmt.Sprintf("%T", v)
			fieldTypes[k][typeName]++
		}
	}

	// Build schema result
	type fieldInfo struct {
		Types []string `json:"types"`
		Count int      `json:"count"`
	}
	schema := map[string]any{
		"collection":        input.Collection,
		"dimension":         coll.Dimension(),
		"distance_type":     coll.DistanceType(),
		"index_type":        coll.IndexType(),
		"record_count":      len(all),
		"content_available": contentAvailable,
		"has_vector":        hasVector,
		"payload_fields":    make(map[string]fieldInfo),
	}

	fields := schema["payload_fields"].(map[string]fieldInfo)
	for k, typeCounts := range fieldTypes {
		types := make([]string, 0, len(typeCounts))
		for t := range typeCounts {
			types = append(types, t)
		}
		totalCount := 0
		for _, c := range typeCounts {
			totalCount += c
		}
		fields[k] = fieldInfo{Types: types, Count: totalCount}
	}

	return textResult(schema)
}

func (s *MCPServer) toolSearch(input searchInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
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
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
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
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
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
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
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

	var vec []float32
	if len(input.Vector) > 0 {
		vec = float64ToFloat32(input.Vector)
	} else if s.embedder != nil {
		var err error
		vec, err = s.embedder.Embed(input.Text)
		if err != nil {
			return errorResult("Auto-embedding failed: " + err.Error())
		}
	} else {
		return structuredError("NO_EMBEDDER", "Vector is required when embedder is not configured", "Provide a 'vector' field or configure an embedder")
	}

	coll := s.db.Collection(memoriesCollection)

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
	var queryVec []float32
	if len(input.Query) > 0 {
		queryVec = float64ToFloat32(input.Query)
	} else if input.Text != "" && s.embedder != nil {
		var err error
		queryVec, err = s.embedder.Embed(input.Text)
		if err != nil {
			return errorResult("Auto-embedding failed: " + err.Error())
		}
	} else if input.Text != "" {
		return errorResult("Query vector is required when embedder is not configured. Provide 'query' vector or configure ONNX embedder.")
	} else {
		return errorResult("Either 'query' vector or 'text' is required")
	}

	coll, err := s.db.GetCollection(memoriesCollection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Memories collection not found", "Use memory_remember to create it first")
	}

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
		return structuredError("COLLECTION_NOT_FOUND", "Memories collection not found", "Use memory_remember to create it first")
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

// Embed tool implementation

func (s *MCPServer) toolEmbed(input embedInput) (*mcp.CallToolResult, any, error) {
	if s.embedder == nil {
		return structuredError("NO_EMBEDDER", "No embedder configured", "Set VECLITE_MODEL_DIR environment variable or configure an ONNX embedder")
	}

	if input.Text != "" {
		vec, err := s.embedder.Embed(input.Text)
		if err != nil {
			return errorResult("Embedding failed: " + err.Error())
		}
		return textResult(map[string]any{
			"vector":    vec,
			"dimension": len(vec),
		})
	}

	if len(input.Texts) > 0 {
		vecs, err := s.embedder.EmbedBatch(input.Texts)
		if err != nil {
			return errorResult("Batch embedding failed: " + err.Error())
		}
		return textResult(map[string]any{
			"vectors":   vecs,
			"count":     len(vecs),
			"dimension": s.embedder.Dimension(),
		})
	}

	return errorResult("Either 'text' or 'texts' is required")
}

// Graph tool implementations

func (s *MCPServer) toolGraphAddEntity(input graphAddEntityInput) (*mcp.CallToolResult, any, error) {
	kg, err := s.getOrCreateGraph(input.Graph)
	if err != nil {
		return errorResult("Failed to create/get graph: " + err.Error())
	}

	entity := veclite.Entity{
		ID:         input.ID,
		Type:       input.Type,
		Name:       input.Name,
		Properties: input.Properties,
	}

	if len(input.Vector) > 0 {
		entity.Vector = float64ToFloat32(input.Vector)
	}

	if err := kg.AddEntity(entity); err != nil {
		return errorResult("Failed to add entity: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":    "added",
		"entity_id": input.ID,
		"graph":     input.Graph,
	})
}

func (s *MCPServer) toolGraphAddRelationship(input graphAddRelationshipInput) (*mcp.CallToolResult, any, error) {
	kg, err := s.getOrCreateGraph(input.Graph)
	if err != nil {
		return errorResult("Failed to create/get graph: " + err.Error())
	}

	rel := veclite.Relationship{
		ID:            input.ID,
		SourceID:      input.SourceID,
		TargetID:      input.TargetID,
		Type:          input.Type,
		Weight:        float32(input.Weight),
		Bidirectional: input.Bidirectional,
		Properties:    input.Properties,
	}

	if err := kg.AddRelationship(rel); err != nil {
		return errorResult("Failed to add relationship: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":          "added",
		"relationship_id": input.ID,
		"graph":           input.Graph,
	})
}

func (s *MCPServer) toolGraphGetRelationships(input graphGetRelationshipsInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	direction := input.Direction
	if direction == "" {
		direction = "both"
	}

	rels := kg.GetRelationships(input.EntityID, direction)

	relOut := make([]map[string]any, len(rels))
	for i, r := range rels {
		relOut[i] = map[string]any{
			"id":            r.ID,
			"source_id":     r.SourceID,
			"target_id":     r.TargetID,
			"type":          r.Type,
			"weight":        r.Weight,
			"bidirectional": r.Bidirectional,
			"properties":    r.Properties,
		}
	}

	return textResult(map[string]any{
		"entity_id":     input.EntityID,
		"direction":     direction,
		"relationships": relOut,
		"count":         len(relOut),
	})
}

func (s *MCPServer) toolGraphTraverse(input graphTraverseInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	config := veclite.TraversalConfig{
		MaxDepth:          input.MaxDepth,
		MaxNodes:          input.MaxNodes,
		MinWeight:         float32(input.MinWeight),
		RelationshipTypes: input.RelationshipTypes,
		EntityTypes:       input.EntityTypes,
		Direction:         input.Direction,
	}

	result, err := kg.Traverse(input.StartIDs, config)
	if err != nil {
		return errorResult("Traversal failed: " + err.Error())
	}

	entities := make([]map[string]any, len(result.Entities))
	for i, e := range result.Entities {
		entities[i] = map[string]any{
			"id":         e.ID,
			"type":       e.Type,
			"name":       e.Name,
			"depth":      result.Depths[e.ID],
			"properties": e.Properties,
		}
	}

	rels := make([]map[string]any, len(result.Relationships))
	for i, r := range result.Relationships {
		rels[i] = map[string]any{
			"id":        r.ID,
			"source_id": r.SourceID,
			"target_id": r.TargetID,
			"type":      r.Type,
			"weight":    r.Weight,
		}
	}

	return textResult(map[string]any{
		"entities":      entities,
		"relationships": rels,
		"entity_count":  len(entities),
		"rel_count":     len(rels),
	})
}

func (s *MCPServer) toolGraphExpandedSearch(input graphExpandedSearchInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	var queryVec []float32
	if len(input.Query) > 0 {
		queryVec = float64ToFloat32(input.Query)
	} else if input.Text != "" && s.embedder != nil {
		var err error
		queryVec, err = s.embedder.Embed(input.Text)
		if err != nil {
			return errorResult("Auto-embedding failed: " + err.Error())
		}
	} else if input.Text != "" {
		return errorResult("Query vector is required when embedder is not configured")
	} else {
		return errorResult("Either 'query' vector or 'text' is required")
	}

	topK := 10
	if input.TopK > 0 {
		topK = input.TopK
	}

	expandDepth := 1
	if input.ExpandDepth > 0 {
		expandDepth = input.ExpandDepth
	}

	results, err := kg.SearchWithExpansion(queryVec, veclite.TraversalConfig{
		MaxDepth: expandDepth,
	}, veclite.TopK(topK))
	if err != nil {
		return errorResult("Search failed: " + err.Error())
	}

	out := make([]map[string]any, len(results))
	for i, r := range results {
		related := make([]map[string]any, len(r.RelatedEntities))
		for j, re := range r.RelatedEntities {
			related[j] = map[string]any{
				"id":   re.ID,
				"type": re.Type,
				"name": re.Name,
			}
		}

		rels := make([]map[string]any, len(r.Relationships))
		for j, rel := range r.Relationships {
			rels[j] = map[string]any{
				"id":        rel.ID,
				"source_id": rel.SourceID,
				"target_id": rel.TargetID,
				"type":      rel.Type,
			}
		}

		out[i] = map[string]any{
			"entity": map[string]any{
				"id":         r.Entity.ID,
				"type":       r.Entity.Type,
				"name":       r.Entity.Name,
				"properties": r.Entity.Properties,
			},
			"score":            r.Score,
			"related_entities": related,
			"relationships":    rels,
		}
	}

	return textResult(map[string]any{
		"results": out,
		"count":   len(out),
	})
}

// Conversation tool implementations

func (s *MCPServer) toolConversationAddTurn(input conversationAddTurnInput) (*mcp.CallToolResult, any, error) {
	coll := s.db.Collection(input.Collection)

	var vec []float32
	if len(input.Vector) > 0 {
		vec = float64ToFloat32(input.Vector)
	} else if s.embedder != nil && input.Content != "" {
		var err error
		vec, err = s.embedder.Embed(input.Content)
		if err != nil {
			return errorResult("Auto-embedding failed: " + err.Error())
		}
	}

	turn := veclite.ConversationTurn{
		SessionID:     input.SessionID,
		TurnNumber:    input.TurnNumber,
		Role:          input.Role,
		Content:       input.Content,
		Vector:        vec,
		ParentChunkID: input.ParentChunkID,
		Payload:       input.Metadata,
		Importance:    float32(input.Importance),
	}

	if input.TTLHours > 0 {
		turn.TTL = time.Duration(input.TTLHours * float64(time.Hour))
	}

	id, err := coll.InsertTurn(turn)
	if err != nil {
		return errorResult("Failed to add turn: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":     "added",
		"id":         id,
		"session_id": input.SessionID,
	})
}

func (s *MCPServer) toolConversationGetSession(input conversationGetSessionInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	records, err := coll.GetSession(input.SessionID)
	if err != nil {
		return errorResult("Failed to get session: " + err.Error())
	}

	turns := make([]map[string]any, len(records))
	for i, r := range records {
		turn := map[string]any{
			"id":         r.ID,
			"content":    r.Content,
			"created_at": r.CreatedAt.Format(time.RFC3339),
		}
		if r.Payload != nil {
			if role, ok := r.Payload[veclite.PayloadKeyRole]; ok {
				turn["role"] = role
			}
			if tn, ok := r.Payload[veclite.PayloadKeyTurnNumber]; ok {
				turn["turn_number"] = tn
			}
		}
		turns[i] = turn
	}

	return textResult(map[string]any{
		"session_id": input.SessionID,
		"turns":      turns,
		"count":      len(turns),
	})
}

func (s *MCPServer) toolConversationSearchSession(input conversationSearchSessionInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	var queryVec []float32
	if len(input.Query) > 0 {
		queryVec = float64ToFloat32(input.Query)
	} else if input.Text != "" && s.embedder != nil {
		queryVec, err = s.embedder.Embed(input.Text)
		if err != nil {
			return errorResult("Auto-embedding failed: " + err.Error())
		}
	} else if input.Text != "" {
		return errorResult("Query vector required when embedder is not configured")
	} else {
		return errorResult("Either 'query' vector or 'text' is required")
	}

	limit := 10
	if input.Limit > 0 {
		limit = input.Limit
	}

	results, err := coll.SearchInSession(input.SessionID, queryVec, veclite.TopK(limit))
	if err != nil {
		return errorResult("Search failed: " + err.Error())
	}

	return textResult(map[string]any{
		"session_id": input.SessionID,
		"results":    formatResults(results),
		"count":      len(results),
	})
}

func (s *MCPServer) toolConversationListSessions(input conversationListSessionsInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	sessions := coll.ListSessions()

	return textResult(map[string]any{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (s *MCPServer) toolConversationGetThread(input conversationGetThreadInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	records, err := coll.GetThread(input.ChunkID)
	if err != nil {
		return errorResult("Failed to get thread: " + err.Error())
	}

	turns := make([]map[string]any, len(records))
	for i, r := range records {
		turns[i] = map[string]any{
			"id":         r.ID,
			"content":    r.Content,
			"created_at": r.CreatedAt.Format(time.RFC3339),
			"payload":    r.Payload,
		}
	}

	return textResult(map[string]any{
		"chunk_id": input.ChunkID,
		"thread":   turns,
		"count":    len(turns),
	})
}

// Episode tool implementations

func (s *MCPServer) toolEpisodeDetect(input episodeDetectInput) (*mcp.CallToolResult, any, error) {
	es, err := s.getOrCreateEpisodeStore(input.Collection)
	if err != nil {
		return errorResult("Failed to create episode store: " + err.Error())
	}

	config := veclite.EpisodeConfig{
		MinRecords:          input.MinRecords,
		SimilarityThreshold: float32(input.SimilarityThreshold),
	}

	if input.TimeGapMinutes > 0 {
		config.TimeGapThreshold = time.Duration(input.TimeGapMinutes * float64(time.Minute))
	}

	episodes, err := es.DetectEpisodes(config)
	if err != nil {
		return errorResult("Episode detection failed: " + err.Error())
	}

	out := make([]map[string]any, len(episodes))
	for i, ep := range episodes {
		out[i] = map[string]any{
			"id":           ep.ID,
			"title":        ep.Title,
			"record_count": len(ep.RecordIDs),
			"record_ids":   ep.RecordIDs,
			"time_range": map[string]any{
				"start":    ep.TimeRange.Start.Format(time.RFC3339),
				"end":      ep.TimeRange.End.Format(time.RFC3339),
				"duration": ep.Duration().String(),
			},
			"created_at": ep.CreatedAt.Format(time.RFC3339),
		}
	}

	return textResult(map[string]any{
		"episodes": out,
		"count":    len(out),
	})
}

func (s *MCPServer) toolEpisodeCreate(input episodeCreateInput) (*mcp.CallToolResult, any, error) {
	es, err := s.getOrCreateEpisodeStore(input.Collection)
	if err != nil {
		return errorResult("Failed to create episode store: " + err.Error())
	}

	episode, err := es.CreateEpisode(input.RecordIDs, input.Title)
	if err != nil {
		return errorResult("Failed to create episode: " + err.Error())
	}

	return textResult(map[string]any{
		"status":       "created",
		"episode_id":   episode.ID,
		"title":        episode.Title,
		"record_count": len(episode.RecordIDs),
	})
}

func (s *MCPServer) toolEpisodeGet(input episodeGetInput) (*mcp.CallToolResult, any, error) {
	es, ok := s.episodeStore[input.Collection]
	if !ok {
		return errorResult("No episodes detected for collection: " + input.Collection)
	}

	episode, err := es.GetEpisode(input.EpisodeID)
	if err != nil {
		return errorResult("Episode not found: " + err.Error())
	}

	records, _ := es.ExpandEpisode(input.EpisodeID)
	recordsOut := make([]map[string]any, len(records))
	for i, r := range records {
		recordsOut[i] = map[string]any{
			"id":         r.ID,
			"content":    r.Content,
			"created_at": r.CreatedAt.Format(time.RFC3339),
		}
	}

	return textResult(map[string]any{
		"id":       episode.ID,
		"title":    episode.Title,
		"metadata": episode.Metadata,
		"time_range": map[string]any{
			"start":    episode.TimeRange.Start.Format(time.RFC3339),
			"end":      episode.TimeRange.End.Format(time.RFC3339),
			"duration": episode.Duration().String(),
		},
		"records":    recordsOut,
		"created_at": episode.CreatedAt.Format(time.RFC3339),
	})
}

func (s *MCPServer) toolEpisodeList(input episodeListInput) (*mcp.CallToolResult, any, error) {
	es, ok := s.episodeStore[input.Collection]
	if !ok {
		return textResult(map[string]any{
			"episodes": []any{},
			"count":    0,
		})
	}

	episodes := es.ListEpisodes()

	out := make([]map[string]any, len(episodes))
	for i, ep := range episodes {
		out[i] = map[string]any{
			"id":           ep.ID,
			"title":        ep.Title,
			"record_count": len(ep.RecordIDs),
			"duration":     ep.Duration().String(),
			"created_at":   ep.CreatedAt.Format(time.RFC3339),
		}
	}

	return textResult(map[string]any{
		"episodes": out,
		"count":    len(out),
	})
}

func (s *MCPServer) toolEpisodeSearch(input episodeSearchInput) (*mcp.CallToolResult, any, error) {
	es, ok := s.episodeStore[input.Collection]
	if !ok {
		return errorResult("No episodes detected for collection: " + input.Collection)
	}

	var queryVec []float32
	if len(input.Query) > 0 {
		queryVec = float64ToFloat32(input.Query)
	} else if input.Text != "" && s.embedder != nil {
		var err error
		queryVec, err = s.embedder.Embed(input.Text)
		if err != nil {
			return errorResult("Auto-embedding failed: " + err.Error())
		}
	} else if input.Text != "" {
		return errorResult("Query vector required when embedder is not configured")
	} else {
		return errorResult("Either 'query' vector or 'text' is required")
	}

	limit := 10
	if input.Limit > 0 {
		limit = input.Limit
	}

	episodes, err := es.SearchEpisodes(queryVec, limit)
	if err != nil {
		return errorResult("Episode search failed: " + err.Error())
	}

	out := make([]map[string]any, len(episodes))
	for i, ep := range episodes {
		out[i] = map[string]any{
			"id":           ep.ID,
			"title":        ep.Title,
			"record_count": len(ep.RecordIDs),
		}
	}

	return textResult(map[string]any{
		"episodes": out,
		"count":    len(out),
	})
}

func (s *MCPServer) toolEpisodeSearchExpanded(input episodeSearchExpandedInput) (*mcp.CallToolResult, any, error) {
	es, ok := s.episodeStore[input.Collection]
	if !ok {
		return errorResult("No episodes detected for collection: " + input.Collection)
	}

	var queryVec []float32
	if len(input.Query) > 0 {
		queryVec = float64ToFloat32(input.Query)
	} else if input.Text != "" && s.embedder != nil {
		var err error
		queryVec, err = s.embedder.Embed(input.Text)
		if err != nil {
			return errorResult("Auto-embedding failed: " + err.Error())
		}
	} else if input.Text != "" {
		return errorResult("Query vector required when embedder is not configured")
	} else {
		return errorResult("Either 'query' vector or 'text' is required")
	}

	topK := 10
	if input.TopK > 0 {
		topK = input.TopK
	}

	results, err := es.SearchWithEpisodeExpansion(queryVec, veclite.TopK(topK))
	if err != nil {
		return errorResult("Search failed: " + err.Error())
	}

	out := make([]map[string]any, len(results))
	for i, r := range results {
		result := map[string]any{
			"id":      r.Result.Record.ID,
			"score":   r.Result.Score,
			"content": r.Result.Record.Content,
		}

		if r.Episode != nil {
			result["episode"] = map[string]any{
				"id":           r.Episode.ID,
				"title":        r.Episode.Title,
				"record_count": len(r.Episode.RecordIDs),
			}

			contextRecords := make([]map[string]any, len(r.EpisodeRecords))
			for j, cr := range r.EpisodeRecords {
				contextRecords[j] = map[string]any{
					"id":      cr.ID,
					"content": cr.Content,
				}
			}
			result["episode_context"] = contextRecords
		}

		out[i] = result
	}

	return textResult(map[string]any{
		"results": out,
		"count":   len(out),
	})
}

// Consolidation tool implementations

func (s *MCPServer) toolMemoryFindClusters(input memoryFindClustersInput) (*mcp.CallToolResult, any, error) {
	collName := input.Collection
	if collName == "" {
		collName = memoriesCollection
	}

	coll, err := s.db.GetCollection(collName)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+collName+"' not found", "Use veclite_create_collection to create it first")
	}

	config := veclite.ConsolidationConfig{
		SimilarityThreshold: float32(input.SimilarityThreshold),
		MinGroupSize:        input.MinSize,
		MaxGroupSize:        input.MaxSize,
	}

	clusters, err := coll.FindSimilarClusters(config)
	if err != nil {
		return errorResult("Cluster detection failed: " + err.Error())
	}

	out := make([]map[string]any, len(clusters))
	for i, c := range clusters {
		recordIDs := make([]uint64, len(c.Records))
		for j, r := range c.Records {
			recordIDs[j] = r.ID
		}

		out[i] = map[string]any{
			"id":                 c.ID,
			"record_count":       len(c.Records),
			"record_ids":         recordIDs,
			"average_importance": c.AverageImportance,
			"time_range": map[string]any{
				"start":    c.TimeRange.Start.Format(time.RFC3339),
				"end":      c.TimeRange.End.Format(time.RFC3339),
				"duration": c.TimeRange.Duration().String(),
			},
		}
	}

	return textResult(map[string]any{
		"clusters": out,
		"count":    len(out),
	})
}

func (s *MCPServer) toolMemoryArchive(input memoryArchiveInput) (*mcp.CallToolResult, any, error) {
	collName := input.Collection
	if collName == "" {
		collName = memoriesCollection
	}

	coll, err := s.db.GetCollection(collName)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+collName+"' not found", "Use veclite_create_collection to create it first")
	}

	if err := coll.ArchiveRecord(input.RecordID); err != nil {
		return errorResult("Archive failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":    "archived",
		"record_id": input.RecordID,
	})
}

func (s *MCPServer) toolMemoryUnarchive(input memoryUnarchiveInput) (*mcp.CallToolResult, any, error) {
	collName := input.Collection
	if collName == "" {
		collName = memoriesCollection
	}

	coll, err := s.db.GetCollection(collName)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+collName+"' not found", "Use veclite_create_collection to create it first")
	}

	if err := coll.UnarchiveRecord(input.RecordID); err != nil {
		return errorResult("Unarchive failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":    "unarchived",
		"record_id": input.RecordID,
	})
}

func (s *MCPServer) toolMemoryGetArchived(input memoryGetArchivedInput) (*mcp.CallToolResult, any, error) {
	collName := input.Collection
	if collName == "" {
		collName = memoriesCollection
	}

	coll, err := s.db.GetCollection(collName)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+collName+"' not found", "Use veclite_create_collection to create it first")
	}

	records, err := coll.GetArchived()
	if err != nil {
		return errorResult("Failed to get archived: " + err.Error())
	}

	out := make([]map[string]any, len(records))
	for i, r := range records {
		out[i] = map[string]any{
			"id":         r.ID,
			"content":    r.Content,
			"importance": r.Importance,
			"created_at": r.CreatedAt.Format(time.RFC3339),
			"payload":    r.Payload,
		}
	}

	return textResult(map[string]any{
		"archived": out,
		"count":    len(out),
	})
}

// Phase 1: Essential CRUD tool implementations

func (s *MCPServer) toolGet(input getInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	record, err := coll.Get(input.ID)
	if err != nil {
		return errorResult("Record not found: " + err.Error())
	}

	return textResult(map[string]any{
		"id":         record.ID,
		"payload":    record.Payload,
		"content":    record.Content,
		"importance": record.Importance,
		"created_at": record.CreatedAt.Format(time.RFC3339),
		"updated_at": record.UpdatedAt.Format(time.RFC3339),
	})
}

func (s *MCPServer) toolDelete(input deleteInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	if err := coll.Delete(input.ID); err != nil {
		return errorResult("Delete failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status": "deleted",
		"id":     input.ID,
	})
}

func (s *MCPServer) toolUpdate(input updateInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	if err := coll.Update(input.ID, input.Payload); err != nil {
		return errorResult("Update failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status": "updated",
		"id":     input.ID,
	})
}

func (s *MCPServer) toolUpsert(input upsertInput) (*mcp.CallToolResult, any, error) {
	coll := s.db.Collection(input.Collection)

	vec := float64ToFloat32(input.Vector)

	id, err := coll.Upsert(input.ID, vec, input.Payload)
	if err != nil {
		return errorResult("Upsert failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status": "upserted",
		"id":     id,
	})
}

func (s *MCPServer) toolDeleteWhere(input deleteWhereInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	var vecliteFilters []veclite.Filter
	for _, f := range input.Filters {
		filter := parseFilterRequest(f)
		if filter != nil {
			vecliteFilters = append(vecliteFilters, filter)
		}
	}

	if len(vecliteFilters) == 0 {
		return errorResult("At least one filter is required")
	}

	deleted, err := coll.DeleteWhere(vecliteFilters...)
	if err != nil {
		return errorResult("DeleteWhere failed: " + err.Error())
	}

	if deleted > 0 {
		_ = s.db.Sync()
	}

	return textResult(map[string]any{
		"status":  "deleted",
		"deleted": deleted,
	})
}

func (s *MCPServer) toolClear(input clearInput) (*mcp.CallToolResult, any, error) {
	if !input.Confirm {
		return errorResult("This operation requires confirm: true")
	}

	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	if err := coll.Clear(); err != nil {
		return errorResult("Clear failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":     "cleared",
		"collection": input.Collection,
	})
}

// Phase 2: Batch + Collection Management tool implementations

func (s *MCPServer) toolInsertBatch(input insertBatchInput) (*mcp.CallToolResult, any, error) {
	coll := s.db.Collection(input.Collection)

	// Convert vectors
	vectors := make([][]float32, len(input.Vectors))
	for i, v := range input.Vectors {
		vectors[i] = float64ToFloat32(v)
	}

	// Ensure payloads slice matches vectors if provided
	var payloads []map[string]any
	if len(input.Payloads) > 0 {
		payloads = input.Payloads
		// Pad with nil if fewer payloads than vectors
		for len(payloads) < len(vectors) {
			payloads = append(payloads, nil)
		}
	} else {
		payloads = make([]map[string]any, len(vectors))
	}

	ids, err := coll.InsertBatch(vectors, payloads)
	if err != nil {
		return errorResult("InsertBatch failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":   "inserted",
		"ids":      ids,
		"inserted": len(ids),
	})
}

func (s *MCPServer) toolUpsertByKey(input upsertByKeyInput) (*mcp.CallToolResult, any, error) {
	coll := s.db.Collection(input.Collection)

	vec := float64ToFloat32(input.Vector)

	id, inserted, err := coll.UpsertByKey(input.KeyField, input.KeyValue, vec, input.Payload)
	if err != nil {
		return errorResult("UpsertByKey failed: " + err.Error())
	}

	_ = s.db.Sync()

	action := "updated"
	if inserted {
		action = "inserted"
	}

	return textResult(map[string]any{
		"status":   action,
		"id":       id,
		"inserted": inserted,
	})
}

func (s *MCPServer) toolCreateCollection(input createCollectionInput) (*mcp.CallToolResult, any, error) {
	var opts []veclite.CollectionOption

	if input.Dimension > 0 {
		opts = append(opts, veclite.WithDimension(input.Dimension))
	}

	if input.DistanceType != "" {
		var distType veclite.DistanceType
		switch input.DistanceType {
		case "cosine":
			distType = veclite.DistanceCosine
		case "dot":
			distType = veclite.DistanceDot
		case "euclidean":
			distType = veclite.DistanceEuclidean
		default:
			return errorResult("Invalid distance_type: " + input.DistanceType + ". Must be cosine, dot, or euclidean")
		}
		opts = append(opts, veclite.WithDistanceType(distType))
	}

	if input.IndexType != "" {
		switch input.IndexType {
		case "hnsw":
			// Use default HNSW parameters: M=16, efConstruction=200
			opts = append(opts, veclite.WithHNSW(16, 200))
		case "flat":
			// No special option for flat - it's the default when no index is specified
		default:
			return errorResult("Invalid index_type: " + input.IndexType + ". Must be hnsw or flat")
		}
	}

	coll, err := s.db.CreateCollection(input.Name, opts...)
	if err != nil {
		return errorResult("CreateCollection failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":     "created",
		"collection": coll.Name(),
	})
}

func (s *MCPServer) toolDropCollection(input dropCollectionInput) (*mcp.CallToolResult, any, error) {
	if !input.Confirm {
		return errorResult("This operation requires confirm: true")
	}

	if err := s.db.DropCollection(input.Name); err != nil {
		return errorResult("DropCollection failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":     "dropped",
		"collection": input.Name,
	})
}

func (s *MCPServer) toolSync() (*mcp.CallToolResult, any, error) {
	if err := s.db.Sync(); err != nil {
		return errorResult("Sync failed: " + err.Error())
	}

	return textResult(map[string]any{
		"status": "synced",
	})
}

func (s *MCPServer) toolMetrics() (*mcp.CallToolResult, any, error) {
	metrics := s.db.Metrics()

	return textResult(map[string]any{
		"insert_count":    metrics.InsertCount,
		"delete_count":    metrics.DeleteCount,
		"search_count":    metrics.SearchCount,
		"avg_search_time": metrics.AvgSearchTime.String(),
	})
}

// Phase 3: Graph CRUD tool implementations

func (s *MCPServer) toolGraphGetEntity(input graphGetEntityInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	entity, err := kg.GetEntity(input.EntityID)
	if err != nil {
		return errorResult("Entity not found: " + err.Error())
	}

	return textResult(map[string]any{
		"id":         entity.ID,
		"type":       entity.Type,
		"name":       entity.Name,
		"properties": entity.Properties,
		"has_vector": len(entity.Vector) > 0,
	})
}

func (s *MCPServer) toolGraphUpdateEntity(input graphUpdateEntityInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	entity := veclite.Entity{
		ID:         input.ID,
		Type:       input.Type,
		Name:       input.Name,
		Properties: input.Properties,
	}

	if len(input.Vector) > 0 {
		entity.Vector = float64ToFloat32(input.Vector)
	}

	if err := kg.UpdateEntity(entity); err != nil {
		return errorResult("UpdateEntity failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":    "updated",
		"entity_id": input.ID,
		"graph":     input.Graph,
	})
}

func (s *MCPServer) toolGraphDeleteEntity(input graphDeleteEntityInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	if err := kg.DeleteEntity(input.EntityID); err != nil {
		return errorResult("DeleteEntity failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":    "deleted",
		"entity_id": input.EntityID,
		"graph":     input.Graph,
	})
}

func (s *MCPServer) toolGraphDeleteRelationship(input graphDeleteRelationshipInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	if err := kg.DeleteRelationship(input.RelationshipID); err != nil {
		return errorResult("DeleteRelationship failed: " + err.Error())
	}

	_ = s.db.Sync()

	return textResult(map[string]any{
		"status":          "deleted",
		"relationship_id": input.RelationshipID,
		"graph":           input.Graph,
	})
}

func (s *MCPServer) toolGraphListEntities(input graphListEntitiesInput) (*mcp.CallToolResult, any, error) {
	kg, ok := s.graphStore[input.Graph]
	if !ok {
		return structuredError("COLLECTION_NOT_FOUND", "Graph '"+input.Graph+"' not found", "Use veclite_graph_create to create it first")
	}

	entities := kg.ListEntities(input.EntityType)

	out := make([]map[string]any, len(entities))
	for i, e := range entities {
		out[i] = map[string]any{
			"id":         e.ID,
			"type":       e.Type,
			"name":       e.Name,
			"properties": e.Properties,
		}
	}

	return textResult(map[string]any{
		"entities": out,
		"count":    len(out),
		"graph":    input.Graph,
	})
}

// Phase 4: Cleanup, Consolidation, and Conversation tool implementations

func (s *MCPServer) toolCleanupExpired(input cleanupExpiredInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	deleted, err := coll.CleanupExpired()
	if err != nil {
		return errorResult("CleanupExpired failed: " + err.Error())
	}

	if deleted > 0 {
		_ = s.db.Sync()
	}

	return textResult(map[string]any{
		"status":  "cleaned",
		"deleted": deleted,
	})
}

func (s *MCPServer) toolCountExpired(input countExpiredInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	count := coll.CountExpired()

	return textResult(map[string]any{
		"expired": count,
	})
}

func (s *MCPServer) toolMemoryEnforceLimit(input memoryEnforceLimitInput) (*mcp.CallToolResult, any, error) {
	collName := input.Collection
	if collName == "" {
		collName = memoriesCollection
	}

	coll, err := s.db.GetCollection(collName)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+collName+"' not found", "Use veclite_create_collection to create it first")
	}

	config := veclite.MemoryConfig{
		MaxRecords:        input.MaxRecords,
		EvictionPolicy:    input.EvictionPolicy,
		EvictionBatchSize: input.EvictionBatchSize,
	}

	if config.EvictionPolicy == "" {
		config.EvictionPolicy = "fifo"
	}

	evicted := coll.EnforceMemoryLimit(config)

	if evicted > 0 {
		_ = s.db.Sync()
	}

	return textResult(map[string]any{
		"status":  "enforced",
		"evicted": evicted,
	})
}

func (s *MCPServer) toolMemoryConsolidate(input memoryConsolidateInput) (*mcp.CallToolResult, any, error) {
	collName := input.Collection
	if collName == "" {
		collName = memoriesCollection
	}

	coll, err := s.db.GetCollection(collName)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+collName+"' not found", "Use veclite_create_collection to create it first")
	}

	config := veclite.ConsolidationConfig{
		SimilarityThreshold: float32(input.SimilarityThreshold),
		MinGroupSize:        input.MinSize,
		MaxGroupSize:        input.MaxSize,
		ArchiveOriginals:    input.ArchiveOriginals,
	}

	// Note: Without embedder and summary generator, this just finds clusters
	// It won't create consolidated records
	clusters, err := coll.FindSimilarClusters(config)
	if err != nil {
		return errorResult("FindSimilarClusters failed: " + err.Error())
	}

	out := make([]map[string]any, len(clusters))
	for i, c := range clusters {
		recordIDs := make([]uint64, len(c.Records))
		for j, r := range c.Records {
			recordIDs[j] = r.ID
		}

		out[i] = map[string]any{
			"id":                 c.ID,
			"record_count":       len(c.Records),
			"record_ids":         recordIDs,
			"average_importance": c.AverageImportance,
		}
	}

	return textResult(map[string]any{
		"clusters_found": len(clusters),
		"clusters":       out,
	})
}

func (s *MCPServer) toolMemoryExpandConsolidation(input memoryExpandConsolidationInput) (*mcp.CallToolResult, any, error) {
	collName := input.Collection
	if collName == "" {
		collName = memoriesCollection
	}

	coll, err := s.db.GetCollection(collName)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+collName+"' not found", "Use veclite_create_collection to create it first")
	}

	records, err := coll.ExpandConsolidation(input.ConsolidationID)
	if err != nil {
		return errorResult("ExpandConsolidation failed: " + err.Error())
	}

	out := make([]map[string]any, len(records))
	for i, r := range records {
		out[i] = map[string]any{
			"id":         r.ID,
			"content":    r.Content,
			"importance": r.Importance,
			"created_at": r.CreatedAt.Format(time.RFC3339),
			"payload":    r.Payload,
		}
	}

	return textResult(map[string]any{
		"consolidation_id": input.ConsolidationID,
		"records":          out,
		"count":            len(out),
	})
}

func (s *MCPServer) toolConversationDeleteSession(input conversationDeleteSessionInput) (*mcp.CallToolResult, any, error) {
	if !input.Confirm {
		return errorResult("This operation requires confirm: true")
	}

	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	// Get all records in the session
	records, err := coll.GetSession(input.SessionID)
	if err != nil {
		return errorResult("GetSession failed: " + err.Error())
	}

	// Delete each record
	deleted := 0
	for _, r := range records {
		if err := coll.Delete(r.ID); err == nil {
			deleted++
		}
	}

	if deleted > 0 {
		_ = s.db.Sync()
	}

	return textResult(map[string]any{
		"status":     "deleted",
		"session_id": input.SessionID,
		"deleted":    deleted,
	})
}

func (s *MCPServer) toolConversationGetStats(input conversationGetStatsInput) (*mcp.CallToolResult, any, error) {
	coll, err := s.db.GetCollection(input.Collection)
	if err != nil {
		return structuredError("COLLECTION_NOT_FOUND", "Collection '"+input.Collection+"' not found", "Use veclite_create_collection to create it first")
	}

	stats, err := coll.GetSessionStats(input.SessionID)
	if err != nil {
		return errorResult("GetSessionStats failed: " + err.Error())
	}

	result := map[string]any{
		"session_id": stats.SessionID,
		"turn_count": stats.TurnCount,
		"roles":      stats.Roles,
	}

	if stats.TurnCount > 0 {
		result["first_turn"] = stats.FirstTurn.Format(time.RFC3339)
		result["last_turn"] = stats.LastTurn.Format(time.RFC3339)
		result["duration"] = stats.LastTurn.Sub(stats.FirstTurn).String()
	}

	return textResult(result)
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

type mcpError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

func structuredError(code, message, suggestion string) (*mcp.CallToolResult, any, error) {
	errData, _ := json.Marshal(mcpError{
		Code:       code,
		Message:    message,
		Suggestion: suggestion,
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		IsError: true,
		Meta: map[string]any{
			"error": json.RawMessage(errData),
		},
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
