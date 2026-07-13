---
description: "Choose between embedded Go, lock-free shared reads, the HTTP server and Go client, CLI JSON, or MCP for accessing VecLite."
---

# Choose an Interface

VecLite can run inside a Go process or behind command, HTTP, and MCP interfaces. The right choice depends on who owns the database file, how many processes need to write, and how much of the library API you need.

## Quick Decision Guide

| Your situation | Start with | Why |
|---|---|---|
| One Go process owns the data | Embedded Go | No serialization, full library surface, in-process concurrency |
| Other processes only need to inspect a live database | Shared read | Lock-free, point-in-time reads directly from the file |
| Several processes or languages need to write | HTTP server | One server owns the writer lock and coordinates clients |
| A Go program needs basic remote vector operations | Go HTTP client | Typed wrapper over the server's basic collection and vector endpoints |
| A script or pipeline runs discrete operations | CLI with JSON | Process-friendly commands and stable JSON shapes where `--json` is supported |
| An AI agent needs VecLite tools | MCP server | Stdio tools for search, memory, graphs, conversations, and episodes |

There are no official Python or TypeScript SDKs yet. From those languages, call the HTTP API directly or run CLI commands with JSON output.

## Feature Coverage at a Glance

| Interface | Coverage | Important boundary |
|---|---|---|
| Embedded Go | Full exported library API | A file-backed writer owns an exclusive file lock |
| Shared-read Go | The library's read surface | Mutations return `ErrReadOnly`; call `Reload` for newer state |
| Raw HTTP | Basic collections and vectors, filters, search, named spaces, sync/reload, and maintenance endpoints | It is a useful subset, not a transport for every Go API |
| Go HTTP client | Basic collection lifecycle, vector CRUD/search, filters, sync, and reload | Smaller than raw HTTP and much smaller than embedded Go |
| CLI | Core data operations, named spaces, inspection, and maintenance commands | JSON support varies by command |
| MCP | A curated agent-oriented tool surface | It is not a one-to-one wrapper around either Go or HTTP |

## Embedded Go

Embed VecLite when one Go process can own the database and you want the lowest-overhead path or the complete feature set.

```go
db, err := veclite.Open("data.veclite", veclite.WithWAL(true))
if err != nil {
	log.Fatal(err)
}
defer db.Close()

docs, err := db.CreateCollection("docs",
	veclite.WithDimension(384),
	veclite.WithHNSW(16, 200),
)
```

One `DB` supports concurrent goroutines, but a file-backed read-write open holds the database's exclusive writer lock. A second writer process cannot open the same file. Use `:memory:` for tests or temporary indexes.

Choose embedded Go for the full surface, including BM25 and hybrid search, named vector spaces, embedding profiles, memory features, knowledge graphs, conversations, and episodes.

## Lock-free Shared Reads

Use shared-read mode when another process owns writes and your Go process only needs a point-in-time view:

```go
reader, err := veclite.Open("data.veclite",
	veclite.WithReadOnly(true),
	veclite.WithSharedRead(true),
)
if err != nil {
	log.Fatal(err)
}
defer reader.Close()

// Later, discard the in-memory view and load the latest complete state.
if err := reader.Reload(); err != nil {
	log.Fatal(err)
}
```

This mode takes no long-lived file lock, so a reader does not block a writer and a writer does not block the reader. Each open or reload sees a complete snapshot; it does not observe changes continuously. Call `Reload` when freshness matters. If a WAL sidecar exists, read-only open and reload replay it in memory without modifying the files.

`WithSharedRead(true)` requires `WithReadOnly(true)`. Shared-read handles cannot mutate collections, records, metadata, graphs, or episode stores.

## HTTP Server

Run one HTTP server when multiple clients need coordinated writes or when non-Go applications need a long-running JSON API:

```bash
veclite serve data.veclite --host=127.0.0.1 --port=8080 --wal
```

Confirm that it is ready:

```bash
curl http://127.0.0.1:8080/health
```

The server owns the exclusive writer lock for its lifetime. All HTTP clients send operations to that one owner instead of opening the database file themselves. `--wal` is recommended for a long-running writer so completed requests survive a crash between full snapshot saves.

The built-in server has no authentication or TLS. It binds to loopback by default; keep it there for local use. If you expose it on a network, put it behind a trusted reverse proxy that supplies authentication and TLS. `--cors` only adds browser CORS headers—it is not an access-control mechanism.

The HTTP API is a subset of embedded Go. In particular, its create-collection JSON schema cannot configure a BM25 text index. Create text-indexed collections through embedded Go or the CLI before serving the database. See the [HTTP API reference](/reference/http-api) for the endpoints and exact JSON shapes.

### Go HTTP Client

The `client` package wraps the basic HTTP collection and vector endpoints:

```go
import "github.com/abdul-hamid-achik/veclite/client"

db, err := client.Open("http://127.0.0.1:8080")
if err != nil {
	log.Fatal(err)
}
defer db.Close()

docs := db.Collection("docs")
results, err := docs.Search(query, client.TopK(10))
```

Treat this client as a smaller basic-vector subset, not API parity with the embedded library. It covers collection lifecycle, vector CRUD and search, filters, sync, and reload. It does not wrap named-space, BM25/hybrid, or agent-memory features.

Although `client.WithTextIndex` currently exists, `client.CreateCollection` does not transmit that setting, and the HTTP create schema has no matching field. Do not rely on it; create the text-indexed collection through embedded Go or the CLI first. See the [Go client guide](/guide/go-client) for its implemented methods.

## CLI with JSON

Use the CLI for shell scripts, CI jobs, data import, inspection, and other one-operation-at-a-time workflows:

```bash
veclite info data.veclite --json
veclite search data.veclite docs --query='[0.1,0.2,0.3]' --top-k=5 --json
```

Each command opens the database, performs its operation, and closes it. Read-only commands use lock-free shared-read mode, which makes them suitable for inspecting a file while another process writes. Write commands still need the exclusive writer lock and fail if a long-lived writer such as `veclite serve` owns it.

The CLI and HTTP JSON shapes are treated as public cross-language contracts. However, `--json` support varies by CLI command. Check `veclite <command> --help` and the [CLI reference](/reference/cli) before building a parser around a command.

For a long-running application or frequent requests, prefer HTTP instead of starting a process and reopening the database for every operation.

## MCP for AI Agents

Run the MCP server when an MCP-compatible agent needs VecLite as a set of tools:

```bash
veclite mcp /absolute/path/to/data.veclite
```

A typical client configuration launches that command over stdio:

```json
{
  "mcpServers": {
    "veclite": {
      "command": "veclite",
      "args": ["mcp", "/absolute/path/to/data.veclite"]
    }
  }
}
```

MCP exposes curated tools for core vector operations and agent-oriented workflows such as memory, knowledge graphs, conversations, and episodes. Vector-based tools can accept vectors supplied by the caller. Tools that turn text into vectors require a configured embedding provider.

The MCP process reads through a cached lock-free handle and refreshes its view periodically. For a mutation, it acquires the exclusive writer lock for that tool call and releases it afterward. An HTTP server holding the same database open read-write therefore prevents MCP write tools from acquiring the lock, although MCP reads can continue. Choose one long-lived write owner per database.

## Safe Combinations

- An embedded writer plus shared-read Go or read-only CLI processes can coexist. Readers call `Reload` or reopen to see newer state.
- An HTTP server plus any number of HTTP or Go-client consumers can read and write through that server.
- Read-only CLI commands can inspect a database owned by the HTTP server, but CLI write commands cannot acquire its writer lock.
- MCP can share lock-free reads with another writer, but its write tools need moments when no other process holds the exclusive writer lock.

When several independent processes must write continuously, route them through one `veclite serve` process. Do not give each process direct read-write access to the same file.

## Next Steps

- Run the [Go and CLI quickstarts](/guide/getting-started).
- Review [Durability and the WAL](/guide/durability) before operating a long-running writer.
- Explore the [HTTP API](/reference/http-api), [CLI commands](/reference/cli), or [Go client](/guide/go-client) in detail.
- Learn how application-owned embedding pipelines fit VecLite in [Embedding Strategy](/embeddings).
