# VecLite

Embeddable vector database for Go.

Store vectors with metadata in a single file. Search with cosine similarity, dot product, or Euclidean distance. Add HNSW for fast approximate nearest neighbors. Use BM25 for full-text search. Combine both with hybrid search.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Documentation Site](#documentation-site)
- [Use as a Go Library](#use-as-a-go-library)
- [Embedding Strategy](#embedding-strategy)
- [Project Status](#project-status)
- [Library API](#library-api)
  - [Opening a Database](#opening-a-database)
  - [Durability and the Write-Ahead Log](#durability-and-the-write-ahead-log)
  - [Collections](#collections)
  - [Database and Collection Metadata](#database-and-collection-metadata)
  - [Inserting Vectors](#inserting-vectors)
  - [Document Storage](#document-storage)
  - [Searching](#searching)
  - [Text Search (BM25)](#text-search-bm25)
  - [Hybrid Search](#hybrid-search)
  - [Streaming Results](#streaming-results)
  - [Filtering](#filtering)
  - [Pagination](#pagination)
  - [Iteration](#iteration)
  - [Upsert](#upsert)
  - [Updating Records](#updating-records)
  - [Deleting Records](#deleting-records)
  - [Auto-Embedding](#auto-embedding)
  - [Embedding Providers](#embedding-providers)
  - [OpenAI Embedder](#openai-embedder)
  - [Ollama Embedder](#ollama-embedder)
  - [Local ONNX Embedder](#local-onnx-embedder)
  - [Config-Based Embedding](#config-based-embedding)
  - [HNSW Configuration](#hnsw-configuration)
  - [Observability](#observability)
  - [Statistics](#statistics)
- [Agent Memory Features](#agent-memory-features)
  - [TTL and Expiration](#ttl-and-expiration)
  - [Importance and Decay Scoring](#importance-and-decay-scoring)
  - [Timestamp Filters](#timestamp-filters)
  - [Conversation Memory](#conversation-memory)
  - [Subscriptions (Real-time Notifications)](#subscriptions-real-time-notifications)
  - [Memory Consolidation](#memory-consolidation)
  - [Episodic Memory](#episodic-memory)
  - [Memory Pressure Handling](#memory-pressure-handling)
  - [Knowledge Graph](#knowledge-graph)
- [CLI Usage](#cli-usage)
- [HTTP Server](#http-server)
- [MCP Tool Server](#mcp-tool-server)
- [Examples](#examples)
- [Performance](#performance)
- [Thread Safety](#thread-safety)
- [Persistence](#persistence)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Embeddable Go library** -- Core vector storage and search are small and local-first; optional integrations add provider-specific modules
- **Single-file storage** -- Database persists to one `.veclite` file
- **Private by default** -- New database, lock, and storage-directory artifacts are owner-only on POSIX systems
- **HNSW indexing** -- Fast approximate nearest neighbor search
- **BM25 text search** -- Full-text search over record content and payload fields
- **Hybrid search** -- Combine vector and text search with Reciprocal Rank Fusion
- **Named vector spaces** -- Multiple independent embeddings per record (e.g. `text` + `image`) with per-space fusion
- **Embedding profiles** -- First-class `EmbeddingProfile` with dimension validation and compatibility checks
- **Document storage** -- Store original text content alongside vectors
- **Auto-embedding** -- Pluggable `Embedder` interface for text-to-vector conversion
- **Local ONNX embedder** -- Optional `all-MiniLM-L6-v2` embedder for local inference (build with `-tags onnx`)
- **Metadata filtering** -- Rich filter expressions (equality, range, glob, prefix, logical operators)
- **Streaming results** -- Process results via callback without materializing all at once
- **Pagination** -- Offset/limit on search results and record iteration
- **Observability** -- Structured logger interface and atomic metrics counters
- **Thread-safe** -- Safe for concurrent read/write access
- **In-memory mode** -- Use `:memory:` for testing
- **CLI and HTTP server** -- Manage databases from the command line or over REST
- **MCP server** -- Expose VecLite as tools for AI agents via Model Context Protocol

### Agent Memory Features

- **TTL and expiration** -- Records can expire automatically after a duration
- **Background TTL cleanup** -- Automatic periodic cleanup of expired records
- **Importance scoring** -- Assign importance (0.0-1.0) to records for prioritized retrieval
- **Temporal decay** -- Search scores decay based on record age (exponential, linear, gaussian)
- **Access tracking** -- Track when records are accessed and how often
- **Memory pressure handling** -- Automatic eviction with FIFO, LRU, or importance policies
- **Conversation memory** -- Session-based conversation turns with threading support
- **Real-time subscriptions** -- Get notified when new records match a query
- **Memory consolidation** -- Cluster and consolidate similar memories
- **Episodic memory** -- Group related memories into coherent episodes
- **Knowledge graph** -- Entity-relationship graph with traversal and vector search

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/abdul-hamid-achik/veclite"
)

func main() {
    db, _ := veclite.Open("vectors.veclite")
    defer db.Close()

    coll, _ := db.CreateCollection("embeddings",
        veclite.WithDimension(384),
        veclite.WithHNSW(16, 200),
    )

    coll.Insert([]float32{0.1, 0.2 /* ... */}, map[string]any{"file": "main.go"})

    results, _ := coll.Search(queryVector, veclite.TopK(10))
    for _, r := range results {
        fmt.Printf("ID: %d, Score: %.4f\n", r.Record.ID, r.Score)
    }
}
```

## Installation

Requires Go 1.25 or later. VecLite v0.23.1 raised the minimum from Go 1.23 because the official MCP Go SDK now requires Go 1.25. VecLite's core storage and search APIs are local-first; optional integrations such as MCP, YAML config, and ONNX embedding use external Go modules.

```bash
# Library
go get github.com/abdul-hamid-achik/veclite

# CLI
go install github.com/abdul-hamid-achik/veclite/cmd/veclite@latest
```

Pre-built binaries are available on the [Releases](https://github.com/abdul-hamid-achik/veclite/releases) page.

## Documentation Site

The documentation site uses VitePress and Bun. Build it locally with:

```bash
task site
```

Use `task site-dev` while editing documentation and `task site-preview` to preview the production build.

## Use as a Go Library

VecLite is primarily an importable Go library:

```go
import "github.com/abdul-hamid-achik/veclite"
```

Applications can bring their own embedding pipeline and use VecLite for durable local storage, HNSW vector search, BM25 text search, metadata filtering, and hybrid ranking. See [docs/embeddings.md](docs/embeddings.md) for the app/library boundary and embedding-profile guidance.

### Use from any language

VecLite is usable as a Go library **and** as a standalone engine driven from any language through
its CLI and HTTP server, both of which speak JSON. Language drivers (Python, TypeScript, …) are
**planned, not yet written** — but the CLI and HTTP JSON shapes are treated as a stable
cross-language contract and are pinned by the behavior specs under `specs/glyphrun/`. If you are
integrating from another language today, drive `veclite serve` over HTTP or shell out to the
`veclite` CLI with `--json`.

## Embedding Strategy

A collection has one default vector space plus optional **named vector spaces**, so one logical
record can hold several embeddings (e.g. `text`, `image`, `audio`), each with its own dimension and
index. Use one collection with named spaces for multimodal records; use separate collections only
when records are genuinely unrelated. See the [Named Vector Spaces](docs/guide/named-vector-spaces.md)
guide and [ADR-0001](docs/adr/0001-embedding-boundary-and-named-vector-spaces.md) for the design.

## Project Status

See [docs/project-status.md](docs/project-status.md) for the current implementation boundary, related-project usage notes, and missing work.

## Library API

### Opening a Database

```go
// File-based (persistent)
db, err := veclite.Open("vectors.veclite")

// In-memory (testing)
db, err := veclite.Open(":memory:")

// With options
db, err := veclite.Open("vectors.veclite",
    veclite.WithWAL(true),          // Write-ahead log: crash-safe writes
    veclite.WithSyncOnWrite(true),  // Sync after each write
    veclite.WithReadOnly(true),     // Read-only mode
    veclite.WithSharedRead(true),   // Shared lock for multi-process reads
    veclite.WithLogger(myLogger),   // Structured logging
)

defer db.Close()
```

| DB Option | Description |
|-----------|-------------|
| `WithWAL(bool)` | Write-ahead log. Each write appends the affected records to a `*.wal` sidecar with one fsync, so writes survive a crash between snapshot saves at a fraction of `WithSyncOnWrite`'s cost. See [Durability and the WAL](#durability-and-the-write-ahead-log). |
| `WithSyncOnWrite(bool)` | Full snapshot save after each write. Slowest, maximally conservative durability. |
| `WithReadOnly(bool)` | Open in read-only mode. Write operations return errors. |
| `WithSharedRead(bool)` | Open read-only **lock-free** (no flock). Requires `WithReadOnly(true)`. A long-lived reader never blocks a writer and is never blocked by one; readers see a point-in-time snapshot, so call `db.Reload()` to pick up concurrent writes. Consistency is guaranteed by the writer's atomic-replace save. |
| `WithLogger(Logger)` | Set structured logger. Default is `NopLogger` (zero overhead). |

### Durability and the Write-Ahead Log

VecLite persists the whole database as a single snapshot file on `Sync()` and
`Close()`. By default, writes made between those points live only in memory.
Three durability modes cover the spectrum:

| Mode | Cost per write | Crash loses |
|------|----------------|-------------|
| Default | none | everything since the last `Sync`/`Close` |
| `WithWAL(true)` | one small append + fsync | nothing (completed writes are replayed) |
| `WithSyncOnWrite(true)` | full snapshot rewrite + fsync | nothing |

```go
db, err := veclite.Open("vectors.veclite", veclite.WithWAL(true))
```

With the WAL enabled, every completed mutation (inserts, updates, deletes,
multi-space records, text documents, metadata, collection lifecycle,
knowledge-graph and episode-store changes) is
appended to `vectors.veclite.wal` before the call returns. On the next open —
after a crash or `kill -9` — the log is replayed on top of the last snapshot,
indexes (HNSW and BM25) are restored to match, and the recovered state is
folded into a fresh snapshot. A clean `Sync()` or `Close()` truncates the log,
so it stays small.

Notes:

- Opening a database **without** `WithWAL` still recovers and folds a log left
  behind by a crashed WAL-enabled writer; nothing is lost by mixing modes.
- Read-only opens (including `WithSharedRead`) replay the log in memory without
  touching it, and `Reload()` re-applies it — so shared readers can observe a
  live writer's not-yet-snapshotted writes.
- Entries are CRC-checked; a torn append from a crash mid-write is discarded.
- Read-path bookkeeping (access counts) is not logged; it persists on the
  next full save.

### Collections

```go
// Get or create with defaults (auto-detects dimension on first insert)
coll := db.Collection("embeddings")

// Create with explicit options
coll, err := db.CreateCollection("embeddings",
    veclite.WithDimension(384),
    veclite.WithDistanceType(veclite.DistanceCosine),
    veclite.WithHNSW(16, 200),
    veclite.WithTextIndex("title", "body"),
    veclite.WithEmbedder(myEmbedder),
)

// Get existing (returns error if not found)
coll, err := db.GetCollection("embeddings")

// List, check, drop
names := db.Collections()
exists := db.HasCollection("embeddings")
err := db.DropCollection("embeddings")
```

| Collection Option | Description |
|-------------------|-------------|
| `WithDimension(int)` | Fixed vector dimension. 0 (default) auto-detects on first insert. |
| `WithDistanceType(DistanceType)` | Distance metric. Default: `DistanceCosine`. |
| `WithHNSW(m, efConstruction)` | Enable HNSW index with given parameters. |
| `WithHNSWConfig(HNSWConfig)` | Enable HNSW with full configuration struct. |
| `WithTextIndex(fields...)` | Enable BM25 text indexing on named payload fields. `Content` is always indexed. |
| `WithEmbedder(Embedder)` | Set auto-embedding plugin for `InsertText`/`SearchText`. |

### Database and Collection Metadata

Store application-level or collection-level metadata alongside the database file:

```go
err := db.SetMetadataValue("app", "vecgrep")
dbMeta := db.Metadata()

err = coll.SetMetadataValue("embedding_profile", map[string]any{
    "provider":   "ollama",
    "model":      "nomic-embed-text",
    "dimensions": 768,
    "distance":   "cosine",
})
collMeta := coll.Metadata()

err = coll.DeleteMetadataValue("deprecated_key")
```

`Metadata` returns a deep copy. Use database and collection metadata for schema/profile information. Use record payloads for per-record fields that need filtering or retrieval.

**Distance metrics:**

| Metric | Constant | Interpretation |
|--------|----------|----------------|
| Cosine Similarity | `DistanceCosine` | Higher = more similar |
| Dot Product | `DistanceDot` | Higher = more similar |
| Euclidean | `DistanceEuclidean` | Lower = more similar |

### Inserting Vectors

```go
// Single insert with metadata
id, err := coll.Insert(vector, map[string]any{
    "file": "main.go",
    "type": "code",
})

// Batch insert
ids, err := coll.InsertBatch(
    [][]float32{v1, v2, v3},
    []map[string]any{p1, p2, p3},
)
```

### Document Storage

Store original text content alongside vectors for text search and retrieval:

```go
id, err := coll.InsertDocument(
    vector,
    "Go is a statically typed language designed at Google",
    map[string]any{"title": "Go Language", "category": "programming"},
)
```

The `Content` field is stored on the `Record` and automatically indexed by BM25 when text indexing is enabled.

For keyword-first workflows, store text without a vector:

```go
id, err := coll.InsertTextDocument(
    "00:12 OCR and transcript evidence",
    map[string]any{"frame": "frames/frame_0012.png"},
)

id, err = coll.InsertTextDocumentWithOptions(
    "temporary transcript evidence",
    map[string]any{"frame": "frames/frame_0013.png"},
    veclite.WithTTL(24 * time.Hour),
    veclite.WithImportance(0.8),
)
```

Text-only records are returned by `TextSearch`, filters, iteration, and direct lookup. Vector search skips them.

### Searching

```go
// Basic search
results, err := coll.Search(queryVector, veclite.TopK(10))

// With minimum similarity threshold
results, err := coll.Search(queryVector,
    veclite.TopK(10),
    veclite.Threshold(0.8),
)

// With HNSW tuning (higher ef = better recall, slower)
results, err := coll.Search(queryVector,
    veclite.TopK(10),
    veclite.WithEfSearch(200),
)

// Access results
for _, r := range results {
    fmt.Printf("ID: %d, Score: %.4f, Payload: %v\n",
        r.Record.ID, r.Score, r.Record.Payload)
}
```

| Search Option | Description |
|---------------|-------------|
| `TopK(k)` | Maximum results to return. Default: 10. |
| `Threshold(t)` | Minimum similarity score. |
| `WithFilter(f)` | Add a filter. Multiple filters use AND logic. |
| `WithFilters(f...)` | Add multiple filters (AND logic). |
| `WithEfSearch(ef)` | HNSW ef parameter. Higher = better recall, slower. |
| `WithOffset(n)` | Skip first n results (pagination). |
| `WithLimit(n)` | Alias for TopK in pagination contexts. |
| `WithContent(bool)` | Include/exclude `Content` field in results. |
| `WithVectorWeight(w)` | Vector search weight in hybrid search. Default: 1.0. |
| `WithTextWeight(w)` | Text search weight in hybrid search. Default: 1.0. |

### Text Search (BM25)

Full-text search using BM25 ranking. Requires `WithTextIndex` on the collection.

```go
coll, _ := db.CreateCollection("docs",
    veclite.WithTextIndex("title", "body"),
)

// Insert a keyword-searchable document without an embedding vector
coll.InsertTextDocument("Go programming language", map[string]any{
    "title": "Go Language",
    "body":  "Fast and efficient",
})

// Search by text
results, err := coll.TextSearch("Go programming", veclite.TopK(10))
```

BM25 indexes the `Content` field automatically, plus any payload fields specified in `WithTextIndex`. Uses standard BM25 parameters (k1=1.2, b=0.75).

### Hybrid Search

Combine vector similarity and BM25 text search using Reciprocal Rank Fusion (RRF):

```go
results, err := coll.HybridSearch(
    queryVector,
    "Go programming",
    veclite.TopK(10),
    veclite.WithVectorWeight(1.0),
    veclite.WithTextWeight(0.5),
)
```

RRF merges ranked lists from both searches with configurable weights. This produces better results than either search alone when queries have both semantic and keyword components.

### Named Vector Spaces

A collection has one implicit `default` space (backed by `Record.Vector`) and may declare
additional **named** spaces — each with its own dimension, distance metric, and HNSW index. One
logical record can then carry several embeddings at once (e.g. `text` and `image`). This is fully
additive: the entire single-vector API above keeps working on the default space, and databases
written before this feature load unchanged.

```go
coll, _ := db.CreateCollection("items",
    veclite.WithDimension(1536), // default space (text)
    veclite.WithVectorSpace(veclite.VectorSpaceConfig{
        Name: "image", Dimension: 512, Distance: veclite.DistanceCosine, Modality: "image",
        HNSW: &veclite.HNSWConfig{M: 16, EfConstruction: 200, EfSearch: 100, UseHeuristic: true},
    }),
)

// One logical record with vectors in two spaces.
id, _ := coll.InsertRecord(veclite.RecordInput{
    Content: "a red apple",
    Payload: map[string]any{"label": "apple"},
    Vectors: map[string][]float32{
        veclite.DefaultVectorSpace: textVector,
        "image":                    imageVector,
    },
})

// Search one space, or fuse several with Reciprocal Rank Fusion.
byImage, _ := coll.SearchSpace("image", imageQuery, veclite.TopK(10))
fused, _   := coll.MultiSpaceSearch(map[string][]float32{
    veclite.DefaultVectorSpace: textQuery,
    "image":                    imageQuery,
}, veclite.TopK(10))
```

`FuseRRF` is exposed publicly for custom weighted fusion across any result sets (vector spaces,
BM25, or externally produced rankings). Attach a first-class `EmbeddingProfile` to a collection
(`WithEmbeddingProfile`) or a space (`VectorSpaceConfig.Profile`) to validate inserts and detect
index-invalidating provider/model changes via `EmbeddingProfile.Compatible`.

Named spaces also have the upsert-by-key and hybrid-search analogs of the single-space API:

```go
// Upsert by a payload key, carrying vectors in several spaces atomically.
// Returns (id, inserted, err) — inserted=false means an existing record was replaced
// (its CreatedAt and AccessCount are preserved).
id, inserted, err := coll.UpsertRecordByKey("evidence_id", "doc-1", veclite.RecordInput{
    Content: "checkout fails",
    Payload: map[string]any{"evidence_id": "doc-1"},
    Vectors: map[string][]float32{"text": textVec},
})

// Hybrid search over a named space: fuses vector results from the space with
// BM25 text results via Reciprocal Rank Fusion. Passing "" or DefaultVectorSpace
// is equivalent to HybridSearch.
results, err := coll.HybridSearchSpace("text", queryVec, "checkout", veclite.TopK(10))
```

See the full guide: **[Named Vector Spaces](https://github.com/abdul-hamid-achik/veclite/blob/main/docs/guide/named-vector-spaces.md)**.

### Streaming Results

Process results one at a time via callback. Return `false` to stop early:

```go
err := coll.SearchStream(queryVector, func(r veclite.Result) bool {
    fmt.Printf("ID: %d, Score: %.4f\n", r.Record.ID, r.Score)
    return r.Score > 0.5 // stop when score drops below threshold
}, veclite.TopK(100))
```

### Filtering

Filter search results and find operations by metadata:

```go
// Equality
results, _ := coll.Search(query,
    veclite.TopK(10),
    veclite.WithFilter(veclite.Equal("type", "code")),
)

// Multiple filters (AND)
results, _ := coll.Search(query,
    veclite.TopK(10),
    veclite.WithFilters(
        veclite.Equal("language", "go"),
        veclite.Prefix("file", "src/"),
    ),
)

// Logical operators
results, _ := coll.Search(query,
    veclite.TopK(10),
    veclite.WithFilter(veclite.Or(
        veclite.Equal("lang", "go"),
        veclite.Equal("lang", "rust"),
    )),
)

// Range filters
results, _ := coll.Search(query,
    veclite.TopK(10),
    veclite.WithFilters(
        veclite.GT("score", 0.5),
        veclite.Between("line", 100, 500),
    ),
)

// Find records without vector search
records, _ := coll.Find(veclite.Equal("type", "code"))
record, _ := coll.FindOne(veclite.Equal("file", "main.go"))
```

**All filter functions:**

| Filter | Description |
|--------|-------------|
| `Equal(key, value)` | Exact match |
| `NotEqual(key, value)` | Not equal |
| `In(key, values...)` | Value in list |
| `NotIn(key, values...)` | Value not in list |
| `Glob(key, pattern)` | Glob pattern (e.g., `*.go`) |
| `Prefix(key, prefix)` | String prefix |
| `Suffix(key, suffix)` | String suffix |
| `Contains(key, substr)` | String contains |
| `Exists(key)` | Key exists in payload |
| `GT(key, value)` | Greater than (numeric) |
| `GTE(key, value)` | Greater than or equal |
| `LT(key, value)` | Less than (numeric) |
| `LTE(key, value)` | Less than or equal |
| `Between(key, min, max)` | Value in range (inclusive) |
| `And(filters...)` | All must match |
| `Or(filters...)` | Any can match |
| `Not(filter)` | Negate |

### Pagination

Use `WithOffset` and `TopK` (or `WithLimit`) to paginate search results:

```go
// Page 1
page1, _ := coll.Search(query, veclite.TopK(10))

// Page 2
page2, _ := coll.Search(query, veclite.TopK(10), veclite.WithOffset(10))

// Page 3
page3, _ := coll.Search(query, veclite.TopK(10), veclite.WithOffset(20))
```

### Iteration

Browse records without vector search:

```go
// Iterator with offset and limit
it := coll.Iterate(veclite.IterOffset(0), veclite.IterLimit(100))
for {
    record, ok := it.Next()
    if !ok {
        break
    }
    fmt.Println(record.ID, record.Payload)
}
it.Close()

// ForEach (return false to stop early)
coll.ForEach(func(r *veclite.Record) bool {
    fmt.Println(r.ID)
    return true // continue
})

// Get all records
all := coll.All()
```

### Upsert

Insert or update records:

```go
// Upsert by ID (0 = generate new ID)
id, err := coll.Upsert(0, vector, payload)      // insert new
id, err := coll.Upsert(42, vector, payload)     // update if exists, insert otherwise

// Upsert by key field (useful for incremental indexing)
id, wasInsert, err := coll.UpsertByKey("file", "main.go", vector, map[string]any{
    "file": "main.go",
    "line": 100,
})
```

### Migrating a Collection Layout

When your application changes its collection layout across versions (e.g. merging
two collections into one with a named vector space), use the read-transform-insert-drop
pattern with the existing API. There is no built-in `RenameCollection` because the
transformation is almost always application-specific.

```go
// Example: merge a BM25-only collection and a vector collection into one
// collection that uses a named "text" space for vectors.
oldText, _ := db.GetCollection("evidence_text")   // vectors + BM25
oldKeyword, _ := db.GetCollection("evidence_keyword") // BM25 only

merged, _ := db.CreateCollection("evidence",
    veclite.WithTextIndex("evidence_id"),
    veclite.WithVectorSpace(veclite.VectorSpaceConfig{Name: "text", Dimension: dim}),
)

// 1. Copy keyword records (Content + Payload, no vector yet).
for _, r := range oldKeyword.All() {
    merged.InsertRecord(veclite.RecordInput{
        Content: r.Content, Payload: r.Payload,
    })
}

// 2. Attach the matching vector from the text collection by natural key.
for _, r := range oldText.All() {
    key := r.Payload["evidence_id"]
    merged.UpsertRecordByKey("evidence_id", key, veclite.RecordInput{
        Content: r.Content, Payload: r.Payload,
        Vectors: map[string][]float32{"text": r.Vector},
    })
}

// 3. Drop the old collections.
db.DropCollection("evidence_text")
db.DropCollection("evidence_keyword")
```

This keeps the migration logic in the consumer (where the schema lives) and uses
only stable public APIs (`GetCollection`, `All`, `InsertRecord`, `UpsertRecordByKey`,
`DropCollection`).

### Updating Records

```go
// Update payload only (keep vector)
err := coll.Update(id, newPayload)

// Update vector only (keep payload)
err := coll.UpdateVector(id, newVector)
```

### Deleting Records

```go
// Delete by ID
err := coll.Delete(42)

// Delete by filter
count, err := coll.DeleteWhere(veclite.Equal("type", "temp"))

// Clear all records
err := coll.Clear()
```

### Auto-Embedding

Implement the `Embedder` interface to enable automatic text-to-vector conversion:

```go
type Embedder interface {
    Embed(text string) ([]float32, error)
    EmbedBatch(texts []string) ([][]float32, error)
    Dimension() int
}
```

Usage:

```go
coll, _ := db.CreateCollection("docs",
    veclite.WithEmbedder(myEmbedder),
)

// Insert text (auto-embeds to vector)
id, err := coll.InsertText("Go is a programming language", payload)

// Search by text (auto-embeds query)
results, err := coll.SearchText("programming languages", veclite.TopK(10))
```

The `Embedder` interface lives in the core library. VecLite provides built-in implementations for OpenAI, Ollama, and ONNX.

### Embedding Providers

VecLite supports multiple embedding providers out of the box:

| Provider | Package | Dimensions | Notes |
|----------|---------|------------|-------|
| OpenAI | `embed/openai` | 1536/3072 | API key required, best quality |
| Ollama | `embed/ollama` | 768/1024/384 | Local, no API key needed |
| ONNX | `embed/onnx` | 384 | Local, offline capable, build with `-tags onnx` |

### OpenAI Embedder

Use OpenAI's embedding API for high-quality embeddings:

```go
import "github.com/abdul-hamid-achik/veclite/embed/openai"

// Create embedder (uses OPENAI_API_KEY env var by default)
embedder, err := openai.NewEmbedder()

// Or with explicit options
embedder, err := openai.NewEmbedder(
    openai.WithAPIKey("sk-..."),
    openai.WithModel("text-embedding-3-small"),  // or text-embedding-3-large
    openai.WithDimension(512),                    // Reduced dimensions (optional)
)
if err != nil {
    log.Fatal(err)
}
defer embedder.Close()

// Use with veclite
db, _ := veclite.Open("data.veclite")
coll, _ := db.CreateCollection("docs",
    veclite.WithDimension(embedder.Dimension()),
    veclite.WithEmbedder(embedder),
)

coll.InsertText("Hello world", nil)
results, _ := coll.SearchText("greeting", veclite.TopK(5))
```

**OpenAI Models:**

| Model | Default Dimensions | Notes |
|-------|-------------------|-------|
| `text-embedding-3-small` | 1536 | Cheapest, good quality (default) |
| `text-embedding-3-large` | 3072 | Best quality |
| `text-embedding-ada-002` | 1536 | Legacy |

**Options:**

| Option | Description |
|--------|-------------|
| `WithAPIKey(key)` | Set API key (default: `OPENAI_API_KEY` env) |
| `WithModel(model)` | Set model (default: `text-embedding-3-small`) |
| `WithBaseURL(url)` | Custom API endpoint (Azure, proxies) |
| `WithDimension(dim)` | Reduced dimensions for v3 models |
| `WithTimeout(dur)` | Request timeout (default: 30s) |

### Ollama Embedder

Use Ollama for local embeddings with no API key required:

```go
import "github.com/abdul-hamid-achik/veclite/embed/ollama"

// Create embedder with defaults (localhost:11434, nomic-embed-text)
embedder, err := ollama.NewEmbedder()

// Or with options
embedder, err := ollama.NewEmbedder(
    ollama.WithBaseURL("http://localhost:11434"),
    ollama.WithModel("nomic-embed-text"),
)
if err != nil {
    log.Fatal(err)
}
defer embedder.Close()

// Use with veclite
db, _ := veclite.Open("data.veclite")
coll, _ := db.CreateCollection("docs",
    veclite.WithDimension(embedder.Dimension()),
    veclite.WithEmbedder(embedder),
)
```

**Prerequisites:** Pull the model first:

```bash
ollama pull nomic-embed-text
```

**Popular Ollama Models:**

| Model | Dimensions | Notes |
|-------|------------|-------|
| `nomic-embed-text` | 768 | Good general purpose (default) |
| `mxbai-embed-large` | 1024 | High quality |
| `all-minilm` | 384 | Fast, lightweight |

**Options:**

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Ollama API endpoint (default: `http://localhost:11434`) |
| `WithModel(model)` | Embedding model (default: `nomic-embed-text`) |
| `WithTimeout(dur)` | Request timeout (default: 30s) |

### Local ONNX Embedder

VecLite provides an optional local embedding system using ONNX Runtime with the `all-MiniLM-L6-v2` model. This enables:

- **Zero external API dependencies** -- No Ollama or OpenAI required
- **Offline capability** -- Text never leaves your machine
- **Low latency** -- ~10ms per embedding vs ~50ms with external services
- **384-dimensional vectors** -- Compatible with most vector search use cases

#### Quick Start (macOS)

Complete setup in 4 steps:

```bash
# 1. Install ONNX Runtime
brew install onnxruntime

# 2. Download libtokenizers
mkdir -p lib
curl -L -o lib/libtokenizers.tar.gz \
  "https://github.com/daulet/tokenizers/releases/latest/download/libtokenizers.darwin-arm64.tar.gz"
tar -xzf lib/libtokenizers.tar.gz -C lib && rm lib/libtokenizers.tar.gz

# 3. Download model files (~90MB)
mkdir -p models
curl -L -o models/tokenizer.json \
  "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
curl -L -o models/model.onnx \
  "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"

# 4. Build with ONNX support
CGO_LDFLAGS="-L./lib" go build -tags onnx ./...
```

#### Installation Details

The ONNX embedder requires native libraries and model files.

**Step 1: Install ONNX Runtime**

| Platform | Command |
|----------|---------|
| macOS (Homebrew) | `brew install onnxruntime` |
| Linux | Download from [onnxruntime releases](https://github.com/microsoft/onnxruntime/releases) |
| Windows | Download from [onnxruntime releases](https://github.com/microsoft/onnxruntime/releases) |

**Step 2: Install libtokenizers**

The `github.com/daulet/tokenizers` package requires the libtokenizers native library.

macOS ARM64 (Apple Silicon):
```bash
mkdir -p lib
curl -L -o lib/libtokenizers.tar.gz \
  "https://github.com/daulet/tokenizers/releases/latest/download/libtokenizers.darwin-arm64.tar.gz"
tar -xzf lib/libtokenizers.tar.gz -C lib && rm lib/libtokenizers.tar.gz
```

macOS Intel:
```bash
mkdir -p lib
curl -L -o lib/libtokenizers.tar.gz \
  "https://github.com/daulet/tokenizers/releases/latest/download/libtokenizers.darwin-amd64.tar.gz"
tar -xzf lib/libtokenizers.tar.gz -C lib && rm lib/libtokenizers.tar.gz
```

Linux:
```bash
mkdir -p lib
curl -L -o lib/libtokenizers.tar.gz \
  "https://github.com/daulet/tokenizers/releases/latest/download/libtokenizers.linux-amd64.tar.gz"
tar -xzf lib/libtokenizers.tar.gz -C lib && rm lib/libtokenizers.tar.gz
```

See [github.com/daulet/tokenizers](https://github.com/daulet/tokenizers) for other platforms.

**Step 3: Download Model Files**

Option A - Using curl (~90MB):
```bash
mkdir -p models
curl -L -o models/tokenizer.json \
  "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
curl -L -o models/model.onnx \
  "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
```

Option B - Quantized model (~25MB, slightly lower quality):
```bash
mkdir -p models
curl -L -o models/tokenizer.json \
  "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
curl -L -o models/model.onnx \
  "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model_quantized.onnx"
```

Option C - Using Go (requires native libs installed first):
```go
import "github.com/abdul-hamid-achik/veclite/embed/onnx"

// Full model
err := onnx.DownloadMiniLM("./models")

// Or quantized
err := onnx.DownloadMiniLMQuantized("./models")
```

**Step 4: Build**

```bash
# Set library path and build
CGO_LDFLAGS="-L./lib" go build -tags onnx ./...
```

#### Running Tests

```bash
# Run all ONNX tests
CGO_LDFLAGS="-L./lib" VECLITE_ONNX_MODEL_DIR=./models \
  go test -tags onnx -v ./embed/onnx/...

# Run benchmarks
CGO_LDFLAGS="-L./lib" VECLITE_ONNX_MODEL_DIR=./models \
  go test -tags onnx -bench=. -benchmem ./embed/onnx/...
```

#### Usage

```go
import (
    "github.com/abdul-hamid-achik/veclite"
    "github.com/abdul-hamid-achik/veclite/embed/onnx"
)

// Create ONNX embedder
embedder, err := onnx.NewMiniLM("./models")
if err != nil {
    log.Fatal(err)
}
defer embedder.Close()

// Use with veclite
db, _ := veclite.Open("data.veclite")
coll, _ := db.CreateCollection("docs",
    veclite.WithDimension(384),  // MiniLM outputs 384 dimensions
    veclite.WithEmbedder(embedder),
)

// Insert and search by text
id, _ := coll.InsertText("Go is a statically typed language", nil)
results, _ := coll.SearchText("programming languages", veclite.TopK(5))
```

#### Batch Embedding

For better performance with multiple texts, use batch embedding:

```go
texts := []string{
    "First document",
    "Second document",
    "Third document",
}

vectors, err := embedder.EmbedBatch(texts)
// Then insert with InsertBatch
```

#### Performance

Benchmarks on Apple M5:

| Operation | Time | Throughput |
|-----------|------|------------|
| Single embed | ~12ms | ~83 texts/sec |
| Batch 10 | ~100ms | ~100 texts/sec |
| Batch 100 | ~875ms | ~114 texts/sec |

Batching improves throughput by ~37% compared to single embedding.

#### Custom Models

You can use other ONNX-exported sentence transformers:

```go
embedder, err := onnx.NewEmbedder(
    "/path/to/model.onnx",
    "/path/to/tokenizer.json",
    onnx.WithDimension(768),    // For larger models
    onnx.WithMaxLength(512),    // Max token sequence length
)
```

#### Troubleshooting

**"library 'tokenizers' not found"**
```bash
# Ensure CGO_LDFLAGS points to the lib directory
CGO_LDFLAGS="-L/full/path/to/lib" go build -tags onnx ./...
```

**"onnxruntime.so not found"**
```bash
# The library auto-detects common paths. If not found, set manually:
export ONNXRUNTIME_LIB=/opt/homebrew/lib/libonnxruntime.dylib
```

**"model.onnx not found"**
```bash
# Ensure VECLITE_ONNX_MODEL_DIR points to the models directory
VECLITE_ONNX_MODEL_DIR=/full/path/to/models go test -tags onnx ./embed/onnx/...
```

### Config-Based Embedding

Configure embedding providers via `veclite.yaml` for easy switching between providers:

```yaml
# veclite.yaml
embedder:
  provider: openai  # openai | ollama | onnx

  openai:
    api_key: ${OPENAI_API_KEY}  # Supports env var expansion
    model: text-embedding-3-small
    dimension: 1536
    timeout: 30s

  ollama:
    base_url: http://localhost:11434
    model: nomic-embed-text
    timeout: 30s

  onnx:
    model_dir: ~/.veclite/models
```

**Usage:**

```go
import "github.com/abdul-hamid-achik/veclite"

// Load config (searches ./veclite.yaml, ~/.veclite/config.yaml)
cfg, err := veclite.LoadConfig("")

// Or from explicit path
cfg, err := veclite.LoadConfig("/path/to/veclite.yaml")

// Create embedder from config
embedder, err := veclite.NewEmbedderFromConfig(cfg.Embedder)
if err != nil {
    log.Fatal(err)
}
defer embedder.(interface{ Close() error }).Close()

// Use with veclite
db, _ := veclite.Open("data.veclite")
coll, _ := db.CreateCollection("docs",
    veclite.WithDimension(embedder.Dimension()),
    veclite.WithEmbedder(embedder),
)
```

**Environment Variable Expansion:**

The config supports `${VAR}` and `${VAR:-default}` syntax:

```yaml
embedder:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}           # Required env var
    base_url: ${OPENAI_BASE_URL:-https://api.openai.com/v1}  # With default
```

**Config Search Order:**

1. Explicit path provided to `LoadConfig()`
2. `./veclite.yaml` (current directory)
3. `~/.veclite/config.yaml` (home directory)
4. Returns default configuration if no file found

### HNSW Configuration

HNSW (Hierarchical Navigable Small World) provides approximate nearest neighbor search:

```go
// Basic
coll, _ := db.CreateCollection("vectors",
    veclite.WithHNSW(16, 200),
)

// Full configuration
coll, _ := db.CreateCollection("vectors",
    veclite.WithHNSWConfig(veclite.HNSWConfig{
        M:              32,
        EfConstruction: 400,
        EfSearch:       100,
    }),
)
```

| Parameter | Default | Range | Trade-off |
|-----------|---------|-------|-----------|
| M | 16 | 12-48 | Higher = better recall, more memory |
| EfConstruction | 200 | 100-500 | Higher = better index quality, slower build |
| EfSearch | 100 | 50-500 | Higher = better recall, slower search |

Without HNSW, search uses brute-force linear scan (exact results, slower on large datasets).

### Observability

#### Logger

Implement the `Logger` interface to integrate with your logging library:

```go
type Logger interface {
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
}
```

`NopLogger` is the default (zero overhead). Set a logger when opening the database:

```go
db, _ := veclite.Open("data.veclite", veclite.WithLogger(myLogger))
```

#### Metrics

VecLite tracks operation counts and latency using atomic counters:

```go
snapshot := db.Metrics()
fmt.Printf("Searches: %d, Inserts: %d, Deletes: %d, Avg Search: %v\n",
    snapshot.SearchCount,
    snapshot.InsertCount,
    snapshot.DeleteCount,
    snapshot.AvgSearchTime,
)
```

`MetricsSnapshot` fields:

| Field | Type | Description |
|-------|------|-------------|
| `SearchCount` | `int64` | Total search operations |
| `InsertCount` | `int64` | Total insert operations |
| `DeleteCount` | `int64` | Total delete operations |
| `AvgSearchTime` | `time.Duration` | Average search latency |

### Statistics

```go
// Database stats
dbStats := db.Stats()
fmt.Printf("Collections: %d, Total Records: %d\n",
    dbStats.Collections, dbStats.TotalRecords)

// Collection stats
collStats := coll.Stats()
fmt.Printf("Count: %d, Dimension: %d, Index: %s\n",
    collStats.Count, collStats.Dimension, collStats.IndexType)
```

## Agent Memory Features

VecLite includes features designed for AI agent memory systems, enabling intelligent storage, retrieval, and management of memories with temporal awareness, importance scoring, and relationship tracking.

### TTL and Expiration

Records can have a time-to-live (TTL) and expire automatically:

```go
// Insert with TTL
id, err := coll.InsertWithOptions(vector, payload,
    veclite.WithTTL(24 * time.Hour),  // Expires in 24 hours
)

// Insert with explicit expiration time
id, err := coll.InsertWithOptions(vector, payload,
    veclite.WithExpiresAt(time.Now().Add(7 * 24 * time.Hour)),
)

// Check if record is expired
record, _ := coll.Get(id)
if record.IsExpired() {
    fmt.Println("Record has expired")
}
if record.HasTTL() {
    fmt.Printf("TTL remaining: %v\n", record.TTL())
}

// Clean up expired records (manual)
count, err := coll.CleanupExpired()
fmt.Printf("Removed %d expired records\n", count)

// Count expired without removing
expiredCount := coll.CountExpired()

// Automatic background cleanup
stop := db.StartTTLCleaner(5*time.Minute, func(collection string, deleted int) {
    fmt.Printf("Cleaned %d expired records from %s\n", deleted, collection)
})
defer stop()  // Stop the cleaner when done
```

### Importance and Decay Scoring

Assign importance scores to records and apply temporal decay to search results:

```go
// Insert with importance score (0.0 to 1.0)
id, err := coll.InsertWithOptions(vector, payload,
    veclite.WithImportance(0.9),  // High importance
)

// Search with importance boost
results, err := coll.Search(query,
    veclite.TopK(10),
    veclite.WithImportanceBoost(1.5),  // Multiply score by importance * factor
)

// Apply temporal decay to search results
results, err := coll.Search(query,
    veclite.TopK(10),
    veclite.WithDecay(veclite.DecayExponential, 24*time.Hour),  // Half-life of 24 hours
)

// Enable access tracking (updates LastAccessedAt and AccessCount)
results, err := coll.Search(query,
    veclite.TopK(10),
    veclite.WithAccessTracking(true),
)
```

**Decay types:**

| Type | Behavior |
|------|----------|
| `DecayNone` | No decay applied |
| `DecayExponential` | Score halves every half-life period |
| `DecayLinear` | Score decreases linearly over time |
| `DecayGaussian` | Bell curve decay centered at creation time |

### Timestamp Filters

Filter records by creation time, update time, access time, and expiration:

```go
// Time-based filters
results, _ := coll.Search(query,
    veclite.WithFilter(veclite.CreatedAfter(time.Now().Add(-24*time.Hour))),
)

records, _ := coll.Find(
    veclite.AgeNewerThan(1 * time.Hour),      // Created within last hour
    veclite.UpdatedAfter(yesterday),           // Modified since yesterday
)

// TTL and expiration filters
activeRecords, _ := coll.Find(veclite.NotExpired())
expiringRecords, _ := coll.Find(veclite.ExpiredBefore(time.Now().Add(time.Hour)))
hasExpiration, _ := coll.Find(veclite.HasTTLFilter())

// Importance filters
important, _ := coll.Find(veclite.ImportanceAbove(0.7))
lowPriority, _ := coll.Find(veclite.ImportanceBelow(0.3))
midRange, _ := coll.Find(veclite.ImportanceBetween(0.4, 0.6))

// Access tracking filters
recentlyAccessed, _ := coll.Find(veclite.AccessedAfter(time.Now().Add(-time.Hour)))
neverAccessed, _ := coll.Find(veclite.NeverAccessed())
frequentlyAccessed, _ := coll.Find(veclite.AccessCountAbove(10))
```

### Conversation Memory

Track conversation turns with session and thread support:

```go
// Insert a conversation turn
id, err := coll.InsertTurn(veclite.ConversationTurn{
    SessionID:  "session-123",
    TurnNumber: 1,
    Role:       "user",
    Content:    "Hello, how are you?",
    Vector:     vector,  // Or use embedder
    Importance: 0.5,
    TTL:        24 * time.Hour,
})

// Insert a reply (threaded)
replyID, err := coll.InsertTurn(veclite.ConversationTurn{
    SessionID:     "session-123",
    TurnNumber:    2,
    Role:          "assistant",
    Content:       "I'm doing well, thank you!",
    Vector:        vector,
    ParentChunkID: id,  // Links to parent
})

// Get all turns in a session (ordered by turn number)
turns, err := coll.GetSession("session-123")

// Get a conversation thread (follows parent-child links)
thread, err := coll.GetThread(id)

// Search within a specific session
results, err := coll.SearchInSession("session-123", query, veclite.TopK(5))

// List all sessions and get stats
sessions := coll.ListSessions()
stats, err := coll.GetSessionStats("session-123")
fmt.Printf("Turns: %d, Roles: %v\n", stats.TurnCount, stats.Roles)
```

### Subscriptions (Real-time Notifications)

Subscribe to be notified when new records match a query:

```go
// Subscribe to matching records
sub, err := coll.Subscribe(
    queryVector,
    veclite.WithSubscriptionThreshold(0.8),    // Minimum similarity
    veclite.WithSubscriptionFilter(veclite.Equal("type", "important")),
    veclite.WithSubscriptionBufferSize(100),   // Event buffer size
)
defer sub.Close()

// Listen for matching records (non-blocking)
go func() {
    for event := range sub.Events() {
        fmt.Printf("New match: ID=%d, Score=%.4f\n",
            event.Record.ID, event.Score)
    }
}()

// Insert triggers notifications automatically
coll.Insert(similarVector, map[string]any{"type": "important"})

// Unsubscribe when done
coll.Unsubscribe(sub.ID)
```

### Memory Consolidation

Cluster similar memories and consolidate them into summaries:

```go
// Find clusters of similar memories
clusters, err := coll.FindSimilarClusters(veclite.ConsolidationConfig{
    SimilarityThreshold: 0.9,   // How similar records must be
    MinGroupSize:        3,     // Minimum cluster size
    MaxGroupSize:        10,    // Maximum cluster size
    Filters:             []veclite.Filter{veclite.NotExpired()},
})

for _, cluster := range clusters {
    fmt.Printf("Cluster %s: %d records, avg importance: %.2f\n",
        cluster.ID, len(cluster.Records), cluster.AverageImportance)
}

// Consolidate clusters into summary records
result, err := coll.Consolidate(veclite.ConsolidationConfig{
    SimilarityThreshold: 0.9,
    MinGroupSize:        3,
    ArchiveOriginals:    true,  // Archive source records
    Embedder:            embedder,
    SummaryGenerator: func(records []*veclite.Record) (string, map[string]any, error) {
        // Generate summary from records (e.g., using LLM)
        summary := "Summary of " + strconv.Itoa(len(records)) + " memories"
        return summary, map[string]any{"source": "consolidation"}, nil
    },
})

fmt.Printf("Consolidated %d records into %d summaries\n",
    result.RecordsConsolidated, len(result.ConsolidatedRecordIDs))

// Archive/unarchive records manually
coll.ArchiveRecord(id)
coll.UnarchiveRecord(id)
archived, _ := coll.GetArchived()

// Get all consolidation records
consolidations, _ := coll.GetConsolidations()

// Expand a consolidation to see original records
originals, _ := coll.ExpandConsolidation(consolidationID)
```

### Episodic Memory

Group related memories into coherent episodes:

```go
// Create an episode store for a collection
episodeStore, err := db.CreateEpisodeStore("memories")

// Manually create an episode from record IDs
episode, err := episodeStore.CreateEpisode(
    []uint64{id1, id2, id3},
    "Morning standup meeting",
)

// Automatically detect episodes based on time gaps
episodes, err := episodeStore.DetectEpisodes(veclite.EpisodeConfig{
    TimeGapThreshold: 30 * time.Minute,  // Gap between episodes
    MinRecords:       2,                  // Minimum records per episode
    MaxRecords:       100,                // Maximum records per episode
})

// Get episode details
episode, _ := episodeStore.GetEpisode(episodeID)
fmt.Printf("Episode: %s, Duration: %v, Records: %d\n",
    episode.Title, episode.Duration(), episode.RecordCount())

// Expand an episode to get all its records
records, _ := episodeStore.ExpandEpisode(episodeID)

// Search with episode expansion (includes context from same episode)
results, err := episodeStore.SearchWithEpisodeExpansion(query, veclite.TopK(10))
for _, r := range results {
    if r.Episode != nil {
        fmt.Printf("Match in episode: %s (%d related records)\n",
            r.Episode.Title, len(r.EpisodeRecords))
    }
}

// Search for similar episodes
episodes, _ := episodeStore.SearchEpisodes(query, 5)

// Find which episode contains a record
episode, _ := episodeStore.FindRecordEpisode(recordID)

// List and delete episodes
allEpisodes := episodeStore.ListEpisodes()
episodeStore.DeleteEpisode(episodeID)
```

### Memory Pressure Handling

Manage collection size with automatic eviction policies:

```go
// Configure memory limits when creating a collection
coll, _ := db.CreateCollection("memories",
    veclite.WithDimension(384),
    veclite.WithMemoryLimits(veclite.MemoryConfig{
        MaxRecords:        10000,           // Maximum records allowed
        EvictionPolicy:    "importance",    // "fifo", "lru", or "importance"
        EvictionBatchSize: 100,             // Records to evict per cycle
        CleanupInterval:   5 * time.Minute, // Background check interval
    }),
)

// Or enforce limits manually after inserts
evicted := coll.EnforceMemoryLimit(veclite.MemoryConfig{
    MaxRecords:     5000,
    EvictionPolicy: "fifo",  // Remove oldest records first
})
fmt.Printf("Evicted %d records\n", evicted)

// Start a background memory limiter
stop := coll.StartMemoryLimiter(veclite.MemoryConfig{
    MaxRecords:        10000,
    EvictionPolicy:    "lru",        // Remove least recently accessed
    CleanupInterval:   time.Minute,
    EvictionBatchSize: 50,
})
defer stop()
```

**Eviction Policies:**

| Policy | Behavior |
|--------|----------|
| `fifo` | Remove oldest records first (by creation time) |
| `lru` | Remove least recently accessed records first |
| `importance` | Remove lowest importance records first |

Archived records (via `ArchiveRecord`) are never evicted regardless of policy.

### Knowledge Graph

Build entity-relationship graphs with vector search:

```go
// Create a knowledge graph
kg, err := db.CreateKnowledgeGraph("knowledge")

// Add entities with vectors
kg.AddEntity(veclite.Entity{
    ID:     "alice",
    Type:   "person",
    Name:   "Alice Smith",
    Vector: aliceVector,
    Properties: map[string]any{
        "role": "engineer",
        "team": "backend",
    },
})

kg.AddEntity(veclite.Entity{
    ID:     "acme",
    Type:   "company",
    Name:   "Acme Corp",
    Vector: acmeVector,
})

// Add relationships
kg.AddRelationship(veclite.Relationship{
    ID:       "rel-1",
    SourceID: "alice",
    TargetID: "acme",
    Type:     "works_at",
    Weight:   0.9,
})

kg.AddRelationship(veclite.Relationship{
    ID:            "rel-2",
    SourceID:      "alice",
    TargetID:      "bob",
    Type:          "knows",
    Weight:        0.8,
    Bidirectional: true,  // Creates edges in both directions
})

// Traverse the graph
result, err := kg.Traverse([]string{"alice"}, veclite.TraversalConfig{
    MaxDepth:          2,                       // How far to traverse
    MaxNodes:          100,                     // Maximum nodes to visit
    MinWeight:         0.5,                     // Minimum relationship weight
    RelationshipTypes: []string{"knows"},       // Filter by relationship type
    EntityTypes:       []string{"person"},      // Filter by entity type
    Direction:         "both",                  // "outgoing", "incoming", or "both"
})

for _, entity := range result.Entities {
    fmt.Printf("Found: %s (depth %d)\n", entity.Name, result.Depths[entity.ID])
}

// Search with graph expansion
results, err := kg.SearchWithExpansion(query,
    veclite.TraversalConfig{MaxDepth: 1},
    veclite.TopK(10),
)

for _, r := range results {
    fmt.Printf("Entity: %s (score: %.4f)\n", r.Entity.Name, r.Score)
    fmt.Printf("  Related: %d entities via %d relationships\n",
        len(r.RelatedEntities), len(r.Relationships))
}

// Get graph statistics
stats := kg.Stats()
fmt.Printf("Entities: %d, Relationships: %d\n",
    stats.EntityCount, stats.RelationshipCount)
fmt.Printf("Entity types: %v\n", stats.EntityTypes)
fmt.Printf("Relationship types: %v\n", stats.RelationshipTypes)

// Entity and relationship management
entity, _ := kg.GetEntity("alice")
kg.UpdateEntity(updatedEntity)
kg.DeleteEntity("alice")  // Also removes relationships

rel, _ := kg.GetRelationship("rel-1")
kg.DeleteRelationship("rel-1")

// Get relationships for an entity
outgoing := kg.GetRelationships("alice", "outgoing")
incoming := kg.GetRelationships("alice", "incoming")
all := kg.GetRelationships("alice", "both")
```

## CLI Usage

```bash
veclite <command> [arguments]
```

### Commands

#### Read Commands

| Command | Description |
|---------|-------------|
| `version` | Show version information |
| `info <file>` | Show database summary |
| `collections <file>` | List all collections |
| `stats <file>` | Show detailed statistics |
| `dump <file>` | Export database as JSON |
| `get <file> <collection>` | Get a vector by ID |

#### Write Commands

| Command | Description |
|---------|-------------|
| `create-collection <file> <name>` | Create a new collection |
| `drop-collection <file> <name>` | Drop a collection |
| `insert <file> <collection>` | Insert a vector |
| `batch-insert <file> <collection>` | Insert vectors from JSON file |
| `delete <file> <collection>` | Delete a vector by ID |
| `upsert <file> <collection>` | Insert or update a vector |
| `update <file> <collection>` | Update payload for a record |
| `delete-where <file> <collection>` | Delete records matching a filter |
| `find <file> <collection>` | Find records by filter |
| `search <file> <collection>` | Search for similar vectors |

#### Named Vector Spaces

| Command | Description |
|---------|-------------|
| `spaces <file> <collection>` | List a collection's vector spaces |
| `space-add <file> <collection>` | Declare a named vector space (`--name`, `--dim`, `--distance`, `--modality`, `--hnsw`) |
| `record-insert <file> <collection>` | Insert a record with vectors in several spaces (`--vectors`, `--input`) |
| `record-upsert-by-key <file> <collection>` | Insert or replace a multi-space record by a payload key (`--key-field`, `--key-value`, `--vectors`) |
| `search-space <file> <collection> <space>` | Search a single named vector space |
| `hybrid-search-space <file> <collection> <space>` | Hybrid vector+BM25 search over a named space (`--query`, `--text`) |
| `fuse-search <file> <collection>` | Search several spaces and fuse with RRF (`--queries`) |

#### Server Mode

| Command | Description |
|---------|-------------|
| `serve <file>` | Start HTTP server |
| `mcp <file>` | Start MCP tool server over stdio |

#### Maintenance

| Command | Description |
|---------|-------------|
| `compact <file>` | Compact database and reclaim space |
| `validate <file>` | Validate database integrity |
| `benchmark <file>` | Run search performance benchmark |

All commands support `--json` for JSON output.

### CLI Examples

```bash
# Create a collection with HNSW index and text search
veclite create-collection data.veclite embeddings \
    --dimension=384 --distance=cosine --hnsw --text-index=title,body

# Insert a vector
veclite insert data.veclite embeddings \
    --vector='[0.1,0.2,0.3,...]' --payload='{"file":"main.go"}'

# Batch insert from JSON
veclite batch-insert data.veclite embeddings --input=vectors.json

# Search
veclite search data.veclite embeddings \
    --query='[0.1,0.2,0.3,...]' --top-k=10 --filter='type=code'

# Named vector spaces: declare a space, insert a multi-space record, search and fuse
veclite space-add data.veclite items --name=image --dim=512 --modality=image --hnsw
veclite record-insert data.veclite items \
    --vectors='{"default":[0.1,0.2],"image":[0.3,0.4]}' --content='a red apple'
veclite spaces data.veclite items --json
veclite search-space data.veclite items image --query='[0.3,0.4]' --top-k=5
veclite fuse-search data.veclite items \
    --queries='{"default":[0.1,0.2],"image":[0.3,0.4]}' --top-k=10

# Named spaces: upsert by a payload key (idempotent), then hybrid search over a named space
veclite record-upsert-by-key data.veclite evidence \
    --key-field=evidence_id --key-value=doc-1 \
    --vectors='{"text":[0.1,0.2,0.3]}' --content='checkout fails'
veclite hybrid-search-space data.veclite evidence text \
    --query='[0.1,0.2,0.3]' --text='checkout' --top-k=5

# Upsert
veclite upsert data.veclite embeddings \
    --id=42 --vector='[0.1,0.2,0.3,...]' --payload='{"file":"main.go"}'

# Find by filter
veclite find data.veclite embeddings --filter='type=code'

# Delete by filter
veclite delete-where data.veclite embeddings --filter='type=temp'

# Start HTTP server
veclite serve data.veclite --port=8080 --cors

# Start MCP server
veclite mcp data.veclite
```

### Batch Insert File Format

**JSON Array:**
```json
[
  {"vector": [0.1, 0.2, 0.3], "payload": {"file": "a.go"}},
  {"vector": [0.4, 0.5, 0.6], "payload": {"file": "b.go"}}
]
```

**JSONL (one object per line):**
```json
{"vector": [0.1, 0.2, 0.3], "payload": {"file": "a.go"}}
{"vector": [0.4, 0.5, 0.6], "payload": {"file": "b.go"}}
```

## HTTP Server

```bash
veclite serve data.veclite --port=8080 --host=127.0.0.1 --cors

# Long-running servers should enable the write-ahead log so accepted writes
# survive a crash between syncs (see "Durability and the Write-Ahead Log").
veclite serve data.veclite --wal
```

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/info` | Database info |
| GET | `/metrics` | Operation metrics |
| GET | `/collections` | List collections |
| POST | `/collections` | Create collection |
| GET | `/collections/{name}` | Collection info |
| DELETE | `/collections/{name}` | Drop collection |
| GET | `/collections/{name}/vectors` | List all vectors (paginated) |
| POST | `/collections/{name}/vectors` | Insert vector(s) |
| GET | `/collections/{name}/vectors/{id}` | Get vector by ID |
| PUT | `/collections/{name}/vectors/{id}` | Update vector and/or payload |
| DELETE | `/collections/{name}/vectors/{id}` | Delete vector |
| DELETE | `/collections/{name}/vectors` | Delete by filter (with body) |
| POST | `/collections/{name}/search` | Search vectors |
| GET | `/collections/{name}/spaces` | List vector spaces |
| POST | `/collections/{name}/spaces` | Add a named vector space |
| POST | `/collections/{name}/records` | Insert a multi-space record |
| POST | `/collections/{name}/records-upsert-by-key` | Upsert a multi-space record by a payload key |
| POST | `/collections/{name}/search-space` | Search one named vector space |
| POST | `/collections/{name}/hybrid-search-space` | Hybrid vector+BM25 search over a named space |
| POST | `/collections/{name}/fuse-search` | Fuse search across vector spaces |
| POST | `/collections/{name}/upsert` | Upsert vector |
| POST | `/collections/{name}/find` | Find records by filter |
| POST | `/collections/{name}/compact` | Compact collection |
| POST | `/collections/{name}/validate` | Validate integrity |
| POST | `/sync` | Force sync to disk |
| POST | `/reload` | Reload database from disk (pick up external writes) |

### Streaming Results

Set `Accept: application/x-ndjson` on search requests to receive results as newline-delimited JSON:

```bash
curl -X POST http://localhost:8080/collections/embeddings/search \
  -H "Accept: application/x-ndjson" \
  -H "Content-Type: application/json" \
  -d '{"query": [0.1, 0.2, 0.3], "top_k": 100}'
```

Each line is a JSON object:

```json
{"id":1,"score":0.9821,"payload":{"file":"main.go"}}
{"id":5,"score":0.9654,"payload":{"file":"util.go"}}
```

### API Examples

**Create Collection:**
```bash
curl -X POST http://localhost:8080/collections \
  -H "Content-Type: application/json" \
  -d '{"name": "embeddings", "dimension": 384, "distance": "cosine", "hnsw": true}'
```

**Insert Vector:**
```bash
curl -X POST http://localhost:8080/collections/embeddings/vectors \
  -H "Content-Type: application/json" \
  -d '{"vector": [0.1, 0.2, 0.3], "payload": {"file": "main.go"}}'
```

**Batch Insert:**
```bash
curl -X POST http://localhost:8080/collections/embeddings/vectors \
  -H "Content-Type: application/json" \
  -d '{"vectors": [[0.1,0.2,0.3], [0.4,0.5,0.6]], "payloads": [{"file":"a.go"}, {"file":"b.go"}]}'
```

**Search:**
```bash
curl -X POST http://localhost:8080/collections/embeddings/search \
  -H "Content-Type: application/json" \
  -d '{"query": [0.1, 0.2, 0.3], "top_k": 10}'
```

**Search with Filters:**
```bash
curl -X POST http://localhost:8080/collections/embeddings/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": [0.1, 0.2, 0.3],
    "top_k": 10,
    "filters": [{"key": "type", "op": "eq", "value": "code"}]
  }'
```

**Upsert:**
```bash
curl -X POST http://localhost:8080/collections/embeddings/upsert \
  -H "Content-Type: application/json" \
  -d '{"id": 42, "vector": [0.1, 0.2, 0.3], "payload": {"file": "main.go"}}'
```

**Find by Filter:**
```bash
curl -X POST http://localhost:8080/collections/embeddings/find \
  -H "Content-Type: application/json" \
  -d '{"filters": [{"key": "type", "op": "eq", "value": "code"}]}'
```

**Update Vector:**
```bash
curl -X PUT http://localhost:8080/collections/embeddings/vectors/42 \
  -H "Content-Type: application/json" \
  -d '{"vector": [0.4, 0.5, 0.6], "payload": {"file": "updated.go"}}'
```

### Filter Operators

| Operator | Aliases | Description |
|----------|---------|-------------|
| `eq` | `=` | Equal |
| `neq` | `!=` | Not equal |
| `gt` | `>` | Greater than (numeric) |
| `gte` | `>=` | Greater than or equal |
| `lt` | `<` | Less than (numeric) |
| `lte` | `<=` | Less than or equal |
| `glob` | | Glob pattern match |
| `prefix` | | String prefix |
| `suffix` | | String suffix |
| `contains` | | String contains |
| `exists` | | Key exists |

### Python Client Example

```python
import requests

base = "http://localhost:8080"

# Create collection
requests.post(f"{base}/collections", json={
    "name": "embeddings", "dimension": 384, "hnsw": True
})

# Insert
r = requests.post(f"{base}/collections/embeddings/vectors", json={
    "vector": [0.1] * 384,
    "payload": {"file": "main.py"}
})
print(r.json())  # {"status": "inserted", "id": 1}

# Search
r = requests.post(f"{base}/collections/embeddings/search", json={
    "query": [0.1] * 384, "top_k": 5
})
print(r.json())  # {"results": [...], "count": 5}
```

### Go Client

For Go consumers that need multi-process access, use the `client` package to talk to a
running `veclite serve` instance. The API mirrors the embedded library so you can swap
between `veclite.Open(path)` (single-process) and `client.Open(url)` (multi-process) with
minimal code change:

```go
import "github.com/abdul-hamid-achik/veclite/client"

// Instead of:  db, _ := veclite.Open("data.veclite")
// Use:          db, _ := client.Open("http://localhost:8080")

db, err := client.Open("http://localhost:8080")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Create a collection
coll, err := db.CreateCollection("docs",
    client.WithDimension(384),
    client.WithHNSW(16, 200),
)

// Insert
id, err := coll.Insert([]float32{0.1, 0.2, 0.3}, map[string]any{"source": "wiki"})

// Search
results, err := coll.Search([]float32{0.1, 0.2, 0.3}, client.TopK(10))

// Find by filter
records, err := coll.Find(client.Equal("source", "wiki"))

// Upsert by key
id, inserted, err := coll.UpsertByKey("file", "main.go", vec, payload)

// Sync and reload
db.Sync()   // force write to disk
db.Reload() // pick up external writes
```

The client supports the same filter operators (`Equal`, `GT`, `Glob`, `Prefix`, ...),
search options (`TopK`, `Threshold`, `WithFilter`), and collection options
(`WithDimension`, `WithHNSW`, `WithTextIndex`) as the embedded library.

See the [Go client API reference](docs/guide/go-client.md) for the full surface.

## MCP Tool Server

VecLite can run as an [MCP](https://modelcontextprotocol.io/) tool server, making it available to AI agents like Claude Code, Cursor, and other MCP-compatible clients.

```bash
veclite mcp data.veclite
```

The server communicates over stdio using JSON-RPC 2.0. It exposes **56 tools** across several categories:

### Core Vector Operations

| Tool | Description |
|------|-------------|
| `veclite_collections` | List all collections with stats |
| `veclite_stats` | Get database statistics |
| `veclite_search` | Vector similarity search |
| `veclite_text_search` | BM25 full-text search |
| `veclite_hybrid_search` | Combined vector + text search with RRF fusion |
| `veclite_find` | Find records by filter |
| `veclite_insert` | Insert a vector with optional payload and content |
| `veclite_get` | Retrieve a record by ID |
| `veclite_delete` | Delete a record by ID |
| `veclite_update` | Update a record's payload |
| `veclite_upsert` | Insert or update by ID |
| `veclite_delete_where` | Delete records matching filter conditions |
| `veclite_clear` | Clear all records from a collection (requires `confirm: true`) |
| `veclite_insert_batch` | Bulk insert multiple vectors |
| `veclite_upsert_by_key` | Insert or update by payload key field |
| `veclite_embed` | Convert text to vector using configured embedder |

### Collection Management

| Tool | Description |
|------|-------------|
| `veclite_create_collection` | Create a collection with options (dimension, distance, index) |
| `veclite_drop_collection` | Delete a collection (requires `confirm: true`) |
| `veclite_collection_schema` | Discover a collection's schema: payload fields, types, vector dimension, index type, content availability |
| `veclite_sync` | Force persist all changes to disk |
| `veclite_metrics` | Get performance metrics (search/insert/delete counts) |

### Agent Memory Tools

| Tool | Description |
|------|-------------|
| `memory_remember` | Store a memory with importance, tags, and TTL |
| `memory_recall` | Semantic search for memories with filters |
| `memory_forget` | Remove memories by criteria |
| `memory_enforce_limit` | Enforce memory limit with eviction policy (fifo/lru/importance) |
| `memory_consolidate` | Find similar memory clusters |
| `memory_expand_consolidation` | Get original records from a consolidation |

### Knowledge Graph Tools

| Tool | Description |
|------|-------------|
| `graph_add_entity` | Add an entity node with optional vector |
| `graph_add_relationship` | Add a relationship edge between entities |
| `graph_get_entity` | Get an entity by ID |
| `graph_update_entity` | Update an existing entity |
| `graph_delete_entity` | Delete an entity and its relationships |
| `graph_get_relationships` | Get relationships for an entity |
| `graph_delete_relationship` | Delete a relationship by ID |
| `graph_list_entities` | List entities, optionally filtered by type |
| `graph_traverse` | BFS traversal from starting entities |
| `graph_expanded_search` | Vector search with graph context expansion |

### Conversation Memory Tools

| Tool | Description |
|------|-------------|
| `conversation_add_turn` | Add a conversation turn with session tracking |
| `conversation_get_session` | Get all turns in a session |
| `conversation_search_session` | Search within a specific session |
| `conversation_list_sessions` | List all session IDs |
| `conversation_get_thread` | Get a conversation thread by chunk ID |
| `conversation_delete_session` | Delete all turns in a session (requires `confirm: true`) |
| `conversation_get_stats` | Get session statistics (turn count, roles, duration) |

### Episodic Memory Tools

| Tool | Description |
|------|-------------|
| `episode_detect` | Auto-detect episodes using time gaps and similarity |
| `episode_create` | Create an episode from record IDs |
| `episode_get` | Get episode details including records |
| `episode_list` | List all episodes in a collection |
| `episode_search` | Search episodes by vector similarity |
| `episode_search_expanded` | Search with episode context expansion |

### Memory Consolidation Tools

| Tool | Description |
|------|-------------|
| `memory_find_clusters` | Find clusters of similar memories |
| `memory_archive` | Archive a memory (excludes from searches, protects from eviction) |
| `memory_unarchive` | Restore an archived memory |
| `memory_get_archived` | List archived memories |

### TTL/Cleanup Tools

| Tool | Description |
|------|-------------|
| `veclite_cleanup_expired` | Remove all expired records from a collection |
| `veclite_count_expired` | Count expired records without removing them |

### MCP Configuration

Add to your MCP client configuration (e.g., `.claude/settings.json`):

```json
{
  "mcpServers": {
    "veclite": {
      "command": "veclite",
      "args": ["mcp", "/path/to/data.veclite"]
    }
  }
}
```

## Examples

Runnable examples are in the `examples/` directory:

```bash
go run ./examples/basic        # Open, insert, search, close
go run ./examples/hnsw         # HNSW index configuration and benchmarking
go run ./examples/filtering    # Rich filter expressions
go run ./examples/batch        # Batch operations, upsert, iteration, pagination
go run ./examples/http-client  # HTTP API client (start server first)
```

All examples use `:memory:` for zero-setup running (except `http-client`, which connects to a running server).

## Performance

Benchmark results on 10,000 384-dimensional vectors:

| Method | Time | Speedup |
|--------|------|---------|
| Brute Force | ~2.5ms | 1x |
| HNSW | ~0.4ms | 6x |

HNSW provides >95% recall at 6-7x speedup over brute force.

## Thread Safety

VecLite is safe for concurrent access:
- Multiple goroutines can read simultaneously
- Writes are serialized with `sync.RWMutex`
- Metrics use atomic counters for lock-free reads
- Use `WithSyncOnWrite(true)` for durability after each write

### Multi-Process Access

By default, VecLite uses an **exclusive file lock** for writers — only one
process at a time can open the database read-write. This prevents data
corruption from concurrent writes (VecLite is an in-memory DB that persists by
rewriting the entire snapshot file via an atomic replace).

For **multi-process read access**, use `WithSharedRead(true)` together with
`WithReadOnly(true)`. Read-only opens are **lock-free**: they take no flock at
all, so a long-lived reader (an MCP server, a daemon's query client, a CLI
`search`) never blocks a writer, and a writer's exclusive lock never blocks a
read-only open. Consistency is guaranteed by the writer's atomic-replace save
(`.tmp` → rename): a reader's `os.Open` always resolves to a complete old or
new snapshot, never a torn write.

```go
// Writer process — exclusive lock (only one writer at a time)
db, err := veclite.Open("data.veclite",
    veclite.WithSyncOnWrite(true),
)

// Reader process(es) — lock-free, no conflict with the writer
reader, err := veclite.Open("data.veclite",
    veclite.WithReadOnly(true),
    veclite.WithSharedRead(true),
)

defer reader.Close()

// Readers see a point-in-time snapshot taken at Open.
// Call Reload() to pick up changes written by other processes:
if err := reader.Reload(); err != nil {
    log.Fatal(err)
}
```

Key points:
- **Writer** holds an exclusive lock (`LOCK_EX`) — only one writer at a time; a second writer gets `ErrFileLocked`
- **Readers** with `WithSharedRead(true)` take **no lock** — they never block a writer and are never blocked by one
- Readers see a **point-in-time snapshot** from when they opened; they do not auto-detect changes
- Call `db.Reload()` to discard the in-memory state and re-load the latest snapshot from disk
- `Reload()` rebuilds all collections, HNSW indexes, BM25 indexes, and knowledge graphs
- All read-only CLI commands (`veclite search`, `veclite stats`, etc.) already use `WithSharedRead(true)`
- The `veclite serve` HTTP server remains the canonical multi-client surface for write-heavy workloads
- For multi-process **write** access, run `veclite serve` and use the [Go client](#go-client) from other processes

## Persistence

Data is stored using Go's gob encoding in a single `.veclite` file:
- Call `db.Sync()` to persist changes manually
- Use `WithSyncOnWrite(true)` for automatic persistence after each write
- `db.Close()` syncs before closing
- HNSW indexes, BM25 inverted indexes, and record content are all persisted
- File locking prevents concurrent writer access; `WithSharedRead(true)` enables multi-process read-only access
- CRC32 checksums validate data integrity on load
- New snapshots and lock files use owner-only permissions; a save also tightens legacy broad database modes while preserving an already stricter owner-only mode

## Contributing

Contributions welcome. Please:
1. Run `go test -race ./...` before submitting
2. Add tests for new features
3. Follow existing code style

## License

MIT License - see [LICENSE](LICENSE)
