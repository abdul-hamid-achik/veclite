# CLAUDE.md — Working notes for Claude on VecLite

> **Source of truth:** [`AGENTS.md`](./AGENTS.md) holds the full architecture, conventions,
> lock ordering, and task recipes. Read it first. This file adds Claude-specific orientation
> and the few things easy to get wrong.

## What VecLite is

An embeddable vector database for Go: single-file gob persistence, HNSW index, BM25 text
index, hybrid search, **named vector spaces**, and embedding profiles. It ships a CLI
(`cmd/veclite`) and an HTTP server (`veclite serve`).

## Not a Go-only library

VecLite is usable as a Go library **and** as a standalone engine other languages drive through
the CLI and HTTP server. Language drivers (Python, TypeScript, …) are **planned, not built yet**.
Until they exist, the rule is: the CLI and HTTP **JSON shapes are a public cross-language
contract**. Keep them additive and stable, and back every CLI behavior with a
`specs/glyphrun/` spec so a future driver can be validated against the same expectations.

## The named-vector-space model (the load-bearing idea)

- Every collection has an implicit **`default`** space = `Record.Vector` + the collection's
  primary dimension/distance/index. The whole legacy single-vector API operates on it and is
  unchanged.
- Extra **named** spaces (`AddVectorSpace`) each have their own dimension/distance/HNSW index;
  their vectors live in `Record.Vectors[name]`.
- `InsertRecord(RecordInput{...})` = one logical record with vectors in several spaces.
  `SearchSpace` = one space. `MultiSpaceSearch` / `FuseRRF` = fuse across spaces with RRF.
- Persistence is file-format **v4**; old v1–v3 files migrate additively (default space only).

## Easy to get wrong

- **Backward compatibility is non-negotiable.** Don't change the meaning of `Record.Vector`,
  `Insert`, `Search`, `HybridSearch`, or existing snapshot fields. New capability is additive.
- **CLI flag parsing:** Go's `flag` stops at the first positional arg, so `main()` reorders each
  command's args through `hoistFlags` (in `cmd/veclite/spaces.go`) **before dispatch**. Every
  command therefore accepts `--flags` in any position (before or after `<file> <collection>`).
  New command functions just call `fs.Parse(args)` — do not hoist again (it's idempotent anyway).
- **Storage version bumps:** when the on-disk format changes, update `fileVersion` +
  `migrateSnapshot` in `internal/storage/file.go` **and** `CurrentVersion`/`Migrate` in
  `internal/storage/storage.go` (the root `storage.go` only re-exports types). Migrations must be
  additive (gob tolerates new fields).
- **Locks:** DB → Collection → Index, outermost to innermost (see AGENTS.md). Named-space
  searches take the Collection `RLock`; never call a method that re-locks while holding it.
- **Docs are already hosted — don't add a deploy target.** The `docs/` VitePress site is deployed
  to **Vercel** (`vercel.json` + linked `.vercel` project, auto-deploys on push, served at root).
  There is no GitHub Pages and there should not be — a redundant Pages workflow was added once and
  removed. Build locally with `task site`; new assets go in `docs/.vitepress/public/` (root paths).
- **Check before you build infra.** Before adding any deploy/CI/tooling, look at what exists —
  `vercel.json`, `.vercel`, `.goreleaser.yml`, `glyphrun.config.yml`, `Taskfile.yml`,
  `.github/workflows/` — and extend it rather than introducing a parallel mechanism.
- **Cutting a release:** bump `const Version` in `veclite.go` (and `package.json` for docs), commit,
  then push an annotated `vX.Y.Z` tag — that triggers GoReleaser (`release.yml`). See AGENTS.md
  "Release and Deployment Summary".

## Validate your work

```bash
task lint && task test          # or: go vet ./... && go test -race ./...
go test -race ./...             # named-space + migration tests live in vectorspace_test.go
glyph run specs/glyphrun/cli_named_spaces.yml   # CLI behavior contract
glyph run specs/glyphrun/cli_fuse_search.yml
```

Keep `README.md`, the `docs/` site, and `AGENTS.md` in sync with any API or CLI change.
