// Package veclite provides an embeddable vector database with zero external dependencies.
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

	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// Version is the library version.
const Version = "0.2.0"

// DB represents a VecLite database.
type DB struct {
	path        string
	storage     storage.Backend
	config      *dbConfig
	collections map[string]*Collection
	mu          sync.RWMutex
	closed      bool
	createdAt   time.Time
	updatedAt   time.Time
	metrics     *Metrics
	logger      Logger

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
		path:          path,
		storage:       store,
		config:        config,
		collections:   make(map[string]*Collection),
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
		metrics:       newMetrics(),
		logger:        logger,
		subscriptions: make(map[string]*subscriptionManager),
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
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	db.closed = true

	// Stop all background workers first
	db.stopMu.Lock()
	for _, stop := range db.stopFuncs {
		stop()
	}
	db.stopFuncs = nil
	db.stopMu.Unlock()

	// Sync before closing — capture error but don't abort
	syncErr := db.syncLocked()

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

// Collection returns a collection by name, creating it if it doesn't exist.
// This is the preferred way to get collections for most use cases.
// In read-only mode, returns nil if the collection doesn't exist (cannot create).
func (db *DB) Collection(name string) *Collection {
	db.mu.Lock()
	defer db.mu.Unlock()

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
		Version:     1,
		Collections: make(map[string]*storage.CollectionSnapshot, len(db.collections)),
		CreatedAt:   db.createdAt,
		UpdatedAt:   time.Now(),
	}

	for name, coll := range db.collections {
		snapshot.Collections[name] = coll.snapshot()
	}

	return snapshot
}

// loadFromSnapshot restores the database from a snapshot.
func (db *DB) loadFromSnapshot(snapshot *storage.DatabaseSnapshot) {
	db.createdAt = snapshot.CreatedAt
	db.updatedAt = snapshot.UpdatedAt

	for name, collSnapshot := range snapshot.Collections {
		coll := newCollection(name, defaultCollectionConfig(), db)
		coll.loadFromSnapshot(collSnapshot)
		db.collections[name] = coll
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
