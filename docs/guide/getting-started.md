---
description: "Get started with VecLite — an embeddable vector database for Go. Install, create collections, insert vectors, and search with HNSW and BM25."
---

# Getting Started

VecLite is an embeddable vector database for Go. Use it when your application needs local vector storage, metadata filters, BM25 text search, or hybrid search without running a separate database server.

## Install

```bash
go get github.com/abdul-hamid-achik/veclite
```

## Create a Database

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

    vector := make([]float32, 384)
    vector[0], vector[1], vector[2] = 0.1, 0.2, 0.3

    _, err = coll.InsertDocument(vector, "content to retrieve", map[string]any{
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

    fmt.Println(len(results))
}
```

## Build the Documentation Site

This repository uses VitePress for its documentation site:

```bash
task site
```

Use `task site-dev` while editing docs locally, and `task site-preview` to preview a production build.

## Next Steps

- Read [Using VecLite](./using-veclite.md) for common collection and search patterns.
- Read [Embeddings and Vector Spaces](../embeddings.md) before choosing an embedding provider or storing multiple embedding types.
