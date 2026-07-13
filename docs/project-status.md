---
title: Compatibility and Support
description: "VecLite v0.24.0 compatibility: Go requirements, storage migrations, supported access surfaces, optional integrations, and current limitations."
---

# Compatibility and Support

VecLite `v0.24.0` is the current release. It requires Go 1.25 or later and is distributed under the MIT license.

- [Release notes](https://github.com/abdul-hamid-achik/veclite/releases/tag/v0.24.0)
- [All releases](https://github.com/abdul-hamid-achik/veclite/releases)
- [Go package reference](https://pkg.go.dev/github.com/abdul-hamid-achik/veclite)

## Supported Access Surfaces

| Surface | Status | Best fit |
|---|---|---|
| Embedded Go library | Supported | A Go process that owns local reads and writes |
| Go HTTP client | Supported subset | Basic vector operations through `veclite serve` |
| CLI | Supported | Shell automation and JSON-based language bridges |
| HTTP server | Supported | Trusted multi-client access with one writer process |
| MCP server | Supported | MCP-compatible agents and coding tools over stdio |
| Python or TypeScript SDK | Not available | Use the CLI or HTTP JSON contract today |

The CLI and HTTP JSON shapes are treated as a public cross-language contract and covered by behavior specs under `specs/glyphrun/`. Additive fields may appear over time; clients should ignore fields they do not recognize.

The Go HTTP client intentionally covers a smaller surface than the embedded library. It currently wraps basic collection, vector, payload, filter, and maintenance operations—not document/BM25, hybrid, named-space, or agent-memory APIs. See [Go HTTP Client](./guide/go-client).

## Storage Compatibility

The current on-disk storage format is version 4. VecLite migrates version 1–3 snapshots automatically when opening them:

- historical single vectors become the implicit `default` vector space
- named-space and embedding-profile fields are added without rewriting record content
- no separate migration command is required

Back up the snapshot before opening an important database with a new release. If WAL is enabled, make sure the writer is stopped cleanly or preserve both the snapshot and `.wal` sidecar so recovery can replay completed mutations.

## Build Profiles

The default build includes the core database, CLI, HTTP server, MCP server, YAML configuration, and HTTP-based embedding providers.

Local ONNX inference is isolated behind the `onnx` build tag:

```bash
go build -tags onnx ./cmd/veclite
```

Core storage, vector indexing, BM25, filters, and fusion are implemented with the Go standard library. Focused external modules support optional integrations such as MCP, YAML configuration, and ONNX inference.

## Operational Boundaries

VecLite is a local, embeddable database rather than a distributed database cluster.

- One process should own file-backed writes.
- Multiple read-only processes can use lock-free shared reads and call `Reload()` for a newer snapshot.
- Multiple writing clients should connect to one `veclite serve` process.
- The HTTP server has no built-in authentication or TLS. Keep it on loopback or a trusted private network, or place an authenticated TLS proxy in front of it.
- Snapshot persistence occurs on `Sync()` or `Close()`. Enable the WAL when completed writes must survive a crash between snapshots.

Read [Choose an Interface](./guide/interfaces) for topology guidance and [Durability and the WAL](./guide/durability) for the write guarantees of each mode.

## Reporting a Problem

Open a [GitHub issue](https://github.com/abdul-hamid-achik/veclite/issues) with:

- VecLite version and Go version
- operating system and architecture
- the smallest reproducing program or CLI command
- database options, collection dimension, distance metric, and index configuration
- the complete error and relevant logs, with credentials and private data removed

For search-quality reports, include dataset size, vector dimension, `M`, `efConstruction`, `efSearch`, query count, and the brute-force result used for comparison.
