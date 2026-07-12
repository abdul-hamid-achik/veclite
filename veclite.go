// Package veclite provides an embeddable vector database for Go.
// It stores vectors with metadata in a single file using gob encoding.
//
// Basic usage:
//
//	db, err := veclite.Open("data.veclite")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
//
//	coll := db.Collection("embeddings")
//	id, err := coll.Insert(vector, map[string]any{"file": "main.go"})
//
//	results, err := coll.Search(queryVector, veclite.TopK(10))
package veclite

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// Version is the library version.
const Version = "0.23.1"

// DB represents a VecLite database.
type DB struct {
	path        string
	storage     storage.Backend
	config      *dbConfig
	metadata    map[string]any
	collections map[string]*Collection
	mu          sync.RWMutex
	closed      bool
	createdAt   time.Time
	updatedAt   time.Time
	metrics     *Metrics
	logger      Logger

	// knowledgeGraphs tracks created knowledge graphs for persistence.
	knowledgeGraphs map[string]*KnowledgeGraph
	// episodeStores tracks created episode stores for persistence.
	episodeStores map[string]*EpisodeStore

	// subscriptions manages per-collection subscription managers.
	subscriptions map[string]*subscriptionManager
	subMu         sync.Mutex

	// stopFuncs holds stop functions for background workers (TTLCleaner, MemoryLimiter).
	stopFuncs []func()
	stopMu    sync.Mutex

	// Write-ahead log state (see wal.go). wal is set once during Open and never
	// mutated afterwards; walOn gates the hot-path dirty marking; walMu
	// serializes log appends against snapshot-save + log-truncate sequences.
	wal   *storage.WAL
	walOn atomic.Bool
	walMu sync.Mutex
	// walCheckpointing gates automatic WAL checkpoints so concurrent writers
	// don't stampede into redundant full-snapshot saves.
	walCheckpointing atomic.Bool
}

// Open opens or creates a VecLite database at the given path.
// Use ":memory:" for an in-memory database that won't be persisted.
func Open(path string, opts ...Option) (*DB, error) {
	config := defaultDBConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	var store storage.Backend
	if path == ":memory:" {
		store = storage.NewMemory()
	} else {
		fs := storage.NewFile(path)
		// Acquire file lock to prevent concurrent access.
		// Use a shared (read) lock for read-only databases opened with
		// WithSharedRead, allowing multiple read-only processes to coexist.
		// Use the default exclusive lock otherwise.
		if config.sharedRead {
			if !config.readOnly {
				return nil, fmt.Errorf("veclite: %w", ErrSharedReadRequiresReadOnly)
			}
			// Lock-free read-only: take no persistent flock so long-lived
			// readers (MCP servers, daemons' query clients) never block a
			// writer, and a writer's exclusive lock never blocks read-only
			// opens. Consistency rests on Save's atomic replace; callers
			// Reload() to pick up external writes.
			fs.UseTransientShared()
		} else {
			if err := fs.Lock(); err != nil {
				return nil, err
			}
		}
		store = fs
	}

	logger := config.logger
	if logger == nil {
		logger = NopLogger{}
	}

	db := &DB{
		path:            path,
		storage:         store,
		config:          config,
		metadata:        make(map[string]any),
		collections:     make(map[string]*Collection),
		knowledgeGraphs: make(map[string]*KnowledgeGraph),
		episodeStores:   make(map[string]*EpisodeStore),
		createdAt:       time.Now(),
		updatedAt:       time.Now(),
		metrics:         newMetrics(),
		logger:          logger,
		subscriptions:   make(map[string]*subscriptionManager),
	}

	// Load existing data
	snapshot, err := store.Load()
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	if snapshot != nil {
		db.loadFromSnapshot(snapshot)
	}

	// Replay and (for writers) attach the write-ahead log. This also folds a
	// log left behind by a crashed WAL-enabled writer, whether or not this
	// open requested the WAL itself.
	if path != ":memory:" {
		if err := db.initWAL(); err != nil {
			_ = store.Close()
			return nil, err
		}
	}

	return db, nil
}

// Close closes the database, syncing any pending changes.
func (db *DB) Close() error {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return ErrDatabaseClosed
	}

	db.closed = true
	readOnly := db.config.readOnly
	db.mu.Unlock()

	// Stop all background workers before taking the DB lock for snapshotting.
	// Workers may need DB read locks while they shut down.
	db.stopMu.Lock()
	stopFuncs := append([]func(){}, db.stopFuncs...)
	db.stopFuncs = nil
	db.stopMu.Unlock()

	for _, stop := range stopFuncs {
		stop()
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Sync before closing unless the database was opened read-only.
	var syncErr error
	if !readOnly {
		syncErr = db.syncLocked()
	}

	// Always release storage (and file lock), even if sync failed
	closeErr := db.storage.Close()

	var walErr error
	if db.wal != nil {
		walErr = db.wal.Close()
	}

	return errors.Join(syncErr, closeErr, walErr)
}

// registerStopFunc registers a function to be called when the database is closed.
// Used by background workers to ensure they are stopped on Close().
func (db *DB) registerStopFunc(stop func()) {
	db.stopMu.Lock()
	defer db.stopMu.Unlock()
	db.stopFuncs = append(db.stopFuncs, stop)
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// Metadata returns a deep copy of the database metadata.
func (db *DB) Metadata() map[string]any {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return deepCopyMap(db.metadata)
}

// SetMetadata replaces the database metadata.
func (db *DB) SetMetadata(metadata map[string]any) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}
	if db.config.readOnly {
		return ErrReadOnly
	}

	db.metadata = deepCopyMap(metadata)
	if db.metadata == nil {
		db.metadata = make(map[string]any)
	}
	if db.config.syncOnWrite {
		return db.syncLocked()
	}
	db.walAppendDB(storage.WALEntry{Op: storage.WALOpDBMetadata, Metadata: deepCopyMap(db.metadata)})
	return nil
}

// SetMetadataValue sets one database metadata value.
func (db *DB) SetMetadataValue(key string, value any) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}
	if db.config.readOnly {
		return ErrReadOnly
	}

	if db.metadata == nil {
		db.metadata = make(map[string]any)
	}
	db.metadata[key] = deepCopyValue(value)
	if db.config.syncOnWrite {
		return db.syncLocked()
	}
	db.walAppendDB(storage.WALEntry{Op: storage.WALOpDBMetadata, Metadata: deepCopyMap(db.metadata)})
	return nil
}

// DeleteMetadataValue removes one database metadata value.
func (db *DB) DeleteMetadataValue(key string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}
	if db.config.readOnly {
		return ErrReadOnly
	}

	delete(db.metadata, key)
	if db.config.syncOnWrite {
		return db.syncLocked()
	}
	db.walAppendDB(storage.WALEntry{Op: storage.WALOpDBMetadata, Metadata: deepCopyMap(db.metadata)})
	return nil
}

// Collection returns a collection by name, creating it if it doesn't exist.
// This is the preferred way to get collections for most use cases.
// In read-only mode, returns nil if the collection doesn't exist (cannot create).
func (db *DB) Collection(name string) *Collection {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	if coll, ok := db.collections[name]; ok {
		return coll
	}

	// In read-only mode, do not create new collections
	if db.config.readOnly {
		return nil
	}

	// Create new collection with defaults
	coll := newCollection(name, defaultCollectionConfig(), db)
	db.collections[name] = coll
	db.walLogNewCollection(coll)
	return coll
}

// CreateCollection creates a new collection with the given options.
// Returns an error if the collection already exists.
func (db *DB) CreateCollection(name string, opts ...CollectionOption) (*Collection, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}

	if db.config.readOnly {
		return nil, ErrReadOnly
	}

	if _, ok := db.collections[name]; ok {
		return nil, ErrCollectionExists
	}

	config := defaultCollectionConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	coll := newCollection(name, config, db)
	db.collections[name] = coll
	db.walLogNewCollection(coll)
	return coll, nil
}

// GetCollection returns an existing collection or ErrNotFound.
func (db *DB) GetCollection(name string) (*Collection, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}

	coll, ok := db.collections[name]
	if !ok {
		return nil, &NotFoundError{Type: "collection", ID: name}
	}
	return coll, nil
}

// DropCollection removes a collection and all its data.
func (db *DB) DropCollection(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.config.readOnly {
		return ErrReadOnly
	}

	if _, ok := db.collections[name]; !ok {
		return &NotFoundError{Type: "collection", ID: name}
	}

	delete(db.collections, name)
	db.walAppendDB(storage.WALEntry{Op: storage.WALOpDropCollection, Collection: name})
	return nil
}

// Collections returns the names of all collections.
func (db *DB) Collections() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	names := make([]string, 0, len(db.collections))
	for name := range db.collections {
		names = append(names, name)
	}
	return names
}

// HasCollection returns true if a collection exists.
func (db *DB) HasCollection(name string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	_, ok := db.collections[name]
	return ok
}

// Sync writes all pending changes to storage.
func (db *DB) Sync() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.config.readOnly {
		return ErrReadOnly
	}

	return db.syncLocked()
}

// syncLocked performs sync while holding the lock.
func (db *DB) syncLocked() error {
	if !db.walActive() {
		return db.storage.Save(db.snapshotLocked())
	}

	// With a WAL, the save and the log truncation must be one atomic unit with
	// respect to log appends: walMu blocks flushes so that a mutation racing
	// this sync either lands in the snapshot (its marks are cleared with it)
	// or appends to the log after the truncation.
	db.walMu.Lock()
	defer db.walMu.Unlock()

	snapshot := &storage.DatabaseSnapshot{
		Version:         storage.CurrentVersion,
		Metadata:        deepCopyMap(db.metadata),
		Collections:     make(map[string]*storage.CollectionSnapshot, len(db.collections)),
		KnowledgeGraphs: make(map[string]*storage.GraphSnapshot, len(db.knowledgeGraphs)),
		EpisodeStores:   make(map[string]*storage.EpisodeStoreSnapshot, len(db.episodeStores)),
		CreatedAt:       db.createdAt,
		UpdatedAt:       time.Now(),
	}

	drained := make([]walMarks, 0, len(db.collections))
	for name, coll := range db.collections {
		snap, marks := coll.snapshotClearingWAL()
		snapshot.Collections[name] = snap
		drained = append(drained, marks)
	}
	for name, kg := range db.knowledgeGraphs {
		snapshot.KnowledgeGraphs[name] = kg.snapshot()
	}
	for name, es := range db.episodeStores {
		snapshot.EpisodeStores[name] = es.snapshot()
	}

	if err := db.storage.Save(snapshot); err != nil {
		// The snapshot never hit disk: restore the drained marks so the
		// affected records are re-logged by the next flush. Already-appended
		// entries are still in the log (no truncation happened).
		for _, marks := range drained {
			marks.coll.restoreWALMarks(marks)
		}
		return err
	}

	// A failed truncation is not fatal: the stale entries replay
	// idempotently over the snapshot that now already contains them.
	if err := db.wal.Reset(); err != nil {
		db.logger.Error("veclite: WAL truncate after save failed", "error", err)
	}
	return nil
}

// snapshotLocked creates a snapshot while holding the lock.
func (db *DB) snapshotLocked() *storage.DatabaseSnapshot {
	snapshot := &storage.DatabaseSnapshot{
		Version:         storage.CurrentVersion,
		Metadata:        deepCopyMap(db.metadata),
		Collections:     make(map[string]*storage.CollectionSnapshot, len(db.collections)),
		KnowledgeGraphs: make(map[string]*storage.GraphSnapshot, len(db.knowledgeGraphs)),
		EpisodeStores:   make(map[string]*storage.EpisodeStoreSnapshot, len(db.episodeStores)),
		CreatedAt:       db.createdAt,
		UpdatedAt:       time.Now(),
	}

	for name, coll := range db.collections {
		snapshot.Collections[name] = coll.snapshot()
	}

	for name, kg := range db.knowledgeGraphs {
		snapshot.KnowledgeGraphs[name] = kg.snapshot()
	}

	for name, es := range db.episodeStores {
		snapshot.EpisodeStores[name] = es.snapshot()
	}

	return snapshot
}

// loadFromSnapshot restores the database from a snapshot.
func (db *DB) loadFromSnapshot(snapshot *storage.DatabaseSnapshot) {
	// Upgrade older on-disk formats to the current version. The named-vector-space
	// migration (v4) is additive: existing single-vector collections keep working
	// as the implicit "default" space, so this never rewrites record data.
	snapshot = storage.Migrate(snapshot)

	db.createdAt = snapshot.CreatedAt
	db.updatedAt = snapshot.UpdatedAt
	db.metadata = deepCopyMap(snapshot.Metadata)
	if db.metadata == nil {
		db.metadata = make(map[string]any)
	}

	for name, collSnapshot := range snapshot.Collections {
		coll := newCollection(name, defaultCollectionConfig(), db)
		coll.loadFromSnapshot(collSnapshot)
		db.collections[name] = coll
	}

	for name, graphSnap := range snapshot.KnowledgeGraphs {
		collName := "_kg_" + name
		coll := db.Collection(collName)
		kg := &KnowledgeGraph{
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
		kg.loadFromSnapshot(graphSnap)
		db.knowledgeGraphs[name] = kg
	}

	for name, epSnap := range snapshot.EpisodeStores {
		coll := db.Collection(epSnap.CollectionName)
		es := &EpisodeStore{
			collection:   coll,
			episodes:     make(map[string]*Episode),
			distanceFunc: floats.GetDistanceFunc(coll.distanceType),
			higherBetter: floats.IsHigherBetter(coll.distanceType),
		}
		es.loadFromSnapshot(epSnap)
		db.episodeStores[name] = es
	}
}

// Reload re-reads the database from storage, rebuilding all in-memory state
// (collections, HNSW indexes, BM25 inverted indexes, knowledge graphs,
// episode stores). It is intended for read-only databases opened with
// WithSharedRead so they can pick up writes performed by another process
// without closing and reopening.
//
// Reload acquires the DB write lock for the duration of the rebuild. It is not
// safe to call concurrently with reads or writes on the same DB instance.
//
// Background workers (TTL cleaner, memory limiter) are stopped before reload
// because they reference the old collection structs that are replaced during
// reload; callers that need those workers must restart them after Reload
// returns.
//
// Reload returns an error if the database is closed, if the storage backend
// does not support reloading (e.g. in-memory), or if the reloaded snapshot is
// corrupt. On error, the in-memory state is left unchanged.
func (db *DB) Reload() error {
	// Stop background workers before taking the DB lock — workers may need DB
	// read locks while they shut down (same pattern as Close).
	db.stopMu.Lock()
	stopFuncs := append([]func(){}, db.stopFuncs...)
	db.stopFuncs = nil
	db.stopMu.Unlock()

	for _, stop := range stopFuncs {
		stop()
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	// In-memory storage returns the same pointer it was given, so reloading
	// would just re-import the in-memory state — a no-op at best, confusing at
	// worst. Only file-backed storage has an external source of truth.
	if db.path == ":memory:" {
		return nil
	}

	snapshot, err := db.storage.Load()
	if err != nil {
		return err
	}

	// If nothing was ever persisted, leave the in-memory state as-is.
	if snapshot == nil {
		return nil
	}

	// Rebuild in-memory state from the fresh snapshot. We build into new maps
	// so that if anything fails mid-way, the old state is still intact.
	newCollections := make(map[string]*Collection, len(snapshot.Collections))
	newGraphs := make(map[string]*KnowledgeGraph, len(snapshot.KnowledgeGraphs))
	newEpisodes := make(map[string]*EpisodeStore, len(snapshot.EpisodeStores))

	snapshot = storage.Migrate(snapshot)

	newMetadata := deepCopyMap(snapshot.Metadata)
	if newMetadata == nil {
		newMetadata = make(map[string]any)
	}

	for name, collSnapshot := range snapshot.Collections {
		coll := newCollection(name, defaultCollectionConfig(), db)
		coll.loadFromSnapshot(collSnapshot)
		newCollections[name] = coll
	}

	for name, graphSnap := range snapshot.KnowledgeGraphs {
		collName := "_kg_" + name
		coll, ok := newCollections[collName]
		if !ok {
			coll = newCollection(collName, defaultCollectionConfig(), db)
			newCollections[collName] = coll
		}
		kg := &KnowledgeGraph{
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
		kg.loadFromSnapshot(graphSnap)
		newGraphs[name] = kg
	}

	for name, epSnap := range snapshot.EpisodeStores {
		coll, ok := newCollections[epSnap.CollectionName]
		if !ok {
			coll = newCollection(epSnap.CollectionName, defaultCollectionConfig(), db)
			newCollections[epSnap.CollectionName] = coll
		}
		es := &EpisodeStore{
			collection:   coll,
			episodes:     make(map[string]*Episode),
			distanceFunc: floats.GetDistanceFunc(coll.distanceType),
			higherBetter: floats.IsHigherBetter(coll.distanceType),
		}
		es.loadFromSnapshot(epSnap)
		newEpisodes[name] = es
	}

	// Atomic swap: replace all in-memory state at once.
	db.metadata = newMetadata
	db.collections = newCollections
	db.knowledgeGraphs = newGraphs
	db.episodeStores = newEpisodes
	db.createdAt = snapshot.CreatedAt
	db.updatedAt = snapshot.UpdatedAt

	// Clear subscription managers — they reference old collections. New ones
	// will be created lazily on the next subscribe call.
	db.subMu.Lock()
	db.subscriptions = make(map[string]*subscriptionManager)
	db.subMu.Unlock()

	// Re-apply the write-ahead log on top of the reloaded snapshot: for a
	// shared reader this picks up a writer's not-yet-snapshotted mutations
	// (a torn in-flight append is skipped); for a WAL-enabled writer it
	// restores its own since-last-save mutations that Load cannot see.
	if entries, err := storage.ReadWALEntries(storage.WALPath(db.path)); err == nil {
		db.applyWALEntries(entries)
	} else {
		db.logger.Error("veclite: reload skipped unreadable WAL", "error", err)
	}

	return nil
}

// Stats returns statistics about the database.
func (db *DB) Stats() DatabaseStats {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := DatabaseStats{
		Path:            db.path,
		Collections:     len(db.collections),
		CollectionStats: make([]CollectionStats, 0, len(db.collections)),
	}

	for _, coll := range db.collections {
		collStats := coll.Stats()
		stats.TotalRecords += collStats.Count
		stats.CollectionStats = append(stats.CollectionStats, collStats)
	}

	return stats
}

// IsClosed returns true if the database is closed.
func (db *DB) IsClosed() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.closed
}

// Metrics returns the current metrics snapshot.
func (db *DB) Metrics() MetricsSnapshot {
	return db.metrics.Snapshot()
}

// getSubscriptionManager returns or creates a subscription manager for the given collection.
func (db *DB) getSubscriptionManager(c *Collection) *subscriptionManager {
	db.subMu.Lock()
	defer db.subMu.Unlock()

	key := c.name
	if sm, ok := db.subscriptions[key]; ok {
		return sm
	}

	sm := newSubscriptionManager(c.distanceType)
	db.subscriptions[key] = sm
	return sm
}
