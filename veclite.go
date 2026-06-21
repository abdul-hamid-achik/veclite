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
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// Version is the library version.
const Version = "0.17.0"

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
		// Acquire file lock to prevent concurrent access
		if err := fs.Lock(); err != nil {
			return nil, err
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

	return errors.Join(syncErr, closeErr)
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
	snapshot := db.snapshotLocked()
	return db.storage.Save(snapshot)
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
