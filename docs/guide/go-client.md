# Go Client

The `client` package provides a thin Go client for a running `veclite serve` HTTP server.
It mirrors the embedded [veclite](https://pkg.go.dev/github.com/abdul-hamid-achik/veclite)
library's API surface so that Go consumers can swap between embedding the library directly
(single-process) and talking to a remote server (multi-process) with minimal code change.

## When to use the client

| Scenario | Use |
|----------|-----|
| Single process, file-based | `veclite.Open(path)` (embedded library) |
| Multi-process read-only | `veclite.Open(path, WithReadOnly(true), WithSharedRead(true))` |
| Multi-process with writes | `veclite serve` + `client.Open(url)` |

The client is the recommended approach when multiple processes need to **write** to the
same database. One process runs `veclite serve` (owns the exclusive file lock), and all
other processes use the Go client to talk to it over HTTP.

## Installation

```bash
go get github.com/abdul-hamid-achik/veclite/client
```

## Quick start

```go
import "github.com/abdul-hamid-achik/veclite/client"

db, err := client.Open("http://localhost:8080")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

coll, err := db.CreateCollection("docs",
    client.WithDimension(384),
    client.WithHNSW(16, 200),
)

id, err := coll.Insert([]float32{0.1, 0.2, 0.3}, map[string]any{"source": "wiki"})

results, err := coll.Search([]float32{0.1, 0.2, 0.3}, client.TopK(10))
```

## DB operations

| Method | Description |
|--------|-------------|
| `Open(baseURL)` | Connect to a server |
| `Close()` | Release client resources (no server-side session) |
| `CreateCollection(name, opts...)` | Create a collection |
| `Collection(name)` | Get a handle (no existence check) |
| `GetCollection(name)` | Get a handle (error if not found) |
| `DropCollection(name)` | Delete a collection |
| `Collections()` | List collection names |
| `Stats()` | Database-level statistics |
| `Sync()` | Force sync to disk on the server |
| `Reload()` | Reload database from disk (pick up external writes) |

## Collection operations

| Method | Description |
|--------|-------------|
| `Insert(vector, payload)` | Insert a single vector |
| `InsertBatch(vectors, payloads)` | Insert multiple vectors |
| `Get(id)` | Get a record by ID |
| `Delete(id)` | Delete a record by ID |
| `UpdateVector(id, vector)` | Replace the vector |
| `Update(id, payload)` | Replace the payload |
| `Upsert(id, vector, payload)` | Insert or update by ID |
| `UpsertByKey(keyField, keyValue, vector, payload)` | Insert or update by payload key |
| `Search(query, opts...)` | Vector similarity search |
| `Find(filters...)` | Find records by metadata filter |
| `DeleteWhere(filters...)` | Delete records by filter |

## Collection options

| Option | Description |
|--------|-------------|
| `WithDimension(d)` | Set vector dimension |
| `WithDistanceType(d)` | Set distance metric (`DistanceCosine`, `DistanceDot`, `DistanceEuclidean`) |
| `WithHNSW(m, ef)` | Enable HNSW index with M and efConstruction |
| `WithTextIndex(field)` | Enable BM25 text indexing on a payload field |

## Search options

| Option | Description |
|--------|-------------|
| `TopK(k)` | Maximum number of results (default 10) |
| `Threshold(t)` | Minimum similarity score |
| `WithFilter(f)` | Add a metadata filter |
| `WithFilters(filters...)` | Add multiple filters |

## Filters

| Filter | Description |
|--------|-------------|
| `Equal(key, value)` | Equality match |
| `NotEqual(key, value)` | Not-equal match |
| `GT(key, value)` | Greater than (numeric) |
| `GTE(key, value)` | Greater than or equal |
| `LT(key, value)` | Less than |
| `LTE(key, value)` | Less than or equal |
| `Glob(key, pattern)` | Glob pattern match |
| `Prefix(key, value)` | String prefix |
| `Suffix(key, value)` | String suffix |
| `Contains(key, value)` | String contains |
| `Exists(key)` | Key exists |

## Swapping from embedded to client

The API is designed for a minimal diff:

```go
// Before (embedded, single-process):
db, _ := veclite.Open("data.veclite")
coll, _ := db.CreateCollection("docs", veclite.WithDimension(384))
coll.Insert(vec, payload)
results, _ := coll.Search(query, veclite.TopK(10))

// After (client, multi-process):
db, _ := client.Open("http://localhost:8080")
coll, _ := db.CreateCollection("docs", client.WithDimension(384))
coll.Insert(vec, payload)
results, _ := coll.Search(query, client.TopK(10))
```

The main difference is that `float32` vectors are passed directly (the client converts
to `float64` for JSON transport internally), and `Reload()` is available to pick up
writes from other processes.