---
title: Go HTTP Client
description: "Use VecLite's Go HTTP client for basic vector operations when one veclite serve process owns the database file."
---

# Go HTTP Client

The `client` package is a thin Go client for a running `veclite serve` process. Use it when several processes need to write to one database and you want the server to be the single file owner.

The client covers the basic collection, vector, payload, filter, and maintenance surface. It is not a drop-in replacement for every embedded-library feature.

::: warning Current scope
The client does not currently expose document insertion, BM25 or hybrid search, named vector spaces, embedding profiles, or agent-memory APIs. `client.WithTextIndex` exists but is not transmitted by `CreateCollection`; do not rely on it. Use the embedded library, CLI, or raw HTTP endpoints for capabilities outside the subset below.
:::

## Choose the Client or Embedded Library

| Scenario | Recommended access |
|---|---|
| One process reads and writes | `veclite.Open(path)` |
| Several processes only read | `veclite.Open(path, WithReadOnly(true), WithSharedRead(true))` |
| Several processes write | `veclite serve` plus `client.Open(url)` |

Shared read-only opens do not take a file lock. They see a point-in-time snapshot and call `db.Reload()` to observe later writes. The HTTP client instead observes the state owned by the server process.

## Start the Server

Install the CLI and start a writer on the loopback interface:

```bash
go install github.com/abdul-hamid-achik/veclite/cmd/veclite@latest
veclite serve data.veclite --host=127.0.0.1 --port=8080 --wal
```

`--wal` makes completed mutations crash-safe. The server has no built-in authentication or TLS; keep it on a trusted local/private network or put an authenticated TLS proxy in front of it.

## Install the Client

```bash
go get github.com/abdul-hamid-achik/veclite/client
```

## Connect and Search

This example uses four-dimensional vectors throughout:

```go
package main

import (
    "fmt"
    "log"

    "github.com/abdul-hamid-achik/veclite/client"
)

func main() {
    db, err := client.Open("http://127.0.0.1:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    docs, err := db.CreateCollection("docs",
        client.WithDimension(4),
        client.WithDistanceType(client.DistanceCosine),
        client.WithHNSW(16, 200),
    )
    if err != nil {
        log.Fatal(err)
    }

    _, err = docs.Insert(
        []float32{0.1, 0.2, 0.3, 0.4},
        map[string]any{"source": "README.md", "kind": "docs"},
    )
    if err != nil {
        log.Fatal(err)
    }

    results, err := docs.Search(
        []float32{0.15, 0.25, 0.35, 0.45},
        client.TopK(5),
        client.WithFilter(client.Equal("kind", "docs")),
    )
    if err != nil {
        log.Fatal(err)
    }

    for _, result := range results {
        fmt.Println(result.Record.ID, result.Score, result.Record.Payload)
    }
}
```

If `docs` already exists, use `db.GetCollection("docs")` rather than creating it again.

## Supported Database Operations

| Method | Description |
|---|---|
| `Open(baseURL)` | Connect to a running server |
| `CreateCollection(name, opts...)` | Create a basic vector collection |
| `Collection(name)` | Return a collection handle without checking existence |
| `GetCollection(name)` | Return a handle after checking existence |
| `DropCollection(name)` | Delete a collection and its records |
| `Collections()` | List collection names |
| `Stats()` | Read database statistics |
| `Sync()` | Ask the server to persist a snapshot |
| `Reload()` | Ask the server to reload from disk |

## Supported Collection Operations

| Method | Description |
|---|---|
| `Insert` / `InsertBatch` | Insert vectors and payloads |
| `Get` / `Delete` | Read or delete a record by ID |
| `UpdateVector` / `Update` | Replace a vector or payload |
| `Upsert` / `UpsertByKey` | Insert or replace a vector record |
| `Search` | Run default-space vector search |
| `Find` / `DeleteWhere` | Match payload filters without a query vector |
| `Stats` | Read collection statistics |

Collection creation supports `WithDimension`, `WithDistanceType`, and `WithHNSW`. Search supports `TopK`, `Threshold`, `WithFilter`, and `WithFilters`.

## Filters

The client exposes equality, inequality, numeric comparison, glob, prefix, suffix, substring, and existence filters:

```go
records, err := docs.Find(
    client.Equal("kind", "docs"),
    client.Suffix("source", ".md"),
)
```

## Error Handling

Non-2xx HTTP responses become descriptive Go errors that include the server's machine-readable code and message when available. The client does not currently expose a typed API-error value, so do not branch on concrete error types or parse the text as a stable contract. Treat connection failures separately from application errors such as a missing collection or dimension mismatch.

## Next Steps

- Read [Choose an Interface](./interfaces) to compare this client with shared reads and raw HTTP.
- Use the [HTTP API reference](/reference/http-api) for operations the client does not wrap.
- Add [WAL durability](./durability) to the server before handling important writes.
