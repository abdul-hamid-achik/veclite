# Plan: Vidtrace Adoption of VecLite Named Vector Spaces

> **Status:** Ready for consumer-side implementation (Phases 1–4). Phase 0
> (the API gap check) is **resolved** — VecLite `v0.17.0` shipped the two
> methods this migration depended on. See "Phase 0 — Resolved" below.
> **Author:** Forge
> **Date:** 2026-06-21
> **Tracking:** `~/notes/projects/vidtrace/Artifact Polish and Bash Extractor Removal.md`
> (Missing Work section)
> **Related:**
> - VecLite: `collection_spaces.go`, `vectorspace.go`, `AGENTS.md:92-106`
> - Vidtrace: `internal/evidence/evidence.go`, `docs/adr/0003-use-veclite-for-optional-evidence-search.md`

## Objective

Migrate the `vidtrace` evidence index from its current **dual-collection**
layout to a **single collection with named vector spaces**, using the
`AddVectorSpace` / `InsertRecord` / `SearchSpace` / `MultiSpaceSearch` API
shipped in VecLite `v0.16.0`.

This is a consumer-side change (lives in the `vidtrace` repo), but the design
is authored here because:

1. It exercises the new VecLite API as the first non-trivial real-world
   consumer, and may surface API gaps or papercuts worth fixing upstream.
2. VecLite's `AGENTS.md:251-264` requires new vector-space behavior to be
   mirrored on CLI/server and covered by `specs/glyphrun/` specs; the
   consumer migration is a forcing function for confirming those surfaces.
3. Keeping the design next to the feature it depends on makes the upstream
   contract review easy.

## Why Now

- VecLite `v0.16.0` shipped `AddVectorSpace`, `InsertRecord`,
  `SearchSpace`, `MultiSpaceSearch`, `SetRecordVector`, and
  `EmbeddingProfile` compatibility checks — all additive and
  backward-compatible (`AGENTS.md:101-105`, `collection_spaces.go:21-89`).
- Vidtrace just bumped to `v0.16.0` (see vidtrace `CHANGELOG.md` and
  `internal/evidence/evidence.go:10`). The dual-collection layout is now
  the single biggest source of complexity in the evidence package.

## Current State (the thing we are removing)

Vidtrace currently uses **three collections** to support optional semantic
search:

| Collection | Purpose | Created by |
|---|---|---|
| `evidence_entries_keyword` | BM25 text index over every timeline entry (always built) | `keywordCollection` (`evidence.go:631-638`) |
| `evidence_entries_text` | Vector + BM25 hybrid index (built only when `--embed` is set) | `textCollection` (`evidence.go:413-422`) |
| `evidence_meta` | Stores the embedding profile (provider/model/dim) so search can detect mismatched embedders | `metaCollection` (`evidence.go:424-429`) |

### Problems with this layout

1. **Duplicated content.** `recordsForBundle` (`evidence.go:689-716`)
   builds one `record` per timeline entry and the same content + payload is
   written to *both* `evidence_entries_keyword` and `evidence_entries_text`.
   Every timeline entry is stored twice when semantic search is enabled.
2. **Duplicated filter wiring.** `buildFilters` (`evidence.go:643-687`)
   builds one set of payload filters that must be applied correctly to two
   different collections with the same payload schema. Drift risk is real.
3. **No relationship between the two copies.** There is no id guaranteeing
   that the keyword record and the semantic record for the same
   `evidence_id` are the same logical record. `indexSemanticBundle`
   (`evidence.go:358-411`) does `DeleteWhere(evidence_id=...)` then
   `InsertDocument` because veclite has no upsert that carries both vector
   and content — but VecLite `v0.16.0` now does: `InsertRecord`.
4. **Hand-rolled embedding profile.** `evidence_meta` re-implements what
   `EmbeddingProfile` (`vectorspace.go:77-101`) now provides as a first-class
   persisted type with `Compatible` checks. The hand-rolled version
   (`ensureEmbeddingProfile`, `evidence.go:433-475`) is a subset of the
   real thing and can drift.
5. **Keyword-only indexes pay nothing today, but they will.** Future
   multimodal evidence (frame image embeddings alongside text) will need a
   third collection under the current pattern, then a fourth. The
   dual-collection pattern does not scale.

## Target State

One collection, `evidence_entries`, with:

- The implicit **`default` vector space** used as the keyword/BM25 space
  (text-only records). This preserves the existing keyword-only contract:
  `vidtrace index` with no `--embed` still produces a searchable BM25 index
  with zero vector overhead.
- A named **`text` vector space** (declared via `AddVectorSpace` when the
  first `--embed` run happens) carrying the text embedding for each record
  that has one.

### Sketch

```go
// internal/evidence/evidence.go (target)

const (
    CollectionName   = "evidence_entries"
    TextVectorSpace  = "text" // named space, not the reserved "default"
)

func ensureEvidenceCollection(db *veclite.DB, withTextSpace bool) (*veclite.Collection, error) {
    if db.HasCollection(CollectionName) {
        return db.GetCollection(CollectionName)
    }
    opts := []veclite.Option{
        veclite.WithTextIndex("evidence_id", "bundle", "source_video", "frame", "ocr_path", "source"),
    }
    coll, err := db.CreateCollection(CollectionName, opts...)
    if err != nil {
        return nil, err
    }
    if withTextSpace {
        // Embedder Profile is known at index time; attach it as a first-class
        // profile so VecLite validates every inserted vector against it.
        if err := coll.AddVectorSpace(veclite.VectorSpaceConfig{
            Name:     TextVectorSpace,
            Modality: "text",
            Provider: profile.Provider,
            Model:    profile.Model,
            Distance: veclite.DistanceCosine,
            Profile: &veclite.EmbeddingProfile{
                Provider:  profile.Provider,
                Model:     profile.Model,
                Dimension: profile.Dimensions,
                Distance:  veclite.DistanceCosine,
                Normalize: true,
            },
            HNSW: &veclite.HNSWConfig{M: 16, EfConstruction: 200, EfSearch: 64},
        }); err != nil {
            return nil, err
        }
    }
    return coll, nil
}
```

### Insert path

```go
// Replace the dual indexLoadedInto + indexSemanticBundle pair with one loop.

vec := map[string][]float32{}
if embedder != nil {
    // batch embed all records once, then attach per-record
    vec[TextVectorSpace] = vectors[i]
}

_, err := coll.InsertRecord(veclite.RecordInput{
    Content: item.content,
    Payload: item.payload,
    Vectors: vec, // empty for keyword-only runs -> text-only record
})
```

`InsertRecord` (`collection_spaces.go:292-313`) upserts by `in.ID`; to stay
idempotent by `evidence_id`, keep the current `evidenceID(...)` → deterministic
numeric ID mapping OR look up the existing record's ID by
`evidence_id` payload before insert. (Design question Q1 below.)

### Search path

```go
switch mode {
case ModeKeyword:
    results, err = coll.TextSearch(query, searchOpts...)      // BM25 only
case ModeSemantic:
    results, err = coll.SearchSpace(TextVectorSpace, queryVec, searchOpts...)
case ModeHybrid:
    // Option A: veclite-native multimodal fusion
    results, err = coll.MultiSpaceSearch(map[string][]float32{
        TextVectorSpace: queryVec,
    }, searchOpts...)
    // then FuseRRF with BM25 results, OR
    // Option B: keep existing HybridSearch on the text space (simplest migration)
    results, err = coll.HybridSearch(queryVec, query, searchOpts...)
}
```

`MetaCollection` and `ensureEmbeddingProfile` are deleted entirely; the
`EmbeddingProfile` on the `text` space is the source of truth, read via
`coll.VectorSpace(TextVectorSpace)`.

## Plan

### Phase 0 — Resolved (upstream, done in VecLite `v0.17.0`)

All three open questions are resolved upstream. VecLite `v0.17.0` shipped:

1. **`UpsertRecordByKey(keyField string, keyValue any, in RecordInput) (uint64, bool, error)`**
   — the named-space analog of `UpsertTextDocumentByKey`. Mirrors the
   pre-1.0 API's scan-and-replace behavior, preserves `CreatedAt` and
   `AccessCount` on replace, and carries vectors across all the record's
   spaces atomically. Vidtrace can index idempotently by `evidence_id`
   without a manual `Find`-then-`InsertRecord` round-trip.
   - Code: `collection_spaces.go` (`UpsertRecordByKey`).
   - CLI: `veclite record-upsert-by-key`.
   - HTTP: `POST /collections/{name}/records-upsert-by-key`.

2. **`HybridSearchSpace(space string, query []float32, text string, opts ...SearchOption) ([]Result, error)`**
   — fuses vector results from a named space with BM25 text results via
   RRF. Passing `""` or `DefaultVectorSpace` is equivalent to
   `HybridSearch`. Vidtrace's `ModeHybrid` can now target the `text`
   named space directly.
   - Code: `collection_spaces.go` (`HybridSearchSpace`).
   - CLI: `veclite hybrid-search-space <file> <collection> <space>`.
   - HTTP: `POST /collections/{name}/hybrid-search-space`.

3. **Consumer layout-migration recipe** — documented in the VecLite README
   ("Migrating a Collection Layout" section) using only stable public APIs
   (`GetCollection`, `All`, `InsertRecord`, `UpsertRecordByKey`,
   `DropCollection`). No new VecLite methods (`RenameCollection` /
   `MergeCollections` were rejected as too generic or too niche). Vidtrace's
   `migrate-evidence` command (Phase 3) implements this recipe.

**Deliverable:** done. Phase 1 may begin immediately against VecLite
`v0.17.0`; vidtrace's `go.mod` should bump
`github.com/abdul-hamid-achik/veclite v0.16.0 → v0.17.0` as the first
consumer-side change.

### Phase 1 — Single-collection index (consumer, ~1 day)

In vidtrace `internal/evidence/evidence.go`:

1. Replace `KeywordCollection` + `TextCollection` + `MetaCollection` with
   one `CollectionName = "evidence_entries"` constant.
2. Replace `keywordCollection` + `textCollection` + `metaCollection` with
   one `ensureEvidenceCollection(db, withTextSpace bool)` helper.
3. Replace `indexLoadedInto` + `indexSemanticBundle` with one
   `indexLoadedIntoSingle(coll, loaded, embedder)` that:
   - batches embeddings once per bundle (as today),
   - calls `InsertRecord` with `Content`, `Payload`, and `Vectors[text]`
     (or no vectors when `embedder == nil`).
4. Delete `ensureEmbeddingProfile`, `readEmbeddingProfile`,
   `loadEmbeddingProfile` — replace with `coll.VectorSpace(TextVectorSpace)`
   profile reads in the search path.
5. Keep the `IndexReport` / `MultiIndexReport` JSON shapes identical
   (collection name changes from `evidence_entries_keyword` to
   `evidence_entries` — note in CHANGELOG as a breaking change for anyone
   scripting against the JSON, but the vidtrace CLI contract is stable).

**Exit:** `go test ./internal/evidence/...` passes with new tests covering
keyword-only, semantic, and hybrid modes on one collection.

### Phase 2 — Search path (consumer, ~half day)

1. Rewrite `Search` (`evidence.go:490-582`) to use the single collection:
   - `ModeKeyword` → `coll.TextSearch(query, ...)`.
   - `ModeSemantic` → `coll.SearchSpace(TextVectorSpace, queryVec, ...)`.
   - `ModeHybrid` → `coll.HybridSearchSpace(TextVectorSpace, queryVec,
     query, ...)` (the new VecLite `v0.17.0` method; Q2 is resolved).
2. Delete the `indexedProfile` hand-comparison; rely on
   `EmbeddingProfile.Compatible` via the space's profile.
3. Update `searchResultFromPayload` only if the record shape changes (it
   shouldn't — `Result.Record.Payload` is unchanged).

**Exit:** E2E `task e2e` evidence search flows pass; real-video dogfood
(`extract → index → search → investigate`) produces the same ranked results
as before, modulo collection name.

### Phase 3 — Migration tooling (consumer, ~half day)

1. Add `vidtrace migrate-evidence <db>`: detect old three-collection layout,
   read every `evidence_entries_keyword` record, look up the matching
   `evidence_entries_text` vector by `evidence_id`, insert into
   `evidence_entries`, drop the three old collections. Idempotent.
2. Detect old layout on `index`/`search` and print a one-line "run
   `vidtrace migrate-evidence`" hint (not a hard fail).
3. Document in `docs/USAGE.md` and `CHANGELOG.md`.

**Exit:** A pre-existing v0.8-era `.veclite` file migrates cleanly and
`search` works on the result.

### Phase 4 — Docs and ADR (consumer, ~quarter day)

1. Update `docs/adr/0003-use-veclite-for-optional-evidence-search.md` with a
   "Named vector spaces" addendum or a successor ADR referencing this plan.
2. Update `docs/ARCHITECTURE.md` evidence section.
3. Update `docs/USAGE.md` `--embed` section.
4. Note in `CHANGELOG.md` as a breaking change (DB layout) with the
   migration command.

### Phase 5 — Verification

- `go test ./... -race` (vidtrace)
- `task check`, `task smoke`, `task e2e`, `task all`
- Real-video dogfood on `~/Downloads/bug.mp4` and `~/Downloads/OPG-15061.mp4`
- Confirm `vidtrace migrate-evidence` round-trips an old DB.

## Verification Criteria

1. A keyword-only `vidtrace index` (no `--embed`) produces a DB with exactly
   one collection, `evidence_entries`, and zero named vector spaces.
2. `vidtrace index --embed ollama` produces `evidence_entries` with a `text`
   vector space whose `EmbeddingProfile` matches the embedder.
3. `vidtrace search --mode keyword|semantic|hybrid` all work against the
   same collection.
4. Re-running `vidtrace index` on an already-indexed bundle is idempotent
   (no duplicate `evidence_id` records, vectors replaced not duplicated).
5. `vidtrace migrate-evidence` converts an old DB and `search` returns
   identical results before/after.
6. VecLite's named-space API has no gaps that forced a workaround (Q1–Q3
   resolved; any gaps filed as upstream issues).

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Breaking existing `.veclite` files silently | Phase 3 migration tool + detection hint; never auto-migrate |
| `InsertRecord` lacks key-based upsert (Q1) | Resolved in VecLite `v0.17.0` — `UpsertRecordByKey` shipped |
| `HybridSearch` BM25 semantics differ on named spaces (Q2) | Resolved in VecLite `v0.17.0` — `HybridSearchSpace` shipped |
| Performance regression from single collection | Benchmark before/after in Phase 2; HNSW on the `text` space should match or beat the separate `evidence_entries_text` collection |
| JSON `collection` field changes value | Documented in CHANGELOG as breaking; CLI contract otherwise stable |

## Out of Scope

- Image embeddings for frames (would add an `image` named space — natural
  follow-up once this migration lands, but explicitly not now).
- Cross-bundle deduplication of evidence.
- Any change to VecLite itself unless Phase 0 surfaces an API gap.

## Open Questions for VecLite Maintainer

All three questions are **resolved** in VecLite `v0.17.0`:

1. **Upsert by key:** Yes — `UpsertRecordByKey(keyField string, keyValue any, in RecordInput)`
   was added to `collection_spaces.go`, mirroring `UpsertTextDocumentByKey`.
2. **BM25 fusion on named spaces:** Yes — `HybridSearchSpace(space, query []float32, text string, opts ...SearchOption)`
   was added to `collection_spaces.go`, fusing vector results from the named
   space with BM25 text results via RRF.
3. **Migration story:** No new VecLite methods; instead a "Migrating a Collection
   Layout" recipe was added to the README documenting the
   read-transform-insert-drop pattern using existing stable APIs.

---

This plan is the forcing function for vidtrace's v0.17.0 follow-up. Phase 0
is done; the consumer-side work (Phases 1–4) is roughly two days and is
self-contained in the `vidtrace` repo, gated only on bumping
`github.com/abdul-hamid-achik/veclite` from `v0.16.0` to `v0.17.0`.