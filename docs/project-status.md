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

Use one VecLite collection when records share one embedding profile: provider, model, dimensions, distance metric, modality, and preprocessor.

Use separate collections when embedding profiles differ. This is the current path for code search, video evidence search, and future multimodal search.

Text-only records are supported through `InsertTextDocument` and `InsertTextDocumentWithOptions`. They are indexed by BM25, returned by filters and iteration, and skipped by vector search until an application adds vectors through a later indexing workflow.

## Related Projects

`vecgrep` should use VecLite for durable code chunk storage, vector search, text search, filters, and hybrid ranking. It should keep file discovery, chunking, provider setup, and index rebuild policy in `vecgrep`.

`vidtrace` should start with text-only evidence records for timeline, OCR, and transcript entries. It can add semantic text embeddings later in a separate collection. Image/frame embeddings should remain separate until VecLite has named vector spaces.

## Missing Work

- Named vector spaces are not implemented.
- Storage migrations for multiple vector spaces are not implemented.
- Embedding-profile compatibility is a metadata convention, not a first-class API.
- Multi-space result fusion is not implemented.
- The docs site builds locally with `task site`, but this repo does not yet define hosted docs deployment.
- GitHub Actions has Node runtime deprecation warnings from upstream actions.
- The release workflow uses GoReleaser `latest`; pin it before release automation becomes stricter.

## Next Steps

1. Use `v0.15.1` from `vecgrep` and `vidtrace` with separate collections per embedding profile.
2. Add small profile helper APIs if app-side metadata comparison becomes repetitive.
3. Design named vector spaces as an additive API with a storage migration plan.
4. Add hosted docs deployment only after the local docs content stabilizes.
