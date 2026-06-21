# AGENTS.md - AI Agent Instructions for VecLite

> Guidelines for AI agents working on the VecLite codebase.

## Project Overview

VecLite is an embeddable vector database for Go. It provides:
- Single-file persistence using gob encoding
- HNSW index for fast approximate nearest neighbor search
- Named vector spaces: multiple independent embeddings per record (e.g. `text` + `image`)
- BM25 text indexing, hybrid search, and Reciprocal Rank Fusion
- In-memory mode for testing
- CLI tool and HTTP server for database operations

**Repository:** `github.com/abdul-hamid-achik/veclite`
**Go Version:** 1.23+
**Dependency Discipline:** Prefer the standard library for core database behavior. Optional integrations may use focused external modules.

### Not a Go-only library

VecLite can be imported directly as a Go library, but it is **not** intended to be Go-only.
The CLI (`cmd/veclite`) and HTTP server (`veclite serve`) are stable, JSON-in/JSON-out
surfaces meant to be the contract that language drivers (Python, TypeScript, etc.) will build
on. Drivers are **not** being written yet, but when changing the CLI or HTTP API, treat their
JSON shapes as a public, cross-language contract: keep them additive and stable, and cover
them with `specs/glyphrun/` behavior specs so any driver can be validated against the same
expectations.

## Project Structure

```
veclite/
├── veclite.go          # DB struct, Open/Close, Collection management, snapshot migration
├── collection.go       # Collection struct, Insert/Delete/Search, snapshot/load
├── collection_spaces.go# Named vector space API (AddVectorSpace, InsertRecord, SearchSpace, MultiSpaceSearch)
├── vectorspace.go      # VectorSpaceConfig, RecordInput, EmbeddingProfile, internal vectorSpace
├── search.go           # Search options and configuration
├── fusion.go           # Reciprocal Rank Fusion (internal + public FuseRRF)
├── record.go           # Record struct with ID, Vector, Vectors (named), Metadata
├── filter.go           # Metadata filtering (Equal, In, Glob, Prefix, etc.)
├── options.go          # Functional options (WithHNSW, WithDimension, WithVectorSpace, ...)
├── index.go            # Index interface definition
├── index_hnsw.go       # HNSW wrapper implementing Index interface
├── explain.go          # SearchExplain for debugging
├── errors.go           # Error types
├── storage.go          # Storage interface alias + snapshot type re-exports
│                        #   (CurrentVersion + Migrate live in internal/storage/storage.go)
├── storage_file.go     # File-based persistence
├── storage_memory.go   # In-memory storage
├── config.go           # YAML config loading (Config, EmbedderConfig, LoadConfig)
├── embedder.go         # Embedder interface
├── embedder_factory.go # NewEmbedderFromConfig (default build)
├── embedder_factory_onnx.go # NewEmbedderFromConfig (onnx build tag)
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
├── embed/              # Embedder provider implementations (build-tag isolated)
│   ├── common/         # Shared HTTP/normalize helpers
│   ├── ollama/         # Ollama embedder
│   ├── onnx/           # ONNX embedder (onnx build tag)
│   └── openai/         # OpenAI embedder
└── cmd/veclite/        # CLI application
    ├── main.go         # CLI entry point, read/write commands
    ├── spaces.go       # Named-space CLI (space-add, spaces, record-insert, search-space, fuse-search)
    ├── server.go       # HTTP server mode (serve command)
    ├── server_spaces.go# HTTP handlers for named vector spaces
    ├── mcp.go          # MCP tool server
    └── maintenance.go  # compact, validate, benchmark commands
```

> Note: the public API files live at the module root in `package veclite`; the
> `storage_file.go`/`storage_memory.go` names above map to `internal/storage/file.go`
> and `internal/storage/memory.go`.

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

### Named Vector Spaces
- A collection has one implicit **`default`** space (backed by `Record.Vector` and the
  collection's primary dimension/distance/index) plus zero or more **named** spaces declared
  with `AddVectorSpace` / `WithVectorSpace`.
- Each space has its own dimension, distance metric, and optional HNSW index. A named space's
  vectors live in `Record.Vectors[name]`; the index uses a vector provider that reads them.
- `InsertRecord(RecordInput{...})` inserts one logical record carrying vectors in several
  spaces at once. `SearchSpace(space, query, ...)` searches one space; `MultiSpaceSearch`
  fuses several spaces with RRF (or use the public `FuseRRF`).
- **Backward compatible & additive:** the entire pre-existing single-vector API
  (`Insert`, `Search`, `HybridSearch`, `UpdateVector`, ...) is unchanged and operates on the
  default space. Old snapshots load as a collection with only the default space.
- Reserved name `DefaultVectorSpace` (`"default"`) and the empty string both target the
  default space; they cannot be redeclared with `AddVectorSpace`.

### Embedding Profiles
- `EmbeddingProfile` (provider, model, dimension, distance, normalize, version) is a
  first-class type, not just a metadata convention. Attach it via `WithEmbeddingProfile` /
  `SetEmbeddingProfile` (collection default space) or `VectorSpaceConfig.Profile` (per space).
- A set profile validates inserted vectors against its dimension. `EmbeddingProfile.Compatible`
  reports whether two profiles describe interchangeable vectors (used to detect when a
  provider/model/distance change invalidates an index).
- Profiles persist in the snapshot. The historical "store a profile in metadata" convention
  still works for callers that prefer it.

### Embedding and Modality Boundary
- VecLite owns durable vector, text, payload, index, filter, and search primitives — now
  including named vector spaces and embedding profiles.
- Applications own domain extraction, chunking, OCR, transcript parsing, frame selection,
  provider credentials, embedding generation, and rebuild policy.
- Use named vector spaces for multimodal records (e.g. `text` + `image` for one item). Use
  separate collections only when the records are genuinely unrelated.
- Use text-only records (`InsertTextDocument`) for BM25-first workflows; vector search skips
  records without a vector in the queried space.

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
2. Preserve the existing single-vector API; the default space must stay backward compatible.
3. New persisted fields go on `storage.VectorSpaceSnapshot`/`RecordSnapshot`/`CollectionSnapshot`.
   When the on-disk format changes, bump `fileVersion` in `internal/storage/file.go`, add a
   case to `migrateSnapshot`, and bump `storage.CurrentVersion`. The v3→v4 named-vector-space
   migration is the reference: gob tolerates added fields, so migrations are additive and never
   rewrite record data.
4. Old `Record.Vector` data maps into the implicit `default` space with no transformation.
5. Add tests for dimension mismatches, profile compatibility, persistence round-trips, per-space
   search, multi-space fusion, update, delete, and the v3→v4 migration (see `vectorspace_test.go`).
6. Mirror new behavior on the CLI (`cmd/veclite/spaces.go`) and HTTP server
   (`cmd/veclite/server_spaces.go`), and add a `specs/glyphrun/` behavior spec — these are the
   cross-language contract.

### Adding CLI Commands
1. Add command case to `switch` in `cmd/veclite/main.go`
2. Implement `cmd<CommandName>` function in appropriate file:
   - Read/write commands → `main.go`
   - Named vector spaces → `spaces.go`
   - Server-related → `server.go` / `server_spaces.go`
   - Maintenance (compact, validate, benchmark) → `maintenance.go`
3. Add flag parsing with `flag.NewFlagSet` and call `fs.Parse(args)`. `main()` already reorders
   each command's args through `hoistFlags` before dispatch, so flags work in any position
   (Go's `flag` otherwise stops at the first non-flag token). Don't hoist again inside the command.
4. Support `--json` flag for JSON output (use the `encodeJSON()` helper in `spaces.go`)
5. Update `printUsage()` with the new command
6. Add documentation to README.md and a `specs/glyphrun/` behavior spec

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

## Documentation Site and Deployment

The documentation lives in `docs/` and is a **VitePress** site (Bun-based tooling). This is the
**public website** — it is *only* for published user-facing documentation, not for working notes,
handoffs, TODOs, or scratch content.

- **Build/preview locally:** `task site`, `task site-dev`, `task site-preview` (or `bun run site*`).
- **Hosting — already solved, do not duplicate:** the site is **deployed to Vercel** via
  `vercel.json` at the repo root; the linked `.vercel` project (gitignored) auto-deploys on every
  push to `main`. It is served at the **domain root**, so the VitePress `base` is the default `/`.
- **Do NOT add a second deploy path** (no GitHub Pages workflow, no `gh-pages`, no `DOCS_BASE`
  base-path juggling). A previous attempt added a redundant GitHub Pages workflow; it was removed.
  If hosting ever needs to change, change/replace the Vercel config — keep exactly one deploy target.
- When you add a docs page, wire it into `docs/.vitepress/config.mts` (`nav` + `sidebar`). Static
  assets (icons, images) go in `docs/.vitepress/public/` and are referenced with root-absolute
  paths (e.g. `/logo.svg`).

> General rule this encodes: before adding infrastructure (deploy, CI, tooling), check what the repo
> already does — `vercel.json`, `.vercel`, `glyphrun.config.yml`, `Taskfile.yml`, `.goreleaser.yml`,
> `.github/workflows/` — and extend the existing mechanism instead of introducing a parallel one.

## Working Notes and the Obsidian Vault

**Do not put notes, handoffs, journals, or scratch content in the repo** (not in `docs/`, not in a
`NOTES.md`, not in stray `.md` files). The `docs/` folder is the VitePress website only — polluting it
mixes working state with published docs and can ship internal notes to the public site on push.

Notes live in the **Obsidian vault** at `~/notes`, and per-project notes go in
`~/notes/projects/<project>/`. The vault already has a `veclite/` project folder (with an `index.md`
and release handoffs) and a sibling `vecgrep/` folder.

Use the **`obsidian-cli` skill** to interact with the vault from the terminal — load it with the
skill tool (`obsidian-cli`) and follow its instructions. Typical operations:

```bash
# Create a project note (extension auto-added)
obsidian create path="projects/veclite/<Note Title>" content="..."
# Append to an existing note
obsidian append path="projects/veclite/index.md" content="- new bullet"
# Add frontmatter
obsidian property:set path="projects/veclite/<Note>.md" name="project" value="veclite"
# Search the vault
obsidian search query="named vector spaces" path="projects"
```

### What goes where

| Content | Location |
|---------|----------|
| Published user docs (guides, API reference, ADRs) | `docs/` (VitePress site) |
| README-level usage / CLI reference | `README.md` |
| Release handoffs, session journals, design scratch, TODOs | `~/notes/projects/veclite/` (Obsidian) |
| Cross-project context (e.g. how vecgrep uses veclite) | `~/notes/projects/<project>/` (Obsidian) |

When you finish a chunk of work, write a dated note in `~/notes/projects/veclite/` (or the relevant
sibling project folder) via the obsidian-cli skill instead of dropping a markdown file in the repo.

## Release and Deployment Summary

| Concern | Mechanism | Source of truth |
|---------|-----------|-----------------|
| Go module / library | `go get` | git tags (`vX.Y.Z`) |
| CLI binaries + Homebrew | GoReleaser on tag push | `.goreleaser.yml`, `.github/workflows/release.yml` |
| Documentation site | Vercel (VitePress) | `vercel.json` + linked `.vercel` project |
| CLI behavior contract | glyphrun specs | `specs/glyphrun/`, `glyphrun.config.yml` |

## Versioning

- Library version is in `veclite.go` as `const Version` (keep `package.json` `version` in sync for docs).
- CLI version is injected via ldflags at build time; GoReleaser sets it from the git tag.
- Follow semantic versioning (major.minor.patch). Cutting a release = bump `const Version`, commit,
  then push an annotated `vX.Y.Z` tag, which triggers `release.yml` (GoReleaser).

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
