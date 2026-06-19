# Embeddings and Vector Spaces

VecLite stores vectors, text content, and metadata. It does not need to own the full embedding pipeline for every application.

Use VecLite from Go when you want a local vector database with single-file persistence, HNSW search, BM25 text search, metadata filters, and hybrid ranking. Let your application decide how to split content, which embedding provider to call, and when an index should be rebuilt.

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

## Current Model: One Vector per Record

Current VecLite collections store one vector per record. A collection has one dimension and one distance metric.

Use one collection when all records share the same embedding profile:

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

An embedding profile describes the vector meaning, not just its length. Applications should persist a profile next to each index and compare it before indexing or searching.

Recommended fields:

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

## Future Direction: Named Vector Spaces

Named vector spaces are the accepted long-term design for multiple embedding types in one logical record.

Planned shape:

```go
textSpace := veclite.VectorSpaceConfig{
    Name: "text",
    Dimension: 1536,
    Distance: veclite.DistanceCosine,
    Modality: "text",
    Provider: "openai",
    Model: "text-embedding-3-small",
}

frameSpace := veclite.VectorSpaceConfig{
    Name: "frame_clip",
    Dimension: 512,
    Distance: veclite.DistanceCosine,
    Modality: "image",
    Provider: "openclip",
    Model: "ViT-B-32",
}
```

The future API should let apps store:

```go
veclite.RecordInput{
    Content: "00:12 OCR text and transcript text",
    Payload: map[string]any{
        "time_seconds": 12.0,
        "frame": "frames/frame_0012.png",
    },
    Vectors: map[string][]float32{
        "text": textVector,
        "frame_clip": imageVector,
    },
}
```

Current releases do not expose this API yet. Treat named vector spaces as the migration target for future multimodal work, not as current behavior.
