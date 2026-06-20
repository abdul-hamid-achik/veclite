# Named Vector Spaces

Named vector spaces let a single logical record carry **several embeddings at once** — for
example a `text` embedding and an `image` embedding for the same item — each indexed and searched
independently. They are VecLite's answer to multimodal retrieval without forcing you to split one
record across unrelated collections.

This feature is **additive and backward compatible**. Every collection still has one implicit
`default` space, and the entire single-vector API (`Insert`, `Search`, `HybridSearch`,
`UpdateVector`, …) keeps working unchanged on it. Databases written before this feature load as a
collection with only the default space.

## Concepts

| Term | Meaning |
|------|---------|
| **Default space** | The implicit space named `default`, backed by `Record.Vector` and the collection's primary dimension/distance/index. Always present. |
| **Named space** | An extra space declared with `AddVectorSpace`. Has its own dimension, distance metric, and optional HNSW index. Its vectors live in `Record.Vectors[name]`. |
| **Record** | One logical item. May hold a vector in the default space, in any named spaces, both, or neither (text-only). |
| **Embedding profile** | First-class description of how a space's vectors were produced (provider, model, dimension, distance, normalize, version). Optional; validates inserts and detects incompatible re-indexing. |

The reserved name `DefaultVectorSpace` (`"default"`) and the empty string both refer to the
default space. You cannot redeclare it with `AddVectorSpace`.

## Go API

### Declare spaces

```go
db, _ := veclite.Open("items.veclite")
defer db.Close()

coll, _ := db.CreateCollection("items",
    veclite.WithDimension(1536), // default space (text)
    veclite.WithVectorSpace(veclite.VectorSpaceConfig{
        Name:      "image",
        Dimension: 512,
        Distance:  veclite.DistanceCosine,
        Modality:  "image",
        Provider:  "openclip",
        Model:     "ViT-B-32",
        HNSW:      &veclite.HNSWConfig{M: 16, EfConstruction: 200, EfSearch: 100, UseHeuristic: true},
    }),
)

// You can also add a space after creation:
_ = coll.AddVectorSpace(veclite.VectorSpaceConfig{Name: "audio", Dimension: 256})
```

### Insert multi-space records

```go
id, _ := coll.InsertRecord(veclite.RecordInput{
    Content: "a red apple on a table",
    Payload: map[string]any{"label": "apple"},
    Vectors: map[string][]float32{
        veclite.DefaultVectorSpace: textVector,  // 1536-dim
        "image":                    imageVector, // 512-dim
    },
})
```

`InsertRecord` validates every vector against its space's dimension (and embedding profile, if
set) **before** mutating any index. A record may omit any space; it is then simply absent from
that space's results. Passing an existing `ID` replaces the record across all of its spaces.

Add or replace a single space's vector on an existing record with `SetRecordVector`:

```go
_ = coll.SetRecordVector(id, "audio", audioVector)
```

### Search one space

```go
byImage, _ := coll.SearchSpace("image", imageQuery, veclite.TopK(10))
byText, _  := coll.SearchSpace(veclite.DefaultVectorSpace, textQuery, veclite.TopK(10))
```

All standard search options apply: `TopK`, `WithFilter`, `Threshold`, `WithOffset`, `WithEfSearch`.

### Fuse multiple spaces

`MultiSpaceSearch` runs one query per space and fuses the result sets with Reciprocal Rank Fusion,
so a record that ranks highly in several spaces rises to the top:

```go
fused, _ := coll.MultiSpaceSearch(map[string][]float32{
    veclite.DefaultVectorSpace: textQuery,
    "image":                    imageQuery,
}, veclite.TopK(10))
```

For weighted fusion, or to fold in BM25 text results, combine sets yourself with the public
`FuseRRF`:

```go
text, _  := coll.SearchSpace(veclite.DefaultVectorSpace, q, veclite.TopK(50))
image, _ := coll.SearchSpace("image", iq, veclite.TopK(50))
bm25, _  := coll.TextSearch("red apple", veclite.TopK(50))

ranked := veclite.FuseRRF(
    [][]veclite.Result{text, image, bm25},
    veclite.WithFusionWeights(1.0, 0.8, 0.5),
    veclite.WithFusionTopK(10),
)
```

### Inspect spaces

```go
for _, s := range coll.VectorSpaces() {
    fmt.Printf("%s: dim=%d distance=%s index=%s vectors=%d\n",
        s.Name, s.Dimension, s.Distance, s.IndexType, s.VectorCount)
}
```

## Embedding profiles

A profile makes the embedding's *meaning* explicit, not just its length:

```go
coll, _ := db.CreateCollection("code",
    veclite.WithEmbeddingProfile(veclite.EmbeddingProfile{
        Provider: "ollama", Model: "nomic-embed-text",
        Dimension: 768, Distance: veclite.DistanceCosine, Normalize: true, Version: "chunker-v1",
    }),
)

// Per-space profile:
_ = coll.AddVectorSpace(veclite.VectorSpaceConfig{
    Name: "image", Dimension: 512,
    Profile: &veclite.EmbeddingProfile{Provider: "openclip", Model: "ViT-B-32", Dimension: 512},
})

// Detect an index-invalidating change:
if err := stored.Compatible(incoming); err != nil {
    // provider/model/dimension/distance/normalize changed → rebuild.
}
```

Profiles persist in the database. The older convention of storing a profile in collection metadata
still works.

## CLI

The CLI is a language-agnostic surface over the same operations (every command supports `--json`):

```bash
# Declare a named space
veclite space-add items.veclite items --name=image --dim=512 --modality=image --hnsw

# List spaces
veclite spaces items.veclite items --json

# Insert one record with vectors in several spaces
veclite record-insert items.veclite items \
  --vectors='{"default":[...],"image":[...]}' \
  --content='a red apple' --payload='{"label":"apple"}'

# Insert many records from a file (object or array of {id,content,payload,vectors})
veclite record-insert items.veclite items --input=records.json

# Search one space
veclite search-space items.veclite items image --query='[...]' --top-k=5 --json

# Fuse several spaces with RRF
veclite fuse-search items.veclite items --queries='{"default":[...],"image":[...]}' --top-k=10 --json
```

## HTTP

`veclite serve` exposes the same surface as JSON over HTTP:

| Method | Path | Body |
|--------|------|------|
| `GET`  | `/collections/{name}/spaces` | — |
| `POST` | `/collections/{name}/spaces` | `{"name":"image","dimension":512,"modality":"image","hnsw":true}` |
| `POST` | `/collections/{name}/records` | `{"content":"...","payload":{...},"vectors":{"default":[...],"image":[...]}}` |
| `POST` | `/collections/{name}/search-space` | `{"space":"image","query":[...],"top_k":5}` |
| `POST` | `/collections/{name}/fuse-search` | `{"queries":{"default":[...],"image":[...]},"top_k":10}` |

```bash
veclite serve --port=8080 items.veclite
curl -X POST localhost:8080/collections/items/search-space \
  -d '{"space":"image","query":[0.3,0.4],"top_k":5}'
```

> The CLI and HTTP JSON shapes are a stable, cross-language contract. Language drivers (Python,
> TypeScript, …) are planned and will build on exactly these shapes; the `specs/glyphrun/` behavior
> specs pin them down.

## Storage and migration

Named vector spaces use on-disk format **v4**. Older databases (v1–v3) migrate automatically and
losslessly on open: their single vector becomes the implicit `default` space, with no record
rewrite. You never need to run a migration command — opening the file is enough.
