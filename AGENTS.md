# AGENTS.md - AI Agent Instructions for VecLite

> Guidelines for AI agents working on the VecLite codebase.

## Project Overview

VecLite is an embeddable vector database for Go with zero external dependencies. It provides:
- Single-file persistence using gob encoding
- HNSW index for fast approximate nearest neighbor search
- In-memory mode for testing
- CLI tool for database operations

**Repository:** `github.com/abdul-hamid-achik/veclite`
**Go Version:** 1.21+
**Zero Dependencies:** Standard library only (no external modules)

## Project Structure

```
veclite/
├── veclite.go          # DB struct, Open/Close, Collection management
├── collection.go       # Collection struct, Insert/Delete/Search
├── search.go           # Search options and configuration
├── record.go           # Record struct with ID, Vector, Metadata
├── filter.go           # Metadata filtering (Equal, In, Glob, Prefix, etc.)
├── options.go          # Functional options (WithHNSW, WithDimension, etc.)
├── index.go            # Index interface definition
├── index_hnsw.go       # HNSW wrapper implementing Index interface
├── explain.go          # SearchExplain for debugging
├── errors.go           # Error types
├── storage.go          # Storage interface and snapshots
├── storage_file.go     # File-based persistence
├── storage_memory.go   # In-memory storage
├── internal/
│   ├── floats/         # Distance functions (cosine, euclidean, dot)
│   └── hnsw/           # HNSW implementation
│       ├── hnsw.go     # Core index structure and insert
│       ├── search.go   # Search algorithm
│       ├── node.go     # Node with neighbors per layer
│       ├── heap.go     # Priority queues for search
│       ├── config.go   # HNSW parameters
│       ├── delete.go   # Soft delete and compaction
│       ├── serialize.go # Snapshot/restore
│       └── errors.go   # HNSW-specific errors
└── cmd/veclite/        # CLI application
    ├── main.go         # CLI entry point, read/write commands
    ├── server.go       # HTTP server mode (serve command)
    └── maintenance.go  # compact, validate, benchmark commands
```

## Key Concepts

### Distance Metrics
- **Cosine** (default): Similarity metric, higher is better
- **Euclidean**: Distance metric, lower is better
- **Dot Product**: Similarity metric, higher is better

The `higherBetter` flag controls sort order and comparison logic throughout the codebase.

### HNSW Index
- Hierarchical Navigable Small World graph
- Parameters: M (connections), efConstruction (build quality), efSearch (query quality)
- Soft delete with tombstones, periodic compaction available

### Collections
- Named containers for vectors
- Optional HNSW index (defaults to brute-force if not specified)
- Metadata filtering on search

## Development Commands

```bash
# Using Task (recommended)
task test          # Run tests
task test-race     # Run tests with race detection
task lint          # Run go vet
task bench         # Run benchmarks
task build         # Build CLI to bin/veclite
task check         # Run fmt, lint, test

# Using Go directly
go test -v ./...
go test -race -v ./...
go test -bench=. -benchmem ./...
go build -o bin/veclite ./cmd/veclite
```

## Testing Requirements

1. **Always run tests with race detection** before committing:
   ```bash
   go test -race ./...
   ```

2. **HNSW recall tests** verify search quality:
   ```bash
   go test -run TestHNSW -v ./internal/hnsw/
   ```

3. **Benchmarks** for performance verification:
   ```bash
   go test -bench=BenchmarkSearch -benchmem ./...
   ```

## Code Style Guidelines

1. **Zero dependencies**: Do not add external modules. Use standard library only.

2. **Error handling**: Return descriptive errors using types in `errors.go`:
   ```go
   return &DimensionError{Expected: idx.dimension, Got: len(vector)}
   ```

3. **Concurrency**: Use `sync.RWMutex` for thread safety. Read operations use `RLock()`.

4. **Distance handling**: Always check `higherBetter` when comparing distances:
   ```go
   if idx.higherBetter {
       if dist > threshold { /* better */ }
   } else {
       if dist < threshold { /* better */ }
   }
   ```

5. **Copy vectors**: Always copy vectors to prevent external mutation:
   ```go
   vec := make([]float32, len(vector))
   copy(vec, vector)
   ```

## Common Tasks

### Adding a New Distance Metric
1. Add constant to `internal/floats/distance.go`
2. Implement function and add to `GetDistanceFunc`
3. Update `IsHigherBetter` if needed
4. Add tests

### Modifying HNSW
1. Changes go in `internal/hnsw/`
2. Update `serialize.go` if struct fields change
3. Run recall tests to verify search quality
4. Run benchmarks to check performance impact

### Adding Collection Options
1. Add option function to `options.go`
2. Add field to `collectionConfig` struct
3. Apply in `newCollection` or relevant method
4. Update snapshot if persistence needed

### Adding CLI Commands
1. Add command case to `switch` in `cmd/veclite/main.go`
2. Implement `cmd<CommandName>` function in appropriate file:
   - Read/write commands → `main.go`
   - Server-related → `server.go`
   - Maintenance (compact, validate, benchmark) → `maintenance.go`
3. Add flag parsing with `flag.NewFlagSet`
4. Support `--json` flag for JSON output (use `outputJSON()` helper)
5. Update `printUsage()` with new command
6. Add documentation to README.md

### Extending HTTP API
1. Add handler method to `Server` struct in `cmd/veclite/server.go`
2. Add route in `cmdServe()` function's mux setup
3. Use `writeJSON()` for success responses and `writeError()` for errors
4. Follow RESTful conventions (GET for reads, POST for creates, DELETE for removes)
5. Parse request body with `json.NewDecoder(r.Body).Decode()`
6. Update README.md API documentation

## CI Pipeline

The GitHub Actions CI runs on every push/PR:
1. **Security Scan**: govulncheck
2. **Lint**: golangci-lint
3. **Test**: go test with race detection
4. **Build**: Verify compilation and CLI

All checks must pass before merging.

## Versioning

- Library version is in `veclite.go` as `const Version`
- CLI version is injected via ldflags at build time
- Follow semantic versioning (major.minor.patch)

## Important Files to Understand

| File | Purpose |
|------|---------|
| `collection.go` | Core API - Insert, Delete, Search |
| `internal/hnsw/hnsw.go` | HNSW insert algorithm |
| `internal/hnsw/search.go` | HNSW search algorithm |
| `internal/hnsw/heap.go` | CandidateSet for search traversal |
| `storage.go` | Snapshot structures for persistence |

## Debugging Tips

1. Use `SearchExplain` to understand search behavior:
   ```go
   explanation, _ := coll.SearchExplain(query, TopK(10))
   fmt.Printf("Index: %s, Visited: %d\n", explanation.IndexType, explanation.NodesVisited)
   ```

2. Check index stats:
   ```go
   stats := coll.Stats()
   fmt.Printf("Type: %s, Count: %d\n", stats.IndexType, stats.Count)
   ```

3. For HNSW issues, compare with brute force:
   ```go
   hnswResults, _ := idx.Search(query, 10)
   bruteResults, _ := idx.KNNBruteForce(query, 10)
   ```
