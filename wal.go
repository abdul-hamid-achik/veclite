package veclite

import (
	"errors"
	"os"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// VecLite's durability model is snapshot-based: the whole database is written
// on Sync/Close. The write-ahead log closes the crash window between
// snapshots. When enabled with WithWAL, every completed mutation appends the
// affected records' final state to a sidecar log (dbpath + ".wal") with one
// fsync per operation batch — far cheaper than WithSyncOnWrite's full-snapshot
// save per write. On open, any log left behind by a crash is replayed on top
// of the last snapshot and folded into a fresh one. A successful Sync/Close
// truncates the log.
//
// The log is a redo log of final record states, so replay is idempotent:
// entries already contained in the snapshot apply as no-ops.
//
// Knowledge graphs and episode stores are logged as full-state entries on
// each mutation (they are small relative to record data). Not covered by the
// WAL (persisted only on full Sync/Close): read-path bookkeeping such as
// access counts.

// walActive reports whether this DB is appending mutations to a WAL.
func (db *DB) walActive() bool {
	return db.walOn.Load()
}

// initWAL runs during Open, after the base snapshot is loaded and before the
// DB is returned to the caller (no concurrent access yet). It replays any
// leftover log from a crashed writer regardless of whether this open
// requested WAL support — otherwise the open would silently serve stale data.
func (db *DB) initWAL() error {
	walPath := storage.WALPath(db.path)

	// Read-only opens (including lock-free shared readers) replay in memory
	// only: they must not write, truncate, or remove the log.
	if db.config.readOnly {
		entries, err := storage.ReadWALEntries(walPath)
		if err != nil {
			return err
		}
		db.applyWALEntries(entries)
		return nil
	}

	if db.config.wal {
		wal, entries, err := storage.OpenWAL(walPath)
		if err != nil {
			return err
		}
		db.applyWALEntries(entries)
		if len(entries) > 0 {
			// Fold recovered state into a fresh snapshot so the log stays short.
			if err := db.storage.Save(db.snapshotLocked()); err != nil {
				_ = wal.Close()
				return err
			}
			if err := wal.Reset(); err != nil {
				_ = wal.Close()
				return err
			}
		}
		db.wal = wal
		db.walOn.Store(true)
		return nil
	}

	// WAL not requested: fold and remove a leftover log so its data is not
	// lost when this (non-WAL) writer's first Save replaces the snapshot.
	if _, err := os.Stat(walPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	wal, entries, err := storage.OpenWAL(walPath)
	if err != nil {
		return err
	}
	db.applyWALEntries(entries)
	if len(entries) > 0 {
		if err := db.storage.Save(db.snapshotLocked()); err != nil {
			_ = wal.Close()
			return err
		}
	}
	return wal.Remove()
}

// applyWALEntries replays entries in log order onto the in-memory state.
// Callers must guarantee exclusive access (Open before publish, or db.mu held
// as in Reload).
func (db *DB) applyWALEntries(entries []storage.WALEntry) {
	for i := range entries {
		e := &entries[i]
		switch e.Op {
		case storage.WALOpDBMetadata:
			db.metadata = deepCopyMap(e.Metadata)
			if db.metadata == nil {
				db.metadata = make(map[string]any)
			}
		case storage.WALOpDropCollection:
			delete(db.collections, e.Collection)
		case storage.WALOpCollectionConfig:
			if e.Config != nil {
				db.walReplayCollection(e.Collection).applyWALConfig(e.Config)
			}
		case storage.WALOpUpsertRecord:
			if e.Record != nil {
				db.walReplayCollection(e.Collection).replayWALUpsert(e.Record)
			}
		case storage.WALOpDeleteRecord:
			if coll, ok := db.collections[e.Collection]; ok {
				coll.replayWALDelete(e.RecordID)
			}
		case storage.WALOpGraph:
			if e.Graph != nil {
				db.replayWALGraph(e.Collection, e.Graph)
			}
		case storage.WALOpEpisodeStore:
			if e.Episodes != nil {
				db.replayWALEpisodeStore(e.Collection, e.Episodes)
			}
		}
	}
}

// replayWALGraph installs a logged knowledge-graph state, reusing the live
// graph struct when the base snapshot already created one.
func (db *DB) replayWALGraph(name string, snap *storage.GraphSnapshot) {
	kg, ok := db.knowledgeGraphs[name]
	if !ok {
		coll := db.walReplayCollection("_kg_" + name)
		kg = &KnowledgeGraph{
			db:            db,
			name:          name,
			entities:      make(map[string]*Entity),
			relationships: make(map[string]*Relationship),
			outgoing:      make(map[string][]string),
			incoming:      make(map[string][]string),
			collection:    coll,
			distanceFunc:  floats.GetDistanceFunc(coll.distanceType),
			higherBetter:  floats.IsHigherBetter(coll.distanceType),
		}
		db.knowledgeGraphs[name] = kg
	}
	kg.loadFromSnapshot(snap)
}

// replayWALEpisodeStore installs a logged episode-store state.
func (db *DB) replayWALEpisodeStore(name string, snap *storage.EpisodeStoreSnapshot) {
	es, ok := db.episodeStores[name]
	if !ok {
		coll := db.walReplayCollection(snap.CollectionName)
		es = &EpisodeStore{
			collection:   coll,
			episodes:     make(map[string]*Episode),
			distanceFunc: floats.GetDistanceFunc(coll.distanceType),
			higherBetter: floats.IsHigherBetter(coll.distanceType),
		}
		db.episodeStores[name] = es
	}
	es.loadFromSnapshot(snap)
}

// walReplayCollection returns the named collection, creating a bare one for
// replay if it does not exist (its config arrives via a config entry or is
// inferred from replayed records).
func (db *DB) walReplayCollection(name string) *Collection {
	if coll, ok := db.collections[name]; ok {
		return coll
	}
	coll := newCollection(name, defaultCollectionConfig(), db)
	db.collections[name] = coll
	return coll
}

// applyWALConfig merges a logged collection configuration into the live
// collection. It never touches records; replayed upserts carry those.
func (c *Collection) applyWALConfig(cs *storage.CollectionSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cs.Dimension > 0 {
		c.dimension = cs.Dimension
	}
	c.distanceType = cs.DistanceType
	c.distanceFunc = floats.GetDistanceFunc(cs.DistanceType)
	c.higherBetter = floats.IsHigherBetter(cs.DistanceType)
	if cs.NextID > c.nextID {
		c.nextID = cs.NextID
	}
	c.metadata = deepCopyMap(cs.Metadata)
	if c.metadata == nil {
		c.metadata = make(map[string]any)
	}
	if cs.IndexType != "" {
		c.indexType = IndexType(cs.IndexType)
	}
	if cs.HNSWConfig != nil {
		c.hnswConfig = cs.HNSWConfig
	}
	c.profile = profileFromSnapshot(cs.EmbeddingProfile)

	for _, vss := range cs.VectorSpaces {
		if vss == nil || vss.Name == "" || vss.Name == DefaultVectorSpace {
			continue
		}
		if sp, ok := c.spaces[vss.Name]; ok {
			if sp.dimension == 0 && vss.Dimension > 0 {
				sp.dimension = vss.Dimension
				c.initSpaceIndexIfNeeded(sp)
			}
			sp.profile = profileFromSnapshot(vss.Profile)
			continue
		}
		sp := &vectorSpace{
			name:         vss.Name,
			dimension:    vss.Dimension,
			distanceType: vss.DistanceType,
			distanceFunc: floats.GetDistanceFunc(vss.DistanceType),
			higherBetter: floats.IsHigherBetter(vss.DistanceType),
			modality:     vss.Modality,
			provider:     vss.Provider,
			model:        vss.Model,
			indexType:    IndexType(vss.IndexType),
			hnswConfig:   vss.HNSWConfig,
			profile:      profileFromSnapshot(vss.Profile),
		}
		c.spaces[sp.name] = sp
		c.initSpaceIndexIfNeeded(sp)
	}

	if c.textIndex == nil && cs.TextIndexSnapshot != nil && len(cs.TextIndexSnapshot.Fields) > 0 {
		c.textIndex = newInvertedIndex(cs.TextIndexSnapshot.Fields)
	}

	c.initHNSWIfNeeded()
}

// replayWALUpsert applies a logged record state: any previous version is
// removed from all indexes, then the logged state is inserted and re-indexed.
func (c *Collection) replayWALUpsert(rs *storage.RecordSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := rs.ID
	rec := recordFromWALSnapshot(rs)

	// Validate before touching existing state so a bad entry skips cleanly
	// instead of destroying the prior version of the record.
	if len(rec.Vector) > 0 {
		if c.dimension == 0 {
			c.dimension = len(rec.Vector)
			c.initHNSWIfNeeded()
		} else if len(rec.Vector) != c.dimension {
			// Entries were dimension-checked when written; a mismatch means the
			// log does not belong to this collection state. Skip rather than
			// corrupt the index.
			if c.db != nil && c.db.logger != nil {
				c.db.logger.Error("veclite: skipping WAL record with mismatched dimension",
					"collection", c.name, "id", id, "got", len(rec.Vector), "want", c.dimension)
			}
			return
		}
	}

	if old, ok := c.records[id]; ok {
		if c.index != nil && len(old.Vector) > 0 {
			c.hardDeleteFromIndex(id)
		}
		c.removeFromSpaceIndexesLocked(id, old)
		if c.textIndex != nil {
			c.textIndex.removeRecord(id)
		}
		delete(c.records, id)
	}

	c.records[id] = rec
	if id >= c.nextID {
		c.nextID = id + 1
	}

	if c.index != nil && len(rec.Vector) > 0 {
		_ = c.index.Insert(id, rec.Vector)
	}
	for name, vec := range rec.Vectors {
		sp, ok := c.spaces[name]
		if !ok || len(vec) == 0 {
			continue
		}
		if sp.dimension == 0 {
			sp.dimension = len(vec)
			c.initSpaceIndexIfNeeded(sp)
		}
		if sp.index != nil && len(vec) == sp.dimension {
			_ = sp.index.Insert(id, vec)
		}
	}
	c.reindexRecordLocked(rec)
}

// replayWALDelete applies a logged record deletion.
func (c *Collection) replayWALDelete(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, ok := c.records[id]
	if !ok {
		return
	}
	if c.index != nil {
		_ = c.index.Delete(id)
	}
	c.removeFromSpaceIndexesLocked(id, record)
	if c.textIndex != nil {
		c.textIndex.removeRecord(id)
	}
	delete(c.records, id)
}

// --- dirty tracking -------------------------------------------------------
//
// Mutation paths mark affected record IDs under c.mu; syncIfNeeded drains the
// marks into WAL entries after the mutation completes. Because entries carry
// the record's state at drain time, the log stays correct under concurrent
// writers: replaying final states in append order is idempotent.

// markWALUpsertLocked records that a record was inserted or modified.
// The caller must hold c.mu for writing.
func (c *Collection) markWALUpsertLocked(id uint64) {
	if c.db == nil || !c.db.walActive() {
		return
	}
	if c.walDirty == nil {
		c.walDirty = make(map[uint64]struct{})
	}
	c.walDirty[id] = struct{}{}
	delete(c.walDeleted, id)
}

// markWALDeleteLocked records that a record was removed.
// The caller must hold c.mu for writing.
func (c *Collection) markWALDeleteLocked(id uint64) {
	if c.db == nil || !c.db.walActive() {
		return
	}
	if c.walDeleted == nil {
		c.walDeleted = make(map[uint64]struct{})
	}
	c.walDeleted[id] = struct{}{}
	delete(c.walDirty, id)
}

// markWALConfigLocked records that collection configuration changed
// (metadata, profile, vector spaces). The caller must hold c.mu for writing.
func (c *Collection) markWALConfigLocked() {
	if c.db == nil || !c.db.walActive() {
		return
	}
	c.walConfigDirty = true
}

// flushWAL drains this collection's dirty marks into the log. Called from
// syncIfNeeded after each completed mutation; a no-op when the WAL is off or
// nothing is dirty. Append failures are logged, not returned: the data is
// still in memory and persists on the next successful Sync/Close.
func (c *Collection) flushWAL() {
	db := c.db
	if db == nil || !db.walActive() {
		return
	}
	db.walMu.Lock()
	defer db.walMu.Unlock()

	entries := c.drainWALEntries()
	if len(entries) == 0 {
		return
	}
	if err := db.wal.Append(entries); err != nil {
		// Restore the drained marks so the next flush retries these records;
		// the data also reaches disk on the next successful Sync/Close.
		db.logger.Error("veclite: WAL append failed; will retry on next write or Sync/Close", "error", err)
		c.remarkWALEntries(entries)
	}
}

// remarkWALEntries puts the dirty marks for failed-to-append entries back so
// a later flush retries them. Duplicate retried entries are harmless: replay
// applies final states idempotently.
func (c *Collection) remarkWALEntries(entries []storage.WALEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range entries {
		switch entries[i].Op {
		case storage.WALOpCollectionConfig:
			c.walConfigDirty = true
		case storage.WALOpUpsertRecord:
			if entries[i].Record != nil {
				if c.walDirty == nil {
					c.walDirty = make(map[uint64]struct{})
				}
				c.walDirty[entries[i].Record.ID] = struct{}{}
			}
		case storage.WALOpDeleteRecord:
			if c.walDeleted == nil {
				c.walDeleted = make(map[uint64]struct{})
			}
			c.walDeleted[entries[i].RecordID] = struct{}{}
		}
	}
}

// drainWALEntries converts and clears the dirty marks. Marks are verified
// against the live records map so an insert-then-delete sequence yields a
// single delete entry.
func (c *Collection) drainWALEntries() []storage.WALEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.walConfigDirty && len(c.walDirty) == 0 && len(c.walDeleted) == 0 {
		return nil
	}

	entries := make([]storage.WALEntry, 0, 1+len(c.walDirty)+len(c.walDeleted))
	if c.walConfigDirty {
		entries = append(entries, storage.WALEntry{
			Op:         storage.WALOpCollectionConfig,
			Collection: c.name,
			Config:     c.walConfigSnapshotLocked(),
		})
		c.walConfigDirty = false
	}
	for id := range c.walDirty {
		rec, ok := c.records[id]
		if !ok {
			continue // deleted after being dirtied; walDeleted covers it
		}
		entries = append(entries, storage.WALEntry{
			Op:         storage.WALOpUpsertRecord,
			Collection: c.name,
			Record:     recordToWALSnapshot(rec),
		})
	}
	c.walDirty = nil
	for id := range c.walDeleted {
		if _, ok := c.records[id]; ok {
			continue // re-inserted after deletion; the upsert above covers it
		}
		entries = append(entries, storage.WALEntry{
			Op:         storage.WALOpDeleteRecord,
			Collection: c.name,
			RecordID:   id,
		})
	}
	c.walDeleted = nil
	return entries
}

// walMarks holds drained dirty state so a failed Save can restore it.
type walMarks struct {
	coll    *Collection
	dirty   map[uint64]struct{}
	deleted map[uint64]struct{}
	config  bool
}

// snapshotClearingWAL snapshots the collection and clears its dirty marks in
// one critical section. This atomicity is what keeps concurrent mutations
// from falling between snapshot and log: a mutation either lands before the
// snapshot is taken (and is cleared here) or after it (and its marks survive
// to be appended once the post-Save log truncation is done).
func (c *Collection) snapshotClearingWAL() (*storage.CollectionSnapshot, walMarks) {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := c.snapshotLocked()
	marks := walMarks{coll: c, dirty: c.walDirty, deleted: c.walDeleted, config: c.walConfigDirty}
	c.walDirty = nil
	c.walDeleted = nil
	c.walConfigDirty = false
	return snap, marks
}

// restoreWALMarks merges drained marks back after a failed Save so the
// affected records are re-logged by the next flush.
func (c *Collection) restoreWALMarks(m walMarks) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m.config {
		c.walConfigDirty = true
	}
	if len(m.dirty) > 0 {
		if c.walDirty == nil {
			c.walDirty = make(map[uint64]struct{}, len(m.dirty))
		}
		for id := range m.dirty {
			c.walDirty[id] = struct{}{}
		}
	}
	if len(m.deleted) > 0 {
		if c.walDeleted == nil {
			c.walDeleted = make(map[uint64]struct{}, len(m.deleted))
		}
		for id := range m.deleted {
			c.walDeleted[id] = struct{}{}
		}
	}
}

// walConfigSnapshotLocked builds a records-free collection snapshot carrying
// only configuration. The caller must hold c.mu.
func (c *Collection) walConfigSnapshotLocked() *storage.CollectionSnapshot {
	cs := &storage.CollectionSnapshot{
		Name:             c.name,
		Metadata:         deepCopyMap(c.metadata),
		Dimension:        c.dimension,
		DistanceType:     c.distanceType,
		NextID:           c.nextID,
		IndexType:        string(c.indexType),
		HNSWConfig:       c.hnswConfig,
		EmbeddingProfile: c.profile.toSnapshot(),
	}
	if len(c.spaces) > 0 {
		cs.VectorSpaces = make([]*storage.VectorSpaceSnapshot, 0, len(c.spaces))
		for _, sp := range c.spaces {
			cs.VectorSpaces = append(cs.VectorSpaces, &storage.VectorSpaceSnapshot{
				Name:         sp.name,
				Dimension:    sp.dimension,
				DistanceType: sp.distanceType,
				Modality:     sp.modality,
				Provider:     sp.provider,
				Model:        sp.model,
				IndexType:    string(sp.indexType),
				HNSWConfig:   sp.hnswConfig,
				Profile:      sp.profile.toSnapshot(),
			})
		}
	}
	if c.textIndex != nil {
		// Fields only: replayed upserts rebuild the postings via re-indexing.
		cs.TextIndexSnapshot = &storage.InvertedIndexSnapshot{Fields: c.textIndex.fields}
	}
	return cs
}

// walLogNewCollection logs a just-created collection's configuration so an
// empty collection survives a crash. The caller must hold db.mu.
func (db *DB) walLogNewCollection(coll *Collection) {
	if !db.walActive() || db.config.syncOnWrite {
		return
	}
	coll.mu.Lock()
	coll.walConfigDirty = true
	coll.mu.Unlock()
	coll.flushWAL()
}

// walAppendDB appends database-level entries (metadata, collection drops).
// The caller must hold db.mu. No-op when the WAL is off or every write
// already saves a full snapshot.
func (db *DB) walAppendDB(entries ...storage.WALEntry) {
	if !db.walActive() || db.config.syncOnWrite {
		return
	}
	db.walMu.Lock()
	defer db.walMu.Unlock()
	if err := db.wal.Append(entries); err != nil {
		db.logger.Error("veclite: WAL append failed; changes reach disk on next Sync/Close", "error", err)
	}
}

// walAppendGraph logs a knowledge graph's full post-mutation state. Called
// after graph mutations with no graph lock held (snapshot takes it; the
// ordering is walMu → kg.mu, matching syncLocked).
func (db *DB) walAppendGraph(kg *KnowledgeGraph) {
	if !db.walActive() || db.config.syncOnWrite {
		return
	}
	db.walMu.Lock()
	defer db.walMu.Unlock()
	entry := storage.WALEntry{Op: storage.WALOpGraph, Collection: kg.name, Graph: kg.snapshot()}
	if err := db.wal.Append([]storage.WALEntry{entry}); err != nil {
		db.logger.Error("veclite: WAL append failed; changes reach disk on next Sync/Close", "error", err)
	}
}

// walAppendEpisodeStore logs an episode store's full post-mutation state.
// name is the store's key in the DB registry.
func (db *DB) walAppendEpisodeStore(name string, es *EpisodeStore) {
	if !db.walActive() || db.config.syncOnWrite {
		return
	}
	db.walMu.Lock()
	defer db.walMu.Unlock()
	entry := storage.WALEntry{Op: storage.WALOpEpisodeStore, Collection: name, Episodes: es.snapshot()}
	if err := db.wal.Append([]storage.WALEntry{entry}); err != nil {
		db.logger.Error("veclite: WAL append failed; changes reach disk on next Sync/Close", "error", err)
	}
}

// maybeCheckpointWAL folds the log into a fresh snapshot when it has grown
// past the configured threshold, bounding log size for long-running writers
// that never call Sync explicitly. It must be called with NO locks held
// (Sync acquires db.mu → walMu → c.mu/kg.mu/es.mu); the syncIfNeeded
// helpers call it after their flush, all from lock-free contexts. A CAS
// gate keeps concurrent writers from stampeding into redundant full saves.
func (db *DB) maybeCheckpointWAL() {
	limit := db.config.walCheckpoint
	if limit <= 0 || !db.walActive() || db.wal.Size() < limit {
		return
	}
	if !db.walCheckpointing.CompareAndSwap(false, true) {
		return
	}
	defer db.walCheckpointing.Store(false)

	// Re-check under the gate: a Sync that just completed may have truncated.
	if db.wal.Size() < limit {
		return
	}
	if err := db.Sync(); err != nil && !errors.Is(err, ErrDatabaseClosed) {
		db.logger.Error("veclite: WAL checkpoint failed; log keeps growing until next successful Sync/Close", "error", err)
	}
}

// recordToWALSnapshot deep-copies a record into a snapshot so log encoding
// (which happens outside the collection lock) cannot race with later
// mutations of the live record.
func recordToWALSnapshot(r *Record) *storage.RecordSnapshot {
	rs := &storage.RecordSnapshot{
		ID:             r.ID,
		Payload:        deepCopyMap(r.Payload),
		Content:        r.Content,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		ExpiresAt:      r.ExpiresAt,
		Importance:     r.Importance,
		AccessCount:    r.AccessCount,
		LastAccessedAt: r.LastAccessedAt,
	}
	if len(r.Vector) > 0 {
		rs.Vector = make([]float32, len(r.Vector))
		copy(rs.Vector, r.Vector)
	}
	if len(r.Vectors) > 0 {
		rs.Vectors = make(map[string][]float32, len(r.Vectors))
		for name, vec := range r.Vectors {
			cp := make([]float32, len(vec))
			copy(cp, vec)
			rs.Vectors[name] = cp
		}
	}
	return rs
}

// recordFromWALSnapshot rebuilds a record from a logged snapshot. The
// snapshot is owned by replay, so its slices and maps are adopted directly.
func recordFromWALSnapshot(rs *storage.RecordSnapshot) *Record {
	var vectors map[string][]float32
	if len(rs.Vectors) > 0 {
		vectors = make(map[string][]float32, len(rs.Vectors))
		for name, vec := range rs.Vectors {
			vectors[name] = vec
		}
	}
	return &Record{
		ID:             rs.ID,
		Vector:         rs.Vector,
		Vectors:        vectors,
		Payload:        rs.Payload,
		Content:        rs.Content,
		CreatedAt:      rs.CreatedAt,
		UpdatedAt:      rs.UpdatedAt,
		ExpiresAt:      rs.ExpiresAt,
		Importance:     rs.Importance,
		AccessCount:    rs.AccessCount,
		LastAccessedAt: rs.LastAccessedAt,
	}
}
