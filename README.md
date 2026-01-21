# VecLite

Embeddable vector database for Go with zero external dependencies.

Store vectors with metadata in a single file. Search with HNSW for fast approximate nearest neighbors.

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
        fmt.Printf("ID: %d, Score: %.4f\n", r.ID, r.Distance)
    }
}
```

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

The `veclite` CLI inspects and manages VecLite database files.

```bash
veclite <command> [arguments]
```

### Commands

| Command | Description |
|---------|-------------|
| `version` | Show version information |
| `info <file>` | Show database summary |
| `collections <file>` | List all collections |
| `stats <file>` | Show detailed statistics |
| `dump <file>` | Export database as JSON |

### Examples

```bash
# Check version
veclite version

# View database info
veclite info data.veclite

# List collections with record counts
veclite collections data.veclite

# Get detailed stats (JSON output)
veclite stats --json data.veclite

# Export specific collection
veclite dump --collection embeddings data.veclite

# Export with record limit
veclite dump --limit 100 data.veclite
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
embeddings: 10000 records, dimension=384, distance=cosine
images: 5000 records, dimension=512, distance=euclidean
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

// Insert with specific ID
err := coll.InsertWithID(42, vector, payload)

// Batch insert
vectors := [][]float32{v1, v2, v3}
payloads := []map[string]any{p1, p2, p3}
ids, err := coll.InsertBatch(vectors, payloads)
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
    fmt.Printf("ID: %d, Distance: %.4f, Payload: %v\n",
        r.ID, r.Distance, r.Payload)
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
        veclite.GreaterThan("score", 0.5),
    ),
)
```

**Available filters:**
- `Equal(key, value)` - Exact match
- `NotEqual(key, value)` - Not equal
- `In(key, values...)` - Value in list
- `NotIn(key, values...)` - Value not in list
- `GreaterThan(key, value)` - Numeric comparison
- `GreaterThanOrEqual(key, value)`
- `LessThan(key, value)`
- `LessThanOrEqual(key, value)`
- `Contains(key, substr)` - String contains
- `HasPrefix(key, prefix)` - String prefix
- `HasSuffix(key, suffix)` - String suffix
- `Exists(key)` - Key exists in payload
- `NotExists(key)` - Key does not exist
- `MatchGlob(key, pattern)` - Glob pattern match

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
