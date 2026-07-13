---
description: "VecLite HTTP API reference: server startup, security, collection and vector routes, metadata filters, named vector spaces, hybrid search, and JSON response contracts."
---

# HTTP API Reference

`veclite serve` exposes a VecLite database as a JSON HTTP API. It is useful for local multi-client
access and as the cross-language contract for applications that do not import the Go package.

## Start the server

```bash
veclite serve data.veclite --host=127.0.0.1 --port=8080 --wal
```

| Flag | Default | Description |
|---|---:|---|
| `--host` | `127.0.0.1` | Interface to bind. |
| `--port` | `8080` | HTTP port. |
| `--cors` | off | Sends permissive CORS headers, including `Access-Control-Allow-Origin: *`. |
| `--wal` | off | Enables the write-ahead log so completed writes survive a crash between snapshots. |

The examples below use `http://127.0.0.1:8080` as the base URL. Send JSON request bodies with
`Content-Type: application/json`.

::: danger Security boundary
The built-in server has **no authentication, authorization, or TLS**. It includes destructive
write and delete routes. Keep the default loopback binding for local use. If you bind to
`0.0.0.0` or another network interface, place the server behind a trusted reverse proxy or gateway
that provides access control and HTTPS. `--cors` only changes browser cross-origin behavior; it
does not secure the API.
:::

## JSON conventions

- Request field names use `snake_case`. In particular, all search routes use `top_k`, not
  `topK`, `top-k`, or `topk`.
- Vectors and queries are JSON number arrays. VecLite converts them to `float32` internally.
- Omitted or non-positive `top_k` values use the library search default (`10`).
- A threshold is applied when the JSON field is present, including when its value is `0`.
- Success bodies are route-specific JSON objects or arrays; there is no universal success wrapper.
- Unknown JSON fields are currently ignored. Do not depend on that behavior for compatibility.

### Errors

Errors use a consistent envelope and an appropriate HTTP status such as `400`, `404`, `405`,
`409`, or `500`:

```json
{
  "error": "Query vector is required",
  "code": "MISSING_QUERY"
}
```

The optional `details` field may provide more context. Client code should branch on the HTTP
status and `code`; human-readable `error` text can change.

### Search responses

Vector, named-space, fused, and hybrid search normally return:

```json
{
  "results": [
    {
      "id": 1,
      "score": 0.9981,
      "payload": {"kind": "guide"}
    }
  ],
  "count": 1
}
```

For the core `POST /collections/{name}/search` route only, send
`Accept: application/x-ndjson` to stream one result object per line. The NDJSON form does not have
the `results`/`count` envelope.

## Metadata filters

Search, find, fused search, and conditional delete accept filter objects:

```json
{
  "filters": [
    {"key": "kind", "op": "eq", "value": "guide"},
    {"key": "score", "op": "gte", "value": 0.8}
  ]
}
```

| Operation | Accepted `op` values | Value type |
|---|---|---|
| Equal | omitted, `eq`, `=` | Any JSON value |
| Not equal | `neq`, `!=` | Any JSON value |
| Greater than | `gt`, `>` | Number |
| Greater than or equal | `gte`, `>=` | Number |
| Less than | `lt`, `<` | Number |
| Less than or equal | `lte`, `<=` | Number |
| Glob | `glob` | String pattern |
| Prefix | `prefix` | String |
| Suffix | `suffix` | String |
| Contains | `contains` | String |
| Field exists | `exists` | Value is ignored |

## System routes

| Method | Path | Request | Success response |
|---|---|---|---|
| `GET` | `/health` | None | `{"status":"ok","version":"..."}` |
| `GET` | `/info` | None | Database path, collection count, total record count, and version. |
| `GET` | `/metrics` | None | Search, insert, and delete counts plus `avg_search_time_ns`. |
| `POST` | `/sync` | Empty body | `{"status":"synced"}` |
| `POST` | `/reload` | Empty body | `{"status":"reloaded"}` |

`/sync` forces a snapshot to disk. `/reload` reloads the database from disk so the process can
observe changes made by another writer.

## Collections

### List collections

```http
GET /collections
```

The response is a JSON array. Each item has `name`, `count`, `dimension`, `distance`, and
`index_type`.

### Create a collection

```http
POST /collections
Content-Type: application/json

{
  "name": "docs",
  "dimension": 3,
  "distance": "cosine",
  "hnsw": true,
  "hnsw_m": 16,
  "hnsw_ef": 200
}
```

| Field | Required | Default | Notes |
|---|---|---|---|
| `name` | Yes | — | Collection name. |
| `dimension` | No | `0` | `0` auto-detects the dimension on first insert. |
| `distance` | No | `cosine` | `cosine`, `dot`, or `euclidean`. |
| `hnsw` | No | `false` | Enables HNSW instead of brute-force search. |
| `hnsw_m` | No | `16` | Maximum HNSW connections when `hnsw` is true. |
| `hnsw_ef` | No | `200` | HNSW construction search width when `hnsw` is true. |

A successful create returns `201 Created`:

```json
{"status":"created","collection":"docs"}
```

::: warning Text indexing is not configurable here
`POST /collections` has no `text_index` or `text-index` field. Sending one has no effect. To use
BM25 or `hybrid-search-space`, create the collection before starting the server:

```bash
veclite create-collection data.veclite docs \
  --dimension=3 --text-index=title,kind
veclite serve data.veclite
```
:::

Some insert and upsert handlers resolve a missing collection with default settings. Use
`POST /collections` when you need to set dimension, distance, or HNSW explicitly.

### Inspect or drop a collection

| Method | Path | Success response |
|---|---|---|
| `GET` | `/collections/{name}` | `name`, `count`, `dimension`, `distance`, and `index_type`. |
| `DELETE` | `/collections/{name}` | `{"status":"dropped","collection":"docs"}` |

## Default-space vectors

These routes operate on the collection's implicit `default` vector space.

### Insert one vector

```http
POST /collections/docs/vectors
Content-Type: application/json

{
  "vector": [0.1, 0.2, 0.3],
  "payload": {"title": "Quickstart", "kind": "guide"}
}
```

Response: `201 Created` with `{"status":"inserted","id":1}`.

### Insert a batch

Use `vectors` and a parallel `payloads` array:

```json
{
  "vectors": [
    [0.1, 0.2, 0.3],
    [0.4, 0.5, 0.6]
  ],
  "payloads": [
    {"kind": "guide"},
    {"kind": "reference"}
  ]
}
```

Response: `201 Created` with `status`, `count`, and `ids`.

### List, read, update, and delete vectors

| Method | Path | Request | Success response |
|---|---|---|---|
| `GET` | `/collections/{name}/vectors?offset=0&limit=100` | Query parameters are optional; defaults are `0` and `100`. | `records`, `count`, `offset`, and `limit`. Each record has `id`, `vector`, and optional `payload` and `content`. |
| `GET` | `/collections/{name}/vectors/{id}` | None | `id`, `vector`, `payload`, `created_at`, and `updated_at`. |
| `PUT` | `/collections/{name}/vectors/{id}` | `{"vector":[...],"payload":{...}}`; either field may be omitted. | `{"status":"updated","id":1}` |
| `DELETE` | `/collections/{name}/vectors/{id}` | None | `{"status":"deleted","id":1}` |

### Search default-space vectors

```http
POST /collections/docs/search
Content-Type: application/json

{
  "query": [0.11, 0.19, 0.31],
  "top_k": 5,
  "threshold": 0.7,
  "filters": [
    {"key": "kind", "op": "eq", "value": "guide"}
  ]
}
```

`query` is required. `top_k`, `threshold`, and `filters` are optional.

### Upsert a vector

Upsert by record ID:

```http
POST /collections/docs/upsert
Content-Type: application/json

{
  "id": 42,
  "vector": [0.1, 0.2, 0.3],
  "payload": {"kind": "guide"}
}
```

Or identify a record by a payload field:

```json
{
  "key_field": "slug",
  "key_value": "quickstart",
  "vector": [0.1, 0.2, 0.3],
  "payload": {"kind": "guide"}
}
```

`vector` is required. With `key_field`, the response status is `inserted` or `updated`; otherwise
it is `upserted`.

### Find or conditionally delete records

```http
POST /collections/docs/find
Content-Type: application/json

{
  "filters": [{"key": "kind", "op": "eq", "value": "guide"}],
  "limit": 20
}
```

`find` returns `{"results":[...],"count":N}`. Each result contains `id` plus optional `payload`
and `content`. An empty `filters` array finds all records.

```http
DELETE /collections/docs/vectors
Content-Type: application/json

{
  "filters": [{"key": "kind", "op": "eq", "value": "temporary"}]
}
```

Conditional delete requires at least one filter and returns
`{"status":"deleted","deleted":N}`.

### Collection maintenance

| Method | Path | Request | Success response |
|---|---|---|---|
| `POST` | `/collections/{name}/compact` | Empty body | `status` and `collection`. |
| `POST` | `/collections/{name}/validate` | Empty body | `collection`, `valid`, `issues`, and record `count`. |

## Named vector spaces

Every collection has an implicit `default` space. The following routes manage and query
additional spaces while keeping one logical record ID and payload.

### List spaces

```http
GET /collections/{name}/spaces
```

The response envelope contains `spaces` and `count`. Each space reports `name`, `dimension`,
`distance`, optional embedding hints (`modality`, `provider`, `model`), `index_type`, and
`vector_count`.

### Add a space

```http
POST /collections/items/spaces
Content-Type: application/json

{
  "name": "image",
  "dimension": 2,
  "distance": "cosine",
  "modality": "image",
  "provider": "openclip",
  "model": "ViT-B-32",
  "hnsw": true,
  "hnsw_m": 16,
  "hnsw_ef": 200
}
```

`name` is required and cannot be `default`. `dimension` defaults to auto-detection. Named spaces
accept `cosine`, `dot`, `euclidean`, and `euclidean_squared` (also spelled
`euclidean-squared`). A successful create returns `201 Created`.

### Insert a multi-space record

```http
POST /collections/items/records
Content-Type: application/json

{
  "id": 0,
  "content": "a red apple",
  "payload": {"sku": "apple-1"},
  "vectors": {
    "default": [0.1, 0.2, 0.3],
    "image": [0.8, 0.2]
  }
}
```

At least one of `vectors` or `content` is required. ID `0` auto-assigns; an existing non-zero ID
replaces that logical record. The response is `201 Created` with `status` and `id`.

### Upsert a multi-space record by key

```http
POST /collections/items/records-upsert-by-key
Content-Type: application/json

{
  "key_field": "sku",
  "key_value": "apple-1",
  "content": "a red apple",
  "payload": {"category": "fruit"},
  "vectors": {
    "default": [0.1, 0.2, 0.3],
    "image": [0.8, 0.2]
  }
}
```

`key_field` and a non-null `key_value` are required. The response includes `status` (`inserted` or
`replaced`), `id`, `inserted`, and `collection`.

### Search one space

```http
POST /collections/items/search-space
Content-Type: application/json

{
  "space": "image",
  "query": [0.79, 0.21],
  "top_k": 5,
  "threshold": 0.7,
  "filters": [{"key": "category", "op": "eq", "value": "fruit"}]
}
```

`query` is required. `space` defaults to the implicit `default` space when empty.

### Fuse several spaces

```http
POST /collections/items/fuse-search
Content-Type: application/json

{
  "queries": {
    "default": [0.1, 0.2, 0.3],
    "image": [0.79, 0.21]
  },
  "top_k": 5,
  "filters": []
}
```

`queries` must contain at least one space. VecLite searches each space and combines the rankings
with Reciprocal Rank Fusion (RRF).

### Hybrid vector and BM25 search

```http
POST /collections/items/hybrid-search-space
Content-Type: application/json

{
  "space": "default",
  "query": [0.1, 0.2, 0.3],
  "text": "red apple",
  "top_k": 5,
  "threshold": 0.7,
  "filters": []
}
```

`query` and non-empty `text` are required. `space` defaults to `default`. The collection must have
a BM25 text index, which you currently configure through the CLI or Go API before starting the
server.
