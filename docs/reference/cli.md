---
description: "Complete VecLite CLI reference: inspect databases, manage collections and vectors, search named spaces, run the HTTP or MCP server, and maintain database files."
---

# CLI Reference

The `veclite` CLI reads and writes the same portable snapshot format as the Go library. It is also a
stable JSON-in/JSON-out surface for scripts and future language drivers.

```bash
veclite <command> [options] [arguments]
```

Run `veclite help` to list commands, or `veclite <command> --help` for that command's flags. In the
forms below, `<file>` is the database path and `<collection>` is a collection name. Keep positional
arguments together at the start or end of the command; the examples use positional arguments first.

## JSON and scripting

Most commands support `--json`. These details matter when you call the CLI from another process:

- `--json` is a command flag, not a universal flag. It is supported by the inspection, collection,
  vector, named-space, validation, compaction, and benchmark commands shown below.
- `dump` always writes JSON and does **not** accept `--json`.
- `version`, `help`, `serve`, `mcp`, and `unlock` do not support `--json`.
- Command failures print `Error: ...` as text to standard error and exit non-zero, even when
  `--json` is set. Only successful output is JSON.
- Inline vectors, vector maps, and payloads must be valid JSON. Quote them so the shell does not
  interpret brackets, braces, or wildcard characters.
- For CLI search commands, `--threshold=0` disables threshold filtering; only positive values are
  applied. The HTTP API treats a present `threshold` field as explicit, including `0`.
- CLI search commands return a bare JSON array. The HTTP API instead returns a
  `{ "results": [...], "count": N }` envelope.
- `dump` includes each record's ID, default-space `vector`, and payload. It does not include
  `content` or named-space vectors, so do not treat it as a full-fidelity backup format.

For example:

```bash
result=$(veclite search data.veclite docs \
  --query='[0.1,0.2,0.3]' --top-k=5 --json)
```

## Inspect databases

| Command | Required positional arguments | Options and behavior |
|---|---|---|
| `version` | None | Prints the CLI and library versions as text. |
| `info` | `<file>` | `--json`; reports collection and record counts plus WAL sidecar status. |
| `collections` | `<file>` | `--json`; lists collection name, count, dimension, distance, and index type. |
| `stats` | `<file>` | `--json`; prints database and per-collection statistics. |
| `dump` | `<file>` | `--collection=<name>` limits the dump to one collection; `--limit=<n>` limits records per collection. Output is always JSON. |
| `get` | `<file> <collection>` | Requires `--id=<uint64>`; supports `--json`. Reads a default-space vector and its payload. |
| `find` | `<file> <collection>` | `--filter='key=value'`, `--limit=<n>`, `--json`. Without a filter, returns all records. |

CLI filters support exact equality (`key=value`) and glob matching when the value contains `*` or
`?` (`path=*.md`). Numeric values are parsed as numbers.

## Collections, vectors, and search

| Command | Required positional arguments | Required and useful options |
|---|---|---|
| `create-collection` | `<file> <name>` | `--dimension=<n>` (`0` auto-detects), `--distance=cosine\|dot\|euclidean`, `--hnsw`, `--hnsw-m=<n>`, `--hnsw-ef=<n>`, `--text-index=field1,field2`, `--json`. |
| `drop-collection` | `<file> <name>` | `--json`. Drops the collection and all of its records. |
| `insert` | `<file> <collection>` | Requires `--vector='[...]'`; optional `--payload='{...}'` and `--json`. |
| `batch-insert` | `<file> <collection>` | Requires `--input=<path>`; accepts a JSON array or JSONL; supports `--json`. |
| `search` | `<file> <collection>` | Requires `--query='[...]'`; optional `--top-k=<n>` (default `10`), `--threshold=<n>`, `--filter='key=value'`, and `--json`. Searches the default space. |
| `upsert` | `<file> <collection>` | Requires `--vector='[...]'`. Use `--id=<n>` or `--key-field=<field> --key-value=<value>`; optional `--payload` and `--json`. ID `0` auto-assigns. |
| `update` | `<file> <collection>` | Requires `--id=<n>` and at least one of `--vector='[...]'` or `--payload='{...}'`; supports `--json`. |
| `delete` | `<file> <collection>` | Requires `--id=<n>`; supports `--json`. |
| `delete-where` | `<file> <collection>` | Requires `--filter='key=value'`; supports `--json`. |

`--text-index` names payload fields to index in addition to a record's `Content`. It enables BM25
text search and hybrid search for that collection.

### Core workflow

```bash
# Create a three-dimensional, HNSW-backed collection.
veclite create-collection data.veclite docs \
  --dimension=3 --distance=cosine --hnsw --text-index=title,kind --json

# Insert a default-space vector and payload.
veclite insert data.veclite docs \
  --vector='[0.1,0.2,0.3]' \
  --payload='{"title":"Quickstart","kind":"guide"}' --json

# Search and filter it.
veclite search data.veclite docs \
  --query='[0.11,0.19,0.31]' --top-k=5 --filter='kind=guide' --json

# Inspect or update a known ID.
veclite get data.veclite docs --id=1 --json
veclite update data.veclite docs --id=1 \
  --payload='{"title":"VecLite quickstart","kind":"guide"}' --json
```

### Batch input

`batch-insert --input` accepts either a JSON array:

```json
[
  {"vector": [0.1, 0.2, 0.3], "payload": {"kind": "guide"}},
  {"vector": [0.4, 0.5, 0.6], "payload": {"kind": "reference"}}
]
```

or JSONL with one object per line:

```jsonl
{"vector":[0.1,0.2,0.3],"payload":{"kind":"guide"}}
{"vector":[0.4,0.5,0.6],"payload":{"kind":"reference"}}
```

## Named vector spaces

Every collection has an implicit `default` space. Named spaces let one record carry independent
embeddings for text, images, audio, or other modalities.

| Command | Required positional arguments | Required and useful options |
|---|---|---|
| `spaces` | `<file> <collection>` | `--json`; includes the implicit `default` space. |
| `space-add` | `<file> <collection>` | Requires `--name=<space>`. Optional `--dim=<n>`, `--distance=cosine\|dot\|euclidean\|euclidean_squared`, `--modality`, `--provider`, `--model`, `--hnsw`, `--hnsw-m`, `--hnsw-ef`, and `--json`. The name `default` is reserved. |
| `record-insert` | `<file> <collection>` | Requires `--vectors='{...}'` or `--input=<path>`. Inline records also accept `--id`, `--content`, `--payload`, and `--json`. An input file may contain one record object or an array. |
| `record-upsert-by-key` | `<file> <collection>` | Requires `--key-field`, `--key-value`, and `--vectors='{...}'`; optional `--content`, `--payload`, and `--json`. |
| `search-space` | `<file> <collection> <space>` | Requires `--query='[...]'`; optional `--top-k`, `--threshold`, `--filter`, and `--json`. |
| `fuse-search` | `<file> <collection>` | Requires `--queries='{ "space": [...] }'`; optional `--top-k`, `--filter`, and `--json`. Fuses rankings with Reciprocal Rank Fusion (RRF). |
| `hybrid-search-space` | `<file> <collection> [space]` | Requires `--query='[...]'` and `--text='<query>'`; optional `--top-k`, `--threshold`, `--filter`, and `--json`. The space defaults to `default`. |

`record-insert --input` uses this shape:

```json
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

### Multi-space workflow

```bash
# The default space belongs to the collection; add an independent image space.
veclite create-collection catalog.veclite items \
  --dimension=3 --text-index=label
veclite space-add catalog.veclite items \
  --name=image --dim=2 --modality=image --distance=cosine --hnsw

# Insert one logical item into both spaces.
veclite record-insert catalog.veclite items \
  --vectors='{"default":[0.1,0.2,0.3],"image":[0.8,0.2]}' \
  --content='a red apple' --payload='{"sku":"apple-1","label":"apple"}' --json

# Search one modality or fuse both rankings.
veclite search-space catalog.veclite items image \
  --query='[0.79,0.21]' --top-k=5 --json
veclite fuse-search catalog.veclite items \
  --queries='{"default":[0.1,0.2,0.3],"image":[0.79,0.21]}' \
  --top-k=5 --json

# Fuse the default vector space with BM25 text results.
veclite hybrid-search-space catalog.veclite items default \
  --query='[0.1,0.2,0.3]' --text='red apple' --top-k=5 --json
```

## Server and integration modes

| Command | Required positional arguments | Options and behavior |
|---|---|---|
| `serve` | `<file>` | `--host=<host>` (default `127.0.0.1`), `--port=<port>` (default `8080`), `--cors`, `--wal`. Starts the HTTP API. |
| `mcp` | `<file>` | Starts the MCP tool server over standard input/output. It has no CLI flags. |

See the [HTTP API reference](./http-api.md) before exposing `serve` beyond your machine.

## Maintenance

| Command | Required positional arguments | Options and behavior |
|---|---|---|
| `compact` | `<file>` | `--json`; rewrites the snapshot and reports bytes saved. |
| `validate` | `<file>` | `--json`; checks whether the database opens and inspects vector dimensions and zero vectors. Returns non-zero when invalid. |
| `benchmark` | `<file>` | Requires `--collection=<name>`; optional `--queries=<n>` (default `100`), `--top-k=<n>` (default `10`), and `--json`. |
| `unlock` | `<file>` | `--force`; removes stale locks automatically. Refuses to remove a live process's lock unless forced. No JSON mode. |

Use `unlock --force` only after confirming that the reported process cannot write the database
again. Removing a live writer's lock can allow concurrent writes and corrupt the database.
