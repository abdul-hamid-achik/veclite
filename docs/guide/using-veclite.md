---
description: "Learn how to use VecLite collections — records, vectors, text content, metadata, HNSW indexes, distance metrics, and BM25 text indexing."
---

# Using VecLite

VecLite stores records in collections. Each record can hold a vector, text content, and payload metadata. Collections define the vector dimension, distance metric, optional HNSW index, optional embedder, and optional BM25 text index fields.

## Choose Collection Boundaries

Use one collection when records share one embedding profile: provider, model, dimension, modality, distance metric, and preprocessing strategy.

```go
code, err := db.CreateCollection("code_chunks",
    veclite.WithDimension(768),
    veclite.WithDistanceType(veclite.DistanceCosine),
    veclite.WithHNSW(16, 200),
    veclite.WithTextIndex("path", "language", "symbol"),
)
```

Use separate collections when embedding profiles differ today. For example, a code search app can keep code text embeddings separate from docs text embeddings, and a video search app can keep transcript text separate from future image vectors.

## Store Embedding Profiles

Dimension checks are useful, but they do not prove that two vectors came from the same model or preprocessing pipeline. Store profile metadata with each collection:

```go
err := code.SetMetadataValue("embedding_profile", map[string]any{
    "profile_id":   "ollama:nomic-embed-text:768:cosine:code-v1",
    "provider":     "ollama",
    "model":        "nomic-embed-text",
    "dimensions":   768,
    "distance":     "cosine",
    "modality":     "text",
    "preprocessor": "code-v1",
})
```

Applications should compare this metadata before inserting or searching. If the profile changes, rebuild the collection or create a new one.

## Use Text-only Records

If an application starts with keyword search before semantic embeddings are ready, insert content without vectors:

```go
id, err := coll.InsertTextDocument("00:12 OCR and transcript evidence", map[string]any{
    "frame": "frames/frame_0012.png",
    "kind":  "evidence",
})
```

Text-only records appear in BM25 search, filters, iteration, and direct lookup. Vector search skips them until you add vectors through a later indexing workflow.

## Combine Vector and Text Search

Hybrid search uses Reciprocal Rank Fusion to merge vector and BM25 results:

```go
matches, err := coll.HybridSearch(
    queryVector,
    "connection pool",
    veclite.TopK(10),
    veclite.WithVectorWeight(1.0),
    veclite.WithTextWeight(0.5),
)
```

Use hybrid search when users type natural-language queries that may include exact symbols, paths, timestamps, or other keywords.

## Keep Embedding Pipelines in the App

VecLite can call an `Embedder`, but applications should still own domain-specific extraction and preprocessing. Keep file chunking, OCR, transcript parsing, frame extraction, batching, retries, and credential handling near the app that understands those workflows.

VecLite should own durable search primitives: storage, indexes, filters, metadata, text search, and result fusion.
