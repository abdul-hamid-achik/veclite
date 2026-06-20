# Project Status

This page records the current implementation state and the next work that matters for VecLite as a shared Go library.

## Current Release

`v0.15.1` is published at:

https://github.com/abdul-hamid-achik/veclite/releases/tag/v0.15.1

The release includes the VitePress documentation site, Bun-based docs tooling, database and collection metadata, text document storage, BM25-first text records, hybrid search, and embedding-boundary documentation.

## Current Design Boundary

VecLite should own durable local search primitives:

- record, vector, content, payload, and metadata persistence
- dimension and distance compatibility checks
- HNSW indexes and brute-force fallback
- BM25 text indexes
- metadata filters, pagination, iteration, and hybrid result fusion
- storage-versioned migrations

Applications should own domain-specific extraction and embedding pipelines:

- file walking and source-code chunking
- OCR, transcript parsing, frame extraction, and media preprocessing
- embedding provider credentials, batching, retries, and model rollout
- deciding which fields become content, payload, and vectors
- deciding when provider or preprocessing changes require a rebuild

## What Works Now

**Named vector spaces** let one logical record carry several embeddings (e.g. `text` + `image`),
each with its own dimension, distance metric, and HNSW index. Declare them with `AddVectorSpace` /
`WithVectorSpace`, insert with `InsertRecord`, search one space with `SearchSpace`, and fuse
across spaces with `MultiSpaceSearch` (or the public `FuseRRF`). The default space stays fully
backward compatible. See the [Named Vector Spaces](/guide/named-vector-spaces) guide.

**Embedding profiles** are a first-class type (`EmbeddingProfile`) attachable to a collection or a
space; they validate inserts and expose `Compatible` for detecting index-invalidating changes.

Use one collection with the default space when records share one embedding profile. Use **named
spaces** when records are the same logical items with multiple embeddings, and separate collections
only when records are genuinely unrelated.

Text-only records are supported through `InsertTextDocument` and `InsertTextDocumentWithOptions`.
They are indexed by BM25, returned by filters and iteration, and skipped by vector search until an
application adds vectors.

The CLI (`space-add`, `spaces`, `record-insert`, `search-space`, `fuse-search`) and HTTP server
expose the same operations as JSON — the cross-language contract that future language drivers will
build on.

## Related Projects

`vecgrep` should use VecLite for durable code chunk storage, vector search, text search, filters, and hybrid ranking. It should keep file discovery, chunking, provider setup, and index rebuild policy in `vecgrep`.

`vidtrace` should start with text-only evidence records for timeline, OCR, and transcript entries.
It can add semantic text embeddings in the default space and frame/image embeddings in a named
`frame` space on the **same** records, then fuse them with `MultiSpaceSearch`.

## Recently Shipped

- **Named vector spaces** — multiple independent embeddings per record (additive, backward compatible).
- **Storage migration to format v4** — pre-v4 databases open as a default-space-only collection with no rewrite.
- **First-class embedding profiles** — `EmbeddingProfile` with dimension validation and `Compatible` checks.
- **Multi-space result fusion** — `MultiSpaceSearch` and the public `FuseRRF` API.
- **CLI + HTTP named-space surface** — `space-add`, `spaces`, `record-insert`, `search-space`, `fuse-search`, mirrored over HTTP.
- **Glyphrun behavior specs** under `specs/glyphrun/` pin the CLI contract.
- **CI hygiene** — upstream actions bumped off the deprecated Node runtime; GoReleaser pinned to `~> v2`.

## Remaining Work

- Hosted docs deployment from this repo (a GitHub Pages workflow alongside the existing Vercel config).
- Language drivers (Python, TypeScript, …) over the CLI/HTTP contract — planned, not started.
- Cross-space fusion ergonomics (weighted multi-space + BM25 in one call) if app usage warrants it.

## Next Steps

1. Adopt named vector spaces from `vidtrace` for multimodal evidence (text default space + `frame` space).
2. Cut a minor release that advertises named vector spaces and the v4 format.
3. Start the first language driver against the CLI/HTTP JSON contract once the surface settles.
