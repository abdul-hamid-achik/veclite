# VecLite

Embeddable vector database for Go with zero external dependencies.

Store vectors with metadata in a single file. Search with HNSW for fast approximate nearest neighbors.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [CLI Usage](#cli-usage)
- [HTTP Server Mode](#http-server-mode)
- [Library API](#library-api)
- [Performance](#performance)
- [Contributing](#contributing)

## Features

- **Zero dependencies** - Standard library only, no CGO
- **Single-file storage** - Database persists to one `.veclite` file
- **HNSW indexing** - Fast approximate nearest neighbor search
- **Metadata filtering** - Filter results by payload fields
- **Thread-safe** - Safe for concurrent read/write access
- **In-memory mode** - Use `:memory:` for testing

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/abdul-hamid-achik/veclite"
)

func main() {
    // Open database (creates if not exists)
    db, _ := veclite.Open("vectors.veclite")
    defer db.Close()

    // Get or create collection with HNSW index
    coll, _ := db.CreateCollection("embeddings",
        veclite.WithDimension(384),
        veclite.WithHNSW(16, 200),
    )

    // Insert vectors with metadata
    coll.Insert([]float32{0.1, 0.2, ...}, map[string]any{"file": "main.go"})

    // Search for similar vectors
    results, _ := coll.Search(queryVector, veclite.TopK(10))
    for _, r := range results {
        fmt.Printf("ID: %d, Score: %.4f\n", r.Record.ID, r.Score)
    }
}
```

## Prerequisites

- Go 1.21 or later
- No external dependencies required

## Installation

```bash
go get github.com/abdul-hamid-achik/veclite
```

### CLI Installation

```bash
go install github.com/abdul-hamid-achik/veclite/cmd/veclite@latest
```

Or download from [Releases](https://github.com/abdul-hamid-achik/veclite/releases).

## CLI Usage

The `veclite` CLI provides full read/write access to VecLite database files.

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
| `search <file> <collection>` | Search for similar vectors |

#### Server Mode

| Command | Description |
|---------|-------------|
| `serve <file>` | Start HTTP server for multi-client access |

#### Maintenance Commands

| Command | Description |
|---------|-------------|
| `compact <file>` | Compact database and reclaim space |
| `validate <file>` | Validate database integrity |
| `benchmark <file>` | Run search performance benchmark |

### Global Flags

All commands support `--json` flag for JSON output (useful for scripting).

### Examples

```bash
# Check version
veclite version

# View database info
veclite info data.veclite
veclite info --json data.veclite

# Create a collection with HNSW index
veclite create-collection data.veclite embeddings --dimension=384 --distance=cosine --hnsw

# Insert a vector
veclite insert data.veclite embeddings --vector='[0.1,0.2,0.3,...]' --payload='{"file":"main.go"}'

# Batch insert from JSON file
veclite batch-insert data.veclite embeddings --input=vectors.json

# Search for similar vectors
veclite search data.veclite embeddings --query='[0.1,0.2,0.3,...]' --top-k=10
veclite search data.veclite embeddings --query='[0.1,0.2,0.3,...]' --filter='type=code'

# Get a specific vector
veclite get data.veclite embeddings --id=42

# Delete a vector
veclite delete data.veclite embeddings --id=42

# Drop a collection
veclite drop-collection data.veclite embeddings

# Start HTTP server
veclite serve data.veclite --port=8080

# Validate database integrity
veclite validate data.veclite

# Compact database
veclite compact data.veclite

# Run benchmark
veclite benchmark data.veclite --collection=embeddings --queries=1000
```

### Batch Insert File Format

The `batch-insert` command supports two input formats:

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

### Output Examples

**info command:**
```
Database: data.veclite
Collections: 2
Total Records: 15000
```

**collections command:**
```
embeddings: 10000 records, dimension=384, distance=cosine, index=hnsw
images: 5000 records, dimension=512, distance=euclidean, index=none
```

**search command (JSON):**
```json
[
  {"id": 42, "score": 0.9821, "payload": {"file": "main.go"}},
  {"id": 17, "score": 0.9654, "payload": {"file": "util.go"}}
]
```

**stats command (JSON):**
```json
{
  "path": "data.veclite",
  "collections": 2,
  "total_records": 15000,
  "collection_stats": [
    {
      "name": "embeddings",
      "count": 10000,
      "dimension": 384,
      "distance_type": "cosine",
      "index_type": "hnsw"
    }
  ]
}
```

## HTTP Server Mode

VecLite can run as an HTTP server for multi-language client access.

```bash
veclite serve data.veclite --port=8080 --cors
```

### REST API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/info` | Database info |
| GET | `/collections` | List collections |
| POST | `/collections` | Create collection |
| GET | `/collections/{name}` | Collection info |
| DELETE | `/collections/{name}` | Drop collection |
| POST | `/collections/{name}/vectors` | Insert vector(s) |
| GET | `/collections/{name}/vectors/{id}` | Get vector by ID |
| DELETE | `/collections/{name}/vectors/{id}` | Delete vector |
| POST | `/collections/{name}/search` | Search vectors |
| POST | `/sync` | Force sync to disk |

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

### Filter Operators

| Operator | Description |
|----------|-------------|
| `eq` or `=` | Equal |
| `neq` or `!=` | Not equal |
| `gt` or `>` | Greater than (numeric) |
| `gte` or `>=` | Greater than or equal (numeric) |
| `lt` or `<` | Less than (numeric) |
| `lte` or `<=` | Less than or equal (numeric) |
| `glob` | Glob pattern match |
| `prefix` | String prefix |
| `suffix` | String suffix |
| `contains` | String contains |
| `exists` | Key exists |

### Python Client Example

```python
import requests

base_url = "http://localhost:8080"

# Create collection
requests.post(f"{base_url}/collections", json={
    "name": "embeddings",
    "dimension": 384,
    "hnsw": True
})

# Insert vector
response = requests.post(f"{base_url}/collections/embeddings/vectors", json={
    "vector": [0.1] * 384,
    "payload": {"file": "main.py"}
})
print(response.json())  # {"status": "inserted", "id": 1}

# Search
response = requests.post(f"{base_url}/collections/embeddings/search", json={
    "query": [0.1] * 384,
    "top_k": 5
})
print(response.json())  # {"results": [...], "count": 5}
```

## Library API

### Opening a Database

```go
// File-based (persistent)
db, err := veclite.Open("vectors.veclite")

// In-memory (testing)
db, err := veclite.Open(":memory:")

// With options
db, err := veclite.Open("vectors.veclite",
    veclite.WithSyncOnWrite(true),  // Sync after each write
    veclite.WithReadOnly(true),     // Read-only mode
)

defer db.Close()
```

### Collections

```go
// Get or create (simple)
coll := db.Collection("embeddings")

// Create with options
coll, err := db.CreateCollection("embeddings",
    veclite.WithDimension(384),                    // Fixed dimension
    veclite.WithDistanceType(veclite.DistanceCosine), // Distance metric
    veclite.WithHNSW(16, 200),                     // Enable HNSW index
)

// Get existing
coll, err := db.GetCollection("embeddings")

// List all
names := db.Collections()

// Delete
err := db.DropCollection("embeddings")
```

### Distance Metrics

| Metric | Constant | Best Score |
|--------|----------|------------|
| Cosine Similarity | `DistanceCosine` | Higher = more similar |
| Dot Product | `DistanceDot` | Higher = more similar |
| Euclidean | `DistanceEuclidean` | Lower = more similar |

### Inserting Vectors

```go
// Insert with auto-generated ID
id, err := coll.Insert(vector, map[string]any{
    "file": "main.go",
    "type": "code",
})

// Batch insert
vectors := [][]float32{v1, v2, v3}
payloads := []map[string]any{p1, p2, p3}
ids, err := coll.InsertBatch(vectors, payloads)
```

### Upserting (Insert or Update)

```go
// Upsert by ID (0 = auto-generate new ID)
id, err := coll.Upsert(0, vector, payload)           // Insert new
id, err := coll.Upsert(42, vector, payload)          // Update if exists, insert with ID 42 if not

// Upsert by key field (useful for incremental indexing)
id, wasInsert, err := coll.UpsertByKey("file", "main.go", vector, map[string]any{
    "file": "main.go",
    "line": 100,
})
// wasInsert is true if new record was created, false if existing was updated

// Update only the vector (keep existing payload)
err := coll.UpdateVector(id, newVector)

// Update only the payload (keep existing vector)
err := coll.Update(id, newPayload)
```

### Searching

```go
// Basic search
results, err := coll.Search(queryVector, veclite.TopK(10))

// With threshold
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

### Filtering

Filter results by metadata fields:

```go
// Equal
results, _ := coll.Search(query,
    veclite.TopK(10),
    veclite.WithFilter(veclite.Equal("type", "code")),
)

// Multiple filters (AND logic)
results, _ := coll.Search(query,
    veclite.TopK(10),
    veclite.WithFilters(
        veclite.Equal("language", "go"),
        veclite.Prefix("file", "src/"),
    ),
)
```

**Available filters:**
- `Equal(key, value)` - Exact match
- `NotEqual(key, value)` - Not equal
- `In(key, values...)` - Value in list
- `NotIn(key, values...)` - Value not in list
- `Glob(key, pattern)` - Glob pattern match
- `Prefix(key, prefix)` - String prefix
- `Suffix(key, suffix)` - String suffix
- `Contains(key, substr)` - String contains
- `Exists(key)` - Key exists in payload
- `And(filters...)` - Combine filters with AND
- `Or(filters...)` - Combine filters with OR
- `Not(filter)` - Negate a filter

**Range filters (numeric):**
- `GreaterThan(key, value)` or `GT(key, value)` - Greater than
- `GreaterThanOrEqual(key, value)` or `GTE(key, value)` - Greater than or equal
- `LessThan(key, value)` or `LT(key, value)` - Less than
- `LessThanOrEqual(key, value)` or `LTE(key, value)` - Less than or equal
- `Between(key, min, max)` - Value in range (inclusive)

```go
// Range filter examples
results, _ := coll.Search(query,
    veclite.TopK(10),
    veclite.WithFilters(
        veclite.GT("score", 0.5),          // score > 0.5
        veclite.Between("line", 100, 500), // 100 <= line <= 500
    ),
)
```

### HNSW Configuration

```go
// Basic HNSW
coll, _ := db.CreateCollection("vectors",
    veclite.WithHNSW(16, 200),  // M=16, efConstruction=200
)

// Custom configuration
coll, _ := db.CreateCollection("vectors",
    veclite.WithHNSWConfig(veclite.HNSWConfig{
        M:              32,   // More connections = better recall, more memory
        EfConstruction: 400,  // Higher = better index quality, slower build
        EfSearch:       100,  // Default search quality
    }),
)
```

**Parameter Guidelines:**

| Parameter | Default | Range | Trade-off |
|-----------|---------|-------|-----------|
| M | 16 | 12-48 | Higher = better recall, more memory |
| efConstruction | 200 | 100-500 | Higher = better index, slower build |
| efSearch | 100 | 50-500 | Higher = better recall, slower search |

### Deleting Records

```go
// Delete by ID
err := coll.Delete(42)

// Delete by filter
count, err := coll.DeleteWhere(veclite.Equal("type", "temp"))
```

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

### Debug Search

```go
// Get search explanation
explanation, err := coll.SearchExplain(query, veclite.TopK(10))
fmt.Printf("Index: %s, Nodes Visited: %d, Duration: %v\n",
    explanation.IndexType,
    explanation.NodesVisited,
    explanation.Duration,
)
```

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
- Writes are serialized with proper locking
- Use `WithSyncOnWrite(true)` for durability after each write

## Persistence

Data is stored using Go's gob encoding:
- Call `db.Sync()` to persist changes manually
- Use `WithSyncOnWrite(true)` for automatic persistence
- `db.Close()` syncs before closing

## Contributing

Contributions welcome. Please:
1. Run `go test -race ./...` before submitting
2. Add tests for new features
3. Follow existing code style

## License

MIT License - see [LICENSE](LICENSE)
