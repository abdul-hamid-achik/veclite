---
description: "Choose between vector, BM25, hybrid RRF, and multi-space search in VecLite, then tune filters, thresholds, and HNSW for your workload."
---

# Choose a Search Strategy

VecLite has four retrieval paths. Choose the path from the kind of evidence your query carries, not from collection size.

| Your query needs | Start with | Why |
|---|---|---|
| Meaning, paraphrases, or conceptual similarity | `Search` | Compares an embedding with the default vector space. |
| Exact words, identifiers, paths, or error codes | `TextSearch` | Ranks indexed text with BM25; no query embedding is required. |
| Meaning **and** exact terms | `HybridSearch` | Fuses vector and BM25 rankings with Reciprocal Rank Fusion (RRF). |
| Several embedding models or modalities for one record | `SearchSpace` or `MultiSpaceSearch` | Searches independent named spaces and can fuse their rankings. |

If you are unsure, enable a text index and evaluate vector, BM25, and hybrid retrieval against the same representative queries. Hybrid search is a strong default for user-entered text, but it is rank fusion—not a new similarity metric.

## Run the core searches

The following program uses small hand-written vectors so you can run it without an embedding provider:

```go
package main

import (
    "fmt"
    "log"

    "github.com/abdul-hamid-achik/veclite"
)

func main() {
    db, err := veclite.Open(":memory:")
    if err != nil {
        log.Fatal(err)
    }
    defer func() { _ = db.Close() }()

    docs, err := db.CreateCollection("docs",
        veclite.WithDimension(3),
        veclite.WithDistanceType(veclite.DistanceCosine),
        veclite.WithTextIndex("title", "kind"),
    )
    if err != nil {
        log.Fatal(err)
    }

    mustInsert := func(vector []float32, content, title, kind string) {
        _, err := docs.InsertDocument(vector, content, map[string]any{
            "title": title,
            "kind":  kind,
        })
        if err != nil {
            log.Fatal(err)
        }
    }

    mustInsert([]float32{1, 0, 0}, "Tune a database connection pool.", "Pooling", "guide")
    mustInsert([]float32{0.9, 0.1, 0}, "Diagnose HTTP timeout errors.", "Timeouts", "guide")
    mustInsert([]float32{0, 1, 0}, "Reference for ERR_POOL_EXHAUSTED.", "Error codes", "reference")

    vector, err := docs.Search([]float32{1, 0, 0}, veclite.TopK(2))
    if err != nil {
        log.Fatal(err)
    }

    lexical, err := docs.TextSearch("ERR_POOL_EXHAUSTED", veclite.TopK(2))
    if err != nil {
        log.Fatal(err)
    }

    hybrid, err := docs.HybridSearch(
        []float32{1, 0, 0},
        "connection pool",
        veclite.TopK(2),
        veclite.WithVectorWeight(1.0),
        veclite.WithTextWeight(0.8),
    )
    if err != nil {
        log.Fatal(err)
    }

    filtered, err := docs.Search(
        []float32{1, 0, 0},
        veclite.TopK(2),
        veclite.WithFilter(veclite.Equal("kind", "guide")),
    )
    if err != nil {
        log.Fatal(err)
    }

    printResults("vector", vector)
    printResults("BM25", lexical)
    printResults("hybrid RRF", hybrid)
    printResults("filtered vector", filtered)
}

func printResults(label string, results []veclite.Result) {
    fmt.Println(label)
    for _, result := range results {
        fmt.Printf("  id=%d score=%.4f title=%v\n",
            result.Record.ID,
            result.Score,
            result.Record.Payload["title"],
        )
    }
}
```

Save it as `main.go`, then run:

```bash
go mod init veclite-search-demo
go get github.com/abdul-hamid-achik/veclite
go run .
```

In a real application, replace the hand-written vectors with embeddings from one consistent model and preprocessing pipeline.

## Vector search

Use `Search` when semantic similarity is the primary signal:

```go
results, err := docs.Search(queryVector,
    veclite.TopK(10),
    veclite.Threshold(0.72),
)
```

`Search` operates on the implicit `default` vector space. Records without a default-space vector are skipped. VecLite validates the query dimension against the collection dimension.

Vector search can run as an exact brute-force scan or through an approximate HNSW index. The API and result shape stay the same.

## BM25 text search

Enable BM25 when you create the collection:

```go
docs, err := db.CreateCollection("docs",
    veclite.WithDimension(768),
    veclite.WithTextIndex("title", "path", "language"),
)
```

`WithTextIndex` always indexes `Record.Content` and additionally indexes string values from the named payload fields. Then query it with:

```go
results, err := docs.TextSearch("ERR_POOL_EXHAUSTED", veclite.TopK(10))
```

BM25 is lexical. VecLite lowercases terms, splits on whitespace, and trims common punctuation from token edges. It does not stem words, expand synonyms, or perform phrase parsing. Text-only records inserted with `InsertTextDocument` participate in BM25 but are skipped by vector search.

## Hybrid search with RRF

`HybridSearch` retrieves candidates from the default vector space and BM25, then combines their ranks:

```go
results, err := docs.HybridSearch(
    queryVector,
    "connection pool",
    veclite.TopK(10),
    veclite.WithVectorWeight(1.0),
    veclite.WithTextWeight(0.6),
)
```

For each result, RRF adds a contribution based on its position in each list:

```text
fused score = sum(weight / (60 + position)) // position starts at 1
```

This makes vector similarities and BM25 scores safely composable without pretending that their numeric scales match. A record that ranks well in both lists receives both contributions. Hybrid search requires a text index and non-empty vector and text queries.

Weights must be positive to override the default of `1.0`. If you only want one retrieval leg, call `Search` or `TextSearch` directly.

## Named and multi-space search

Use named vector spaces when one logical record has embeddings from different models or modalities. Each space has its own dimension, distance metric, and optional HNSW index.

```go
items, err := db.CreateCollection("items",
    veclite.WithDimension(3), // default text space
    veclite.WithTextIndex("sku"),
    veclite.WithVectorSpace(veclite.VectorSpaceConfig{
        Name:      "image",
        Dimension: 2,
        Distance:  veclite.DistanceCosine,
        Modality:  "image",
    }),
)
if err != nil {
    log.Fatal(err)
}

_, err = items.InsertRecord(veclite.RecordInput{
    Content: "a red bicycle",
    Payload: map[string]any{"sku": "bike-1"},
    Vectors: map[string][]float32{
        veclite.DefaultVectorSpace: []float32{1, 0, 0},
        "image":                    []float32{0.8, 0.2},
    },
})
if err != nil {
    log.Fatal(err)
}

imageMatches, err := items.SearchSpace("image", []float32{1, 0}, veclite.TopK(10))
if err != nil {
    log.Fatal(err)
}

fused, err := items.MultiSpaceSearch(map[string][]float32{
    veclite.DefaultVectorSpace: []float32{1, 0, 0},
    "image":                    []float32{1, 0},
}, veclite.TopK(10))
if err != nil {
    log.Fatal(err)
}

fmt.Println(len(imageMatches), len(fused))
```

`MultiSpaceSearch` uses equal-weight RRF. A record can omit a space; it simply cannot appear in that space's result set. To weight spaces differently or include BM25, retrieve each list and call `FuseRRF`:

```go
textResults, _ := items.SearchSpace(veclite.DefaultVectorSpace, textQuery, veclite.TopK(50))
imageResults, _ := items.SearchSpace("image", imageQuery, veclite.TopK(50))
keywordResults, _ := items.TextSearch("red bicycle", veclite.TopK(50))

results := veclite.FuseRRF(
    [][]veclite.Result{textResults, imageResults, keywordResults},
    veclite.WithFusionWeights(1.0, 0.8, 0.5),
    veclite.WithRRFK(60),
    veclite.WithFusionTopK(10),
)
```

See [Named Vector Spaces](./named-vector-spaces.md) for insertion, profiles, CLI commands, and HTTP routes.

## Filter before you trust a result

Filters apply to record payloads and built-in record fields. Multiple `WithFilter` or `WithFilters` options use AND logic:

```go
results, err := docs.Search(queryVector,
    veclite.TopK(10),
    veclite.WithFilters(
        veclite.Equal("tenant", "acme"),
        veclite.In("kind", "guide", "reference"),
        veclite.GreaterThanOrEqual("quality", 0.8),
        veclite.NotExpired(),
    ),
)
```

Compose more complex predicates with `And`, `Or`, and `Not`:

```go
scope := veclite.And(
    veclite.Equal("tenant", "acme"),
    veclite.Or(
        veclite.Prefix("path", "docs/"),
        veclite.Glob("path", "examples/*.go"),
    ),
)

results, err := docs.TextSearch("connection pool", veclite.WithFilter(scope))
```

Available payload predicates include equality, membership, string prefix/suffix/contains/glob, existence, numeric comparisons, and ranges. Time-aware filters include `CreatedAfter`, `AgeNewerThan`, `NotExpired`, and importance/access filters. You can implement custom logic with `FilterFunc`.

Expiry and archival are not implicit search policies. Use `NotExpired()` to hide expired records that have not yet been cleaned up. `ArchiveRecord` sets the reserved `_archived` payload flag; exclude it explicitly when needed:

```go
active := veclite.And(
    veclite.NotExpired(),
    veclite.Not(veclite.Equal(veclite.PayloadKeyArchived, true)),
)
```

With HNSW, filtered searches over-fetch candidates and fall back to brute force if too few survive. Selective filters can therefore trade approximate-search speed for completeness.

BM25 takes its top-ranked candidates before applying filters. For a selective text filter, over-fetch and limit afterward:

```go
results, err := docs.TextSearch("connection pool",
    veclite.TopK(100),
    veclite.WithLimit(10),
    veclite.WithFilter(veclite.Equal("tenant", "acme")),
)
```

## Interpret scores and thresholds

`Result.Score` means different things on different retrieval paths:

| Result source | Better direction | Scale |
|---|---|---|
| Cosine vector search | Higher | `-1` to `1`; `1` means the same direction. |
| Dot-product vector search | Higher | Unbounded and affected by vector magnitude. |
| Euclidean vector search | Lower | `0` is identical. |
| Squared Euclidean vector search | Lower | `0` is identical; values are squared distances. |
| BM25 | Higher | Corpus- and query-dependent lexical relevance. |
| RRF fusion | Higher | Rank contribution, not vector similarity or BM25 relevance. |

`Threshold` follows the vector space's direction:

```go
// Keep cosine scores >= 0.75.
cosineResults, _ := cosineCollection.Search(query, veclite.Threshold(0.75))

// Keep Euclidean distances <= 0.40.
nearby, _ := euclideanCollection.Search(query, veclite.Threshold(0.40))
```

Threshold behavior by path matters:

- `Search` and `SearchSpace` apply it to vector scores.
- `MultiSpaceSearch` applies the same numeric threshold independently to every queried space. Avoid this shortcut when spaces use incompatible score scales; search each space separately instead.
- `HybridSearch` and `HybridSearchSpace` apply it only to the vector leg, before RRF.
- `TextSearch` does not apply `Threshold`.
- RRF scores should not be compared with a cosine, dot-product, Euclidean, or BM25 threshold.

Choose thresholds from labeled queries produced by the same embedding model and preprocessing pipeline. HNSW is approximate, so a threshold does not guarantee that every qualifying record was visited.

## Decide when to enable HNSW

Collections use exact brute-force search unless you opt into HNSW:

```go
docs, err := db.CreateCollection("docs",
    veclite.WithDimension(768),
    veclite.WithHNSW(16, 200),
)
```

| Setting | Increase it when | Cost |
|---|---|---|
| `M` | Recall is insufficient and graph memory is acceptable. | More memory, slower writes, and potentially slower search. |
| `EfConstruction` | You can spend more time building a higher-quality graph. | Slower inserts and index builds. |
| `EfSearch` | Per-query recall matters more than latency. | More candidates visited per query. |

`WithHNSW(16, 200)` also sets the default `EfSearch` to `100` and enables heuristic neighbor selection. Override search breadth per query:

```go
fast, _ := docs.Search(queryVector, veclite.TopK(10), veclite.WithEfSearch(40))
highRecall, _ := docs.Search(queryVector, veclite.TopK(10), veclite.WithEfSearch(250))
```

Brute force is often the better baseline for small collections, highly selective filters, or workloads that need exact results. Benchmark both paths with production-like dimensions, collection sizes, filters, and `TopK` values.

For an unfiltered default-space query, `SearchExplain` shows whether HNSW or brute force ran and reports visited nodes and layers:

```go
explanation, err := docs.SearchExplain(queryVector,
    veclite.TopK(10),
    veclite.WithEfSearch(100),
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("index=%s brute_force=%t visited=%d duration=%s\n",
    explanation.IndexType,
    explanation.BruteForce,
    explanation.NodesVisited,
    explanation.Duration,
)
```

`SearchExplain` deliberately uses brute force when filters or a threshold are present, so use normal `Search` when evaluating filtered HNSW behavior.

## Production checklist

- Keep query and stored embeddings on the same model, dimension, normalization, and distance metric.
- Index `Content` and only the string payload fields that improve lexical recall.
- Apply tenant, authorization, expiry, and archival filters to every relevant retrieval leg.
- Treat BM25 and RRF scores as query-relative ranks, not calibrated probabilities.
- Evaluate recall before tuning HNSW for latency.
- Fetch a wider candidate set before application-level reranking.

Next, see [Build Agent Memory](./agent-memory.md) for recency, importance, TTL, conversations, episodes, and graph expansion.
