# ADR-0001: Embedding Boundary and Named Vector Spaces

## Status

Accepted — **Implemented**. The additive named-vector-space and embedding-profile API described
under "API Direction" now ships (storage format v4). See the
[Named Vector Spaces](/guide/named-vector-spaces) guide for the delivered Go, CLI, and HTTP surface.
This ADR is retained as the historical design record: the "API Direction" section below is the
original design sketch and is intentionally left in its future tense. The shapes it proposes shipped
as `VectorSpaceConfig`, `RecordInput`, `EmbeddingProfile`, `SearchSpace`, and `MultiSpaceSearch` —
see the [Named Vector Spaces](/guide/named-vector-spaces) guide for the delivered API.

## Context

VecLite is used as an embeddable vector database by applications with different domains. `vecgrep` indexes source code chunks for semantic code search. `vidtrace` plans to index bug-video evidence built from timeline entries, OCR text, transcript text, and frame paths.

Those applications need different preprocessing and may use different embedding models. Code, OCR text, transcripts, screenshots, and video frames do not share one universal extraction pipeline. If VecLite owns chunking, frame extraction, OCR, or provider selection, the database becomes coupled to application behavior and media tooling.

Current VecLite records store one vector per record in `Record.Vector`, and collections have one dimension, distance metric, and optional HNSW index. That works well for a single embedding type per collection. It does not directly represent a record with multiple embeddings, such as text and image vectors for the same video timestamp.

## Decision Drivers

- Keep VecLite easy to import from Go as a local-first storage and search library.
- Keep app-specific extraction and preprocessing in the app that understands the domain.
- Support future multimodal search without forcing apps to split one logical record across many unrelated records.
- Preserve backward compatibility with current single-vector APIs and database snapshots.

## Considered Options

1. Apps use separate collections for every embedding type forever.
2. VecLite owns embedding providers, extraction, and chunking for all app domains.
3. VecLite adds named vector spaces while apps continue to own embedding generation.

## Decision Outcome

Chosen option: **VecLite adds named vector spaces while apps continue to own embedding generation**.

VecLite owns:

- durable record, payload, content, and vector storage
- collection configuration and compatibility checks
- distance functions, HNSW indexes, BM25 indexes, filters, pagination, and hybrid fusion
- persistence and migration of vector/index metadata

Applications own:

- source discovery, code chunking, media extraction, OCR, transcripts, and frame selection
- embedding provider configuration, batching, retries, rate limits, and credentials
- choosing which fields become content, payload, and vectors
- deciding when an index must be rebuilt after provider/model/chunker changes

## API Direction

Current releases keep the single-vector model:

- `Record.Vector` is the default vector.
- `Collection.Insert`, `InsertDocument`, `Search`, `HybridSearch`, and `UpdateVector` keep their current behavior.
- Apps that need multiple embedding types today should use separate collections or separate databases and store an explicit embedding profile with the application index.

Future VecLite versions should add an additive named-vector model:

```go
type VectorSpaceConfig struct {
    Name      string
    Dimension int
    Distance  DistanceType
    Modality  string
    Provider  string
    Model     string
    HNSW      *HNSWConfig
}

type RecordInput struct {
    Content string
    Payload map[string]any
    Vectors map[string][]float32
}
```

The future API should allow:

- declaring multiple vector spaces on a collection
- inserting one logical record with vectors such as `text`, `frame_clip`, or `audio`
- searching one named vector space at a time
- fusing BM25 and one or more vector result sets
- loading old single-vector snapshots into a `default` vector space

## Consequences

**Good:**

- VecLite remains a general-purpose Go library instead of becoming a code-search or video-analysis framework.
- vecgrep can keep its provider and chunking code without waiting on VecLite feature work.
- vidtrace can start with BM25 evidence search, then add semantic text and image vectors when useful.
- Named vector spaces give multimodal records a natural future shape.

**Bad:**

- Apps must still decide when provider, model, dimension, or preprocessing changes require a rebuild.
- Named vector spaces require a storage migration and one HNSW index per configured vector space.
- API design must preserve existing `Record.Vector` behavior, which adds compatibility complexity.

## Follow-up Work

- ~~Add a public embedding guide~~ — **done** (`/embeddings` and `/guide/named-vector-spaces`).
- ~~Use collection or database metadata for embedding-profile information~~ — **superseded**: `EmbeddingProfile` is now a first-class, persisted type (the metadata convention still works).
- ~~Use text-only records for BM25-first applications~~ — **done** (`InsertTextDocument`).
- ~~Implement named vector spaces in a storage-versioned release~~ — **done** in storage format v4: `VectorSpaceConfig`, `RecordInput`, `AddVectorSpace`, `InsertRecord`, `SearchSpace`, `MultiSpaceSearch`, and the public `FuseRRF`, all additive over the existing single-vector API.

### Remaining

- Language drivers (Python, TypeScript, …) over the CLI/HTTP JSON contract — planned, not started.
