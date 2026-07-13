---
title: Core Concepts
description: "Understand VecLite databases, collections, records, vector spaces, embedding profiles, indexes, and persistence before designing your application."
---

# Core Concepts

VecLite keeps the storage model small: a database contains collections, and a collection contains records. A record can carry text, payload data, and one or more vectors. Search returns the complete record, so your application does not need a second store to resolve vector IDs.

## Mental Model

```text
database file
└── collection: docs
    ├── configuration
    │   ├── default vector space (dimension, distance, optional HNSW)
    │   ├── optional named vector spaces
    │   └── optional BM25 payload fields
    └── records
        ├── ID
        ├── Content
        ├── Payload
        ├── Vector (default space)
        └── Vectors[space] (named spaces)
```

### Database

`veclite.Open` opens either a persistent snapshot or an in-memory database:

```go
fileDB, err := veclite.Open("search.veclite")
testDB, err := veclite.Open(":memory:")
```

A persistent database writes an atomic snapshot on `Sync()` or `Close()`. Enable the optional WAL when completed writes must survive a process or machine crash. See [Durability and the WAL](./durability).

### Collection

A collection groups records that share search configuration. Declare dimensions and indexes explicitly when you know them:

```go
docs, err := db.CreateCollection("docs",
    veclite.WithDimension(768),
    veclite.WithDistanceType(veclite.DistanceCosine),
    veclite.WithHNSW(16, 200),
    veclite.WithTextIndex("title", "path"),
)
```

`db.Collection("docs")` gets an existing collection or creates one with defaults. With the default configuration, VecLite detects the vector dimension on the first insert and uses brute-force search.

### Record

A record can contain any combination of:

| Field | Purpose |
|---|---|
| `ID` | Stable numeric identifier assigned by VecLite or supplied by the caller |
| `Content` | Source text indexed by BM25 when text indexing is enabled |
| `Payload` | Arbitrary JSON-like metadata used for filtering and BM25 payload fields |
| `Vector` | Embedding in the implicit `default` vector space |
| `Vectors` | Embeddings in declared named spaces |

Use `Insert` for a vector and payload, `InsertDocument` for a default-space vector plus text, and `InsertRecord` when one logical item carries vectors in several spaces.

Text-only records are valid:

```go
id, err := docs.InsertTextDocument(
    "WAL replay restores completed mutations after a crash.",
    map[string]any{"path": "guide/durability.md", "kind": "guide"},
)
```

They participate in BM25 search, filters, iteration, and direct lookup. Vector search skips them until your application adds a vector.

## Choose Collection and Space Boundaries

| Data relationship | Recommended model |
|---|---|
| Records share one compatible embedding profile | One collection using the default space |
| The same logical items have text, image, audio, or other embeddings | One collection with named vector spaces |
| Records are unrelated or have different lifecycle and access policies | Separate collections |
| You are migrating between incompatible profiles | A new collection or space, followed by an explicit rebuild |

Do not put vectors from incompatible models into the same space, even if their dimensions match. A 768-dimensional vector from one model does not share a coordinate system with a 768-dimensional vector from another.

## Persist Embedding Meaning

`EmbeddingProfile` records what a vector means, not only how long it is:

```go
docs, err := db.CreateCollection("docs",
    veclite.WithEmbeddingProfile(veclite.EmbeddingProfile{
        Provider:  "ollama",
        Model:     "nomic-embed-text",
        Dimension: 768,
        Distance:  veclite.DistanceCosine,
        Normalize: true,
        Version:   "chunker-v2",
    }),
)
```

Before reusing an existing index with a new embedding pipeline, compare profiles:

```go
if err := current.Compatible(incoming); err != nil {
    // Rebuild the collection or write to a new space.
}
```

The check catches provider, model, dimension, distance, normalization, and version changes that can invalidate retrieval. Profiles persist in the database. See [Embedding Strategy](/embeddings) for provider and preprocessing guidance.

## Choose an Index

VecLite uses brute-force vector search unless you enable HNSW.

| Index | Use when | Tradeoff |
|---|---|---|
| Brute force | The collection is small, exact ranking matters, or you are testing | Compares the query with every vector |
| HNSW | The collection is large enough that approximate search saves meaningful time | Uses more memory and requires tuning recall versus latency |

Each vector space has its own dimension, distance metric, and optional HNSW index. BM25 is a separate text index over `Record.Content` and selected payload fields.

VecLite supports cosine, dot product, Euclidean, and squared Euclidean distance. Cosine and dot scores are better when higher; Euclidean distances are better when lower. This distinction matters when setting thresholds. See [Search and Ranking](./search).

## Keep Embedding Work in the Application

VecLite owns durable retrieval primitives:

- record, content, payload, vector, and profile persistence
- vector dimensions, distance metrics, HNSW, and brute-force search
- BM25 indexes, metadata filters, pagination, and result fusion
- snapshots, WAL recovery, and storage migrations

Your application should own domain and provider decisions:

- file walking, chunking, OCR, transcripts, and media extraction
- provider credentials, batching, retries, and model rollout
- which text becomes `Content` and which values belong in `Payload`
- when a changed profile or preprocessor requires a rebuild

VecLite includes optional embedder integrations, but keeping extraction and rebuild policy near the application prevents the database layer from guessing at domain semantics.

## Choose a Process Topology

- **One process reading and writing:** embed the Go library.
- **Several read-only processes:** use `WithReadOnly(true)` and `WithSharedRead(true)`. Each reader sees a point-in-time snapshot and calls `Reload()` to observe later writes.
- **Several clients that write:** run `veclite serve` as the single file owner and use HTTP or the Go HTTP client.

The HTTP server is intended for trusted local or private-network use. It does not provide built-in authentication or TLS. See [Choose an Interface](./interfaces) before exposing it to another process.

## Next Steps

- [Run the quickstart](./getting-started) to create and query a working collection.
- [Choose a search mode](./search) for semantic, keyword, hybrid, or multimodal retrieval.
- [Add named vector spaces](./named-vector-spaces) when one record needs several embeddings.
- [Select a durability mode](./durability) before handling important writes.
