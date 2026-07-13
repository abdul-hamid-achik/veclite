---
description: "VecLite embedding strategy: the app/library boundary, embedding profiles, named vector spaces for multimodal records, BM25 text search, and hybrid ranking."
---

# Embeddings and Vector Spaces

VecLite stores vectors, text content, and metadata. It does not need to own the full embedding pipeline for every application.

Use VecLite from Go when you want a local vector database with a portable snapshot, HNSW search,
BM25 text search, metadata filters, and hybrid ranking. Writers may also create lock and WAL
sidecars. Let your application decide how to split content, which embedding provider to call, and
when an index should be rebuilt.

## Install

```bash
go get github.com/abdul-hamid-achik/veclite
```

## Basic Go Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/abdul-hamid-achik/veclite"
)

func main() {
    db, err := veclite.Open("vectors.veclite")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    coll, err := db.CreateCollection("docs",
        veclite.WithDimension(384),
        veclite.WithDistanceType(veclite.DistanceCosine),
        veclite.WithHNSW(16, 200),
        veclite.WithTextIndex("path", "kind"),
    )
    if err != nil {
        log.Fatal(err)
    }

    vector := make([]float32, 384) // use your embedder output here
    vector[0], vector[1], vector[2] = 0.1, 0.2, 0.3
    id, err := coll.InsertDocument(vector, "content to retrieve and text-search", map[string]any{
        "path": "README.md",
        "kind": "docs",
    })
    if err != nil {
        log.Fatal(err)
    }

    results, err := coll.Search(vector, veclite.TopK(5))
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(id, len(results))
}
```

The vector in that example is intentionally app-provided. You can use VecLite's optional embedders, or you can use your own provider interface as `vecgrep` does.

## What VecLite Owns

VecLite should own durable search primitives:

- record, content, payload, and vector persistence
- vector dimensions, distance metrics, and compatibility checks
- HNSW indexes and brute-force fallback
- BM25 text indexing over `Record.Content` and selected payload fields
- metadata filters, pagination, iteration, and hybrid search
- storage-versioned migrations for database format changes

## What Applications Own

Applications should own domain and provider concerns:

- file walking, source-code chunking, OCR, transcript parsing, and frame extraction
- embedding provider selection, credentials, batching, retry policy, and model rollout
- deciding which text becomes `Content` and which values become payload fields
- recording the embedding profile used for an index
- forcing or suggesting re-index when provider, model, dimensions, chunking, or preprocessing changes

This boundary keeps VecLite importable from any Go project while still letting applications build richer workflows on top.

## Collections, Spaces, and Records

A collection has one **default** vector space (one dimension and one distance metric, backed by
`Record.Vector`) and may declare additional **named** vector spaces (see below). Choose the
smallest structure that fits:

- **One collection, default space** — all records share one embedding profile.
- **One collection, named spaces** — records are the same logical items but carry several
  embeddings (e.g. text + image). Use this for multimodal records.
- **Separate collections** — records are genuinely unrelated, or you are mid-migration between
  incompatible profiles.

Use one collection with the default space when all records share the same embedding profile:

```go
code, _ := db.CreateCollection("code_chunks",
    veclite.WithDimension(768),
    veclite.WithDistanceType(veclite.DistanceCosine),
    veclite.WithHNSW(16, 200),
    veclite.WithTextIndex("relative_path", "language", "symbol_name"),
)
```

Use separate collections when you need incompatible embedding types today:

```text
code_text_768       text embeddings for code chunks
evidence_text_1536  text embeddings for video timeline entries
evidence_bm25       keyword-only evidence records inserted with InsertTextDocument
```

Do not mix vectors from different providers, models, dimensions, or modalities in the same collection unless you deliberately rebuilt the collection for that profile.

## Embedding Profiles

An embedding profile describes the vector meaning, not just its length. As of the named-vector-space release, `EmbeddingProfile` is a **first-class type** you can attach to a collection's default space or to an individual named vector space — it persists in the database and validates inserted vectors against its dimension:

```go
coll, _ := db.CreateCollection("code_chunks",
    veclite.WithEmbeddingProfile(veclite.EmbeddingProfile{
        Provider:  "ollama",
        Model:     "nomic-embed-text",
        Dimension: 768,
        Distance:  veclite.DistanceCosine,
        Normalize: true,
        Version:   "chunker-v1",
    }),
)

// Compatible reports whether two profiles describe interchangeable vectors.
if err := current.Compatible(incoming); err != nil {
    // provider/model/dimension/distance/normalize changed — rebuild the index.
}
```

`EmbeddingProfile.Compatible` catches the class of errors a dimension check cannot: a model swap that keeps the same dimension still produces incompatible vectors. The older "store a profile in collection metadata" convention below still works for callers that prefer untyped metadata.

Recommended metadata fields (untyped convention):

```json
{
  "profile_id": "ollama:nomic-embed-text:768:cosine:chunker-v1",
  "provider": "ollama",
  "model": "nomic-embed-text",
  "dimensions": 768,
  "distance": "cosine",
  "modality": "text",
  "preprocessor": "code-chunker-v1"
}
```

If any field that changes vector meaning differs, the application should re-index or create a new collection. Dimension checks catch only one class of error; they do not catch a model change that keeps the same dimension.

Store the profile in collection metadata:

```go
_ = code.SetMetadataValue("embedding_profile", map[string]any{
    "profile_id":   "ollama:nomic-embed-text:768:cosine:chunker-v1",
    "provider":     "ollama",
    "model":        "nomic-embed-text",
    "dimensions":   768,
    "distance":     "cosine",
    "modality":     "text",
    "preprocessor": "code-chunker-v1",
})
```

## BM25 and Hybrid Search

BM25 works over record content and selected payload fields:

```go
coll, _ := db.CreateCollection("docs",
    veclite.WithTextIndex("title", "path"),
)

_, _ = coll.InsertTextDocument("database connection pooling", map[string]any{
    "title": "Runtime notes",
    "path": "docs/runtime.md",
})

matches, _ := coll.TextSearch("connection pool", veclite.TopK(10))
```

Hybrid search fuses vector and BM25 result sets:

```go
matches, _ := coll.HybridSearch(
    queryVector,
    "connection pool",
    veclite.TopK(10),
    veclite.WithVectorWeight(1.0),
    veclite.WithTextWeight(0.5),
)
```

For keyword-first applications that do not yet have semantic embeddings, use `InsertTextDocument`. Use `InsertTextDocumentWithOptions` when text-only records need TTL or importance settings. Text-only records are indexed by BM25 and skipped by vector search until you add semantic vectors later.

## Named Vector Spaces

Named vector spaces let one logical record hold several embeddings at once — for example a
`text` embedding and an `image` embedding for the same item — each with its own dimension,
distance metric, and HNSW index. This is the supported way to model multimodal records without
splitting them across unrelated collections.

```go
coll, _ := db.CreateCollection("frames",
    veclite.WithDimension(1536), // the default space (text)
    veclite.WithVectorSpace(veclite.VectorSpaceConfig{
        Name:      "frame_clip",
        Dimension: 512,
        Distance:  veclite.DistanceCosine,
        Modality:  "image",
        Provider:  "openclip",
        Model:     "ViT-B-32",
        HNSW:      &veclite.HNSWConfig{M: 16, EfConstruction: 200, EfSearch: 100, UseHeuristic: true},
    }),
)

// One logical record, vectors in two spaces.
id, _ := coll.InsertRecord(veclite.RecordInput{
    Content: "00:12 OCR text and transcript text",
    Payload: map[string]any{"time_seconds": 12.0, "frame": "frames/frame_0012.png"},
    Vectors: map[string][]float32{
        veclite.DefaultVectorSpace: textVector,  // the "text" default space
        "frame_clip":               imageVector,
    },
})

// Search one space, or fuse several with Reciprocal Rank Fusion.
byText, _  := coll.SearchSpace(veclite.DefaultVectorSpace, textQuery, veclite.TopK(10))
byImage, _ := coll.SearchSpace("frame_clip", imageQuery, veclite.TopK(10))
fused, _   := coll.MultiSpaceSearch(map[string][]float32{
    veclite.DefaultVectorSpace: textQuery,
    "frame_clip":               imageQuery,
}, veclite.TopK(10))
```

The default space (`Record.Vector`) stays fully backward compatible: every existing single-vector
API operates on it, and databases written before this feature load as a collection with only the
default space. See the dedicated **[Named Vector Spaces](/guide/named-vector-spaces)** guide for the
full Go, CLI, and HTTP surface.
