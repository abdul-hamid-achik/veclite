---
description: "VecLite durability: single-file snapshots, write-ahead log (WAL) with per-batch fsync, crash recovery, auto-checkpointing, and multi-process shared reads."
---

# Durability and the Write-Ahead Log

VecLite persists the whole database as a single snapshot file, written
atomically on `Sync()` and `Close()`. By default, writes made between those
points live only in memory: a crash loses them. The write-ahead log (WAL)
closes that window without paying for a full snapshot on every write.

## Durability modes

| Mode | Cost per write | A crash loses |
|------|----------------|---------------|
| Default | none | everything since the last `Sync`/`Close` |
| `WithWAL(true)` | one small append + fsync | nothing — completed writes are replayed |
| `WithSyncOnWrite(true)` | full snapshot rewrite + fsync | nothing |

`WithSyncOnWrite` rewrites and fsyncs the entire database file on every
mutation, so its cost grows with database size. The WAL appends only the
records a mutation touched, so its cost stays proportional to the write.

## Enabling the WAL

```go
db, err := veclite.Open("vectors.veclite", veclite.WithWAL(true))
```

Or for the HTTP server:

```bash
veclite serve data.veclite --wal
```

With the WAL enabled, every completed mutation — inserts, updates, deletes,
multi-space records, text documents, metadata changes, collection creation and
drops, knowledge-graph and episode-store changes — is appended to a sidecar
file (`vectors.veclite.wal`) before the call returns. Each append batch is
CRC-framed and fsynced once.

## Recovery

On the next `Open` after a crash:

1. The last snapshot is loaded as usual.
2. The log is replayed on top of it, in order. Replay restores records and
   rebuilds the affected HNSW and BM25 index entries, so search works
   immediately.
3. The recovered state is folded into a fresh snapshot and the log is
   truncated.

A clean `Sync()` or `Close()` also truncates the log, so it only ever holds
the writes made since the last snapshot.

## Automatic checkpointing

A long-running writer (like `veclite serve --wal`) might never call `Sync()`
explicitly. To keep the log from growing without bound, the database
automatically folds it into a fresh snapshot once it exceeds a size
threshold — `WithWALCheckpoint(bytes)`, default 64 MiB:

```go
db, err := veclite.Open("vectors.veclite",
    veclite.WithWAL(true),
    veclite.WithWALCheckpoint(16<<20), // checkpoint at 16 MiB
)
```

Pass `0` to disable automatic checkpointing; the log then grows until an
explicit `Sync()` or `Close()`. The checkpoint runs synchronously on the
write that crosses the threshold (that one write pays a full-snapshot save,
like a single `Sync()`).

The log is a *redo log of final record states*: replaying an entry that is
already reflected in the snapshot is a no-op, so recovery is idempotent even
if a previous recovery was interrupted.

## Interaction with other modes

- **Opens without `WithWAL`** still recover and fold a log left behind by a
  crashed WAL-enabled writer, then remove it. Mixing modes never loses data.
- **Read-only opens** (including lock-free `WithSharedRead` readers) replay
  the log in memory without modifying it. `Reload()` re-applies the current
  log, so a shared reader can observe a live writer's not-yet-snapshotted
  writes.
- **`WithSyncOnWrite` + `WithWAL`**: every write saves a full snapshot and
  immediately truncates the log; the WAL adds nothing but costs nothing.
- **In-memory databases** (`:memory:`) ignore the option.

## What the WAL does not cover

- Read-path bookkeeping (record access counts, last-accessed timestamps)
  is not logged — logging on the read path would bloat the file. It persists
  on the next full save.
- A failed log append (disk full, I/O error) does not fail the write: the
  error is logged, the affected records are retried on the next flush, and
  the data still persists on the next successful `Sync`/`Close`. Writes made
  between a failed append and that point would not survive a crash.

Knowledge graphs and episode stores *are* covered: each mutation logs the
graph's or store's full state (they are small relative to record data), which
keeps replay trivially idempotent. Note that graph and episode mutations also
now honor `WithSyncOnWrite`, which previously did not persist them.

## File format

`<db>.wal` starts with a 16-byte header (magic `VECWAL\0\0`, format version)
followed by framed entries: a 4-byte length, a 4-byte CRC32, and a
gob-encoded entry. A torn tail from a crash mid-append fails its CRC and is
discarded on open; everything before it is recovered.
