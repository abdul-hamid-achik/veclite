---
description: "Create a VecLite database, store vectors and metadata, and run your first similarity search from Go or the CLI."
---

# Getting Started

In this quickstart, you create a local VecLite database, add two records, and retrieve the closest record by cosine similarity. Choose the Go path when VecLite runs inside your application, or the CLI path when you want a shell-friendly JSON interface.

VecLite stores and searches vectors; your application normally produces them with an embedding model. The three-dimensional vectors below are deliberately small so you can run the examples without an API key or model download. Do not mix vectors from different models in the same vector space.

## Prerequisites

- Go 1.25 or later
- A terminal with `go` on your `PATH`
- For the CLI path, a directory on Go's binary path (usually `$(go env GOPATH)/bin`) on your `PATH`

## Option 1: Embed VecLite in Go

Create a small Go module:

```bash
mkdir veclite-quickstart
cd veclite-quickstart
go mod init example.com/veclite-quickstart
go get github.com/abdul-hamid-achik/veclite
```

Save the following as `main.go`:

```go
package main

import (
	"fmt"
	"log"

	"github.com/abdul-hamid-achik/veclite"
)

func main() {
	db, err := veclite.Open("quickstart.veclite", veclite.WithWAL(true))
	if err != nil {
		log.Fatal(err)
	}

	// Collection creates the collection on first use. Its dimension is inferred
	// from the first vector; its default index is an exact brute-force search.
	docs := db.Collection("docs")

	_, _, err = docs.UpsertByKey(
		"slug",
		"veclite",
		[]float32{1, 0, 0},
		map[string]any{"slug": "veclite", "title": "VecLite overview"},
	)
	if err != nil {
		log.Fatal(err)
	}

	_, _, err = docs.UpsertByKey(
		"slug",
		"gardening",
		[]float32{0, 1, 0},
		map[string]any{"slug": "gardening", "title": "Growing tomatoes"},
	)
	if err != nil {
		log.Fatal(err)
	}

	results, err := docs.Search([]float32{1, 0, 0}, veclite.TopK(1))
	if err != nil {
		log.Fatal(err)
	}
	if len(results) == 0 {
		log.Fatal("search returned no results")
	}

	fmt.Printf("best match: %s (score %.3f)\n",
		results[0].Record.Payload["title"], results[0].Score)

	// Close writes the current snapshot and releases the file lock.
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
}
```

Run it:

```bash
go run .
```

Expected output:

```text
best match: VecLite overview (score 1.000)
```

The `quickstart.veclite` file now contains the collection and both records. The example uses `UpsertByKey`, so running it again replaces the records with matching `slug` values instead of duplicating them. `WithWAL(true)` also protects completed writes between full snapshot saves.

For a production collection, declare the dimension, distance metric, HNSW index, text index, and embedding profile explicitly with `CreateCollection` before inserting data. The convenience `Collection` method used above creates an exact-search collection with defaults.

## Option 2: Use the CLI with JSON

Install the command:

```bash
go install github.com/abdul-hamid-achik/veclite/cmd/veclite@latest
veclite version
```

Create a three-dimensional collection with an HNSW index:

```bash
veclite create-collection quickstart-cli.veclite docs \
  --dimension=3 --distance=cosine --hnsw --json
```

Insert two records:

```bash
veclite insert quickstart-cli.veclite docs \
  --vector='[1,0,0]' \
  --payload='{"slug":"veclite","title":"VecLite overview"}' \
  --json

veclite insert quickstart-cli.veclite docs \
  --vector='[0,1,0]' \
  --payload='{"slug":"gardening","title":"Growing tomatoes"}' \
  --json
```

Search for the closest record:

```bash
veclite search quickstart-cli.veclite docs \
  --query='[1,0,0]' --top-k=1 --json
```

You should receive one result whose payload contains `"title": "VecLite overview"` and whose score is `1`:

```json
[
  {
    "id": 1,
    "score": 1,
    "payload": {
      "slug": "veclite",
      "title": "VecLite overview"
    }
  }
]
```

The CLI opens the file for each command and closes it after the operation. Write commands require VecLite's exclusive writer lock; read-only commands use lock-free shared-read mode. The create command expects a new collection, so use a fresh database path if you repeat this sequence from the beginning.

`--json` is available on many data commands, but support varies by command. Run `veclite <command> --help` or see the [CLI reference](/reference/cli) before depending on a command's output in a script.

## Where to Go Next

- [Choose an interface](/guide/interfaces) for embedded Go, shared reads, HTTP, CLI, or MCP.
- Learn the [core collection and record concepts](/guide/using-veclite).
- Replace the example vectors with a real [embedding strategy](/embeddings).
- Add HNSW, filters, BM25, or hybrid ranking in [Search and Ranking](/guide/search).
- Store several embeddings per record with [Named Vector Spaces](/guide/named-vector-spaces).
- Choose snapshot and WAL behavior in [Durability and the WAL](/guide/durability).
