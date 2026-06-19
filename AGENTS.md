# AGENTS.md - AI Agent Instructions for VecLite

> Guidelines for AI agents working on the VecLite codebase.

## Project Overview

VecLite is an embeddable vector database for Go. It provides:
- Single-file persistence using gob encoding
- HNSW index for fast approximate nearest neighbor search
- In-memory mode for testing
- CLI tool for database operations

**Repository:** `github.com/abdul-hamid-achik/veclite`
**Go Version:** 1.23+
**Dependency Discipline:** Prefer the standard library for core database behavior. Optional integrations may use focused external modules.

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

### Embedding and Modality Boundary
- VecLite owns durable vector, text, payload, index, filter, and search primitives.
- Applications own domain extraction, chunking, OCR, transcript parsing, frame selection, provider credentials, embedding generation, and rebuild policy.
- Current collections store one vector per record. Use separate collections for incompatible embedding types until named vector spaces are implemented.
- Store or validate an embedding profile in database or collection metadata when provider, model, dimensions, distance, or preprocessing changes can invalidate an index.
- Use text-only records for BM25-first workflows; vector search must skip records without vectors.

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

## Iterative Development Workflow

**Run linter and tests frequently in a loop** as you make changes. This is critical for validating your work:

```bash
# After every meaningful change, run this cycle:
task lint && task test

# Or using Go directly:
go vet ./... && go test -race ./...
```

### Why This Matters
- **Catch issues early**: Linter errors and test failures are easier to fix when caught immediately after the change that caused them
- **Maintain quality**: Frequent validation prevents accumulation of technical debt
- **Build confidence**: Passing tests confirm your changes work as intended

### The Validation Loop
1. Make a small, focused change
2. Run `task lint` to catch static analysis issues
3. Run `task test` to verify functionality
4. Fix any issues before moving on
5. Repeat

### Keep README.md Up to Date
The README.md is the primary documentation for users. **Update it frequently** as you make changes:

- **New features**: Document them immediately after implementation
- **API changes**: Update usage examples and API reference sections
- **CLI changes**: Update command documentation and examples
- **Configuration options**: Document new options as they're added
- **Breaking changes**: Clearly note what changed and migration steps

```bash
# After completing a feature, always check if README needs updates:
# 1. Does this change affect how users use the library?
# 2. Are there new public APIs?
# 3. Did CLI commands change?
# 4. Are examples still accurate?
```

Keeping documentation synchronized with code ensures the project remains useful and accessible to new users.

## Code Style Guidelines

1. **Dependency discipline**: Do not add external modules for core database behavior unless clearly necessary. Prefer the standard library and keep optional integrations isolated.

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

### Adding Embedding or Vector-Space Features
1. Keep app-specific extraction outside VecLite.
2. Preserve the existing single-vector API unless a breaking release is explicitly planned.
3. Add storage-versioned migrations for persisted vector-space metadata.
4. Document how old `Record.Vector` data maps into the default vector space.
5. Add tests for dimension mismatches, persistence, search, update, delete, and hybrid behavior.

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

## Concurrency and Lock Ordering

VecLite uses `sync.RWMutex` for both `Collection` and `HNSW.Index`. The lock ordering convention is:

1. **DB lock** (`db.mu`) — outermost
2. **Collection lock** (`coll.mu`) — middle
3. **HNSW Index lock** (`idx.mu`) — innermost
4. **KnowledgeGraph/EpisodeStore locks** — innermost within their scope

**Rules:**
- Never acquire a higher-level lock while holding a lower-level one
- Searches use `RLock()` on both Collection and HNSW — concurrent reads are safe
- Writes (Insert, Delete) take `Lock()` on Collection, which also takes `Lock()` on HNSW internally
- EpisodeStore and KnowledgeGraph hold their own `mu` locks and access Collection via `RLock()` — never hold EpisodeStore/Graph locks while acquiring Collection write lock
