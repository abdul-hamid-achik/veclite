package veclite

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/hnsw"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// Collection represents a collection of vectors with the same dimension.
type Collection struct {
	name         string
	metadata     map[string]any
	dimension    int
	distanceType floats.DistanceType
	distanceFunc floats.DistanceFunc
	higherBetter bool
	indexType    IndexType
	hnswConfig   *HNSWConfig
	index        Index

	// Text search
	textIndex *invertedIndex

	// Named vector spaces beyond the implicit "default" space. The default space
	// is represented by the dimension/distanceType/index fields above and by
	// Record.Vector; entries here describe extra spaces whose vectors live in
	// Record.Vectors. Guarded by mu like the rest of the collection state.
	spaces map[string]*vectorSpace

	// profile is the collection's first-class default embedding profile (optional).
	profile *EmbeddingProfile

	// Auto-embedding
	embedder Embedder

	// Memory pressure handling
	memoryConfig *MemoryConfig

	mu      sync.RWMutex
	records map[uint64]*Record
	nextID  uint64

	db *DB
}

// newCollection creates a new collection.
func newCollection(name string, config *collectionConfig, db *DB) *Collection {
	c := &Collection{
		name:         name,
		metadata:     make(map[string]any),
		dimension:    config.dimension,
		distanceType: config.distanceType,
		distanceFunc: floats.GetDistanceFunc(config.distanceType),
		higherBetter: floats.IsHigherBetter(config.distanceType),
		indexType:    config.indexType,
		hnswConfig:   config.hnswConfig,
		records:      make(map[uint64]*Record),
		nextID:       1,
		db:           db,
		embedder:     config.embedder,
		memoryConfig: cloneMemoryConfig(config.memoryConfig),
		spaces:       make(map[string]*vectorSpace),
		profile:      cloneProfile(config.profile),
	}

	// Declare any vector spaces requested at creation time.
	for _, vsc := range config.vectorSpaces {
		// newCollection runs before the collection is published, so this cannot
		// race; addVectorSpaceLocked validates names and dimensions.
		_ = c.addVectorSpaceLocked(vsc)
	}

	// Initialize text index if configured
	if len(config.textIndexFields) > 0 {
		c.textIndex = newInvertedIndex(config.textIndexFields)
	}

	// Initialize index if HNSW is configured
	if config.indexType == IndexTypeHNSW && config.hnswConfig != nil && config.dimension > 0 {
		c.index = newHNSWIndex(
			config.dimension,
			config.distanceType,
			config.hnswConfig,
		)
		c.setupVectorProvider()
	}

	if c.memoryConfig != nil && c.memoryConfig.CleanupInterval > 0 && db != nil {
		c.StartMemoryLimiter(*c.memoryConfig)
	}

	return c
}

func cloneMemoryConfig(config *MemoryConfig) *MemoryConfig {
	if config == nil {
		return nil
	}
	cp := *config
	return &cp
}

// initHNSWIfNeeded initializes the HNSW index when the dimension becomes known.
// Must be called with the collection lock held.
func (c *Collection) initHNSWIfNeeded() {
	if c.indexType == IndexTypeHNSW && c.hnswConfig != nil && c.index == nil && c.dimension > 0 {
		c.index = newHNSWIndex(
			c.dimension,
			c.distanceType,
			c.hnswConfig,
		)
		c.setupVectorProvider()
	}
}

// setupVectorProvider configures the HNSW index to use the collection's records
// as the vector source, avoiding duplicate vector storage.
func (c *Collection) setupVectorProvider() {
	if c.index == nil {
		return
	}
	hnswIdx, ok := c.index.(*hnswIndex)
	if !ok {
		return
	}
	hnswIdx.internal().SetVectorProvider(func(id uint64) ([]float32, bool) {
		rec, ok := c.records[id]
		if !ok || len(rec.Vector) == 0 {
			return nil, false
		}
		return rec.Vector, true
	})
}

// checkReadOnly returns ErrReadOnly if the database is in read-only mode.
func (c *Collection) checkReadOnly() error {
	if c.db != nil {
		c.db.mu.RLock()
		closed := c.db.closed
		readOnly := c.db.config != nil && c.db.config.readOnly
		c.db.mu.RUnlock()
		if closed {
			return ErrDatabaseClosed
		}
		if readOnly {
			return ErrReadOnly
		}
	}
	return nil
}

// syncIfNeeded performs a sync if syncOnWrite is enabled.
func (c *Collection) syncIfNeeded() {
	if c.db != nil && c.db.config != nil && c.db.config.syncOnWrite {
		// Sync requires the DB lock; we must not hold the collection lock.
		_ = c.db.Sync()
	}
}

func (c *Collection) reindexRecordLocked(record *Record) {
	if c.textIndex != nil && record != nil {
		c.textIndex.indexRecord(record.ID, record.Payload, record.Content)
	}
}

func (c *Collection) enforceMemoryLimitIfConfigured() {
	if c.memoryConfig == nil || c.memoryConfig.MaxRecords <= 0 {
		return
	}
	c.EnforceMemoryLimit(*c.memoryConfig)
}

// Name returns the collection name.
func (c *Collection) Name() string {
	return c.name
}

// Metadata returns a deep copy of the collection metadata.
func (c *Collection) Metadata() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return deepCopyMap(c.metadata)
}

// SetMetadata replaces the collection metadata.
func (c *Collection) SetMetadata(metadata map[string]any) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()
	c.metadata = deepCopyMap(metadata)
	if c.metadata == nil {
		c.metadata = make(map[string]any)
	}
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// SetMetadataValue sets one collection metadata value.
func (c *Collection) SetMetadataValue(key string, value any) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()
	if c.metadata == nil {
		c.metadata = make(map[string]any)
	}
	c.metadata[key] = deepCopyValue(value)
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// DeleteMetadataValue removes one collection metadata value.
func (c *Collection) DeleteMetadataValue(key string) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()
	delete(c.metadata, key)
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// Dimension returns the vector dimension.
// Returns 0 if no vectors have been inserted yet.
func (c *Collection) Dimension() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dimension
}

// Count returns the number of records in the collection.
func (c *Collection) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.records)
}

// DistanceType returns the distance metric type.
func (c *Collection) DistanceType() floats.DistanceType {
	return c.distanceType
}

// Insert adds a vector with optional payload to the collection.
// Returns the assigned record ID.
func (c *Collection) Insert(vector []float32, payload map[string]any) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}
	if len(vector) == 0 {
		return 0, ErrEmptyVector
	}

	id, err := c.insertLocked(vector, payload)
	if err != nil {
		return 0, err
	}

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordInsert()
	}
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()

	// Notify subscribers about the new record
	if record, err := c.Get(id); err == nil {
		c.notifySubscribers(record)
	}

	return id, nil
}

// InsertWithOptions adds a vector with optional payload and insert options.
// Use this method to set TTL, importance, and other options.
func (c *Collection) InsertWithOptions(vector []float32, payload map[string]any, opts ...InsertOption) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}
	if len(vector) == 0 {
		return 0, ErrEmptyVector
	}

	config := defaultInsertConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	id, err := c.insertWithOptionsLocked(vector, payload, config)
	if err != nil {
		return 0, err
	}

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordInsert()
	}
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()

	// Notify subscribers about the new record
	if record, err := c.Get(id); err == nil {
		c.notifySubscribers(record)
	}

	return id, nil
}

// insertWithOptionsLocked performs the insert with options while holding the collection lock.
func (c *Collection) insertWithOptionsLocked(vector []float32, payload map[string]any, config *insertConfig) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vector)
		c.initHNSWIfNeeded()
	} else if len(vector) != c.dimension {
		return 0, &DimensionError{Expected: c.dimension, Got: len(vector)}
	}

	// Create record
	now := time.Now()
	id := c.nextID
	c.nextID++

	record := &Record{
		ID:         id,
		Vector:     make([]float32, len(vector)),
		Payload:    payload,
		Content:    config.content,
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  config.computeExpiresAt(),
		Importance: config.importance,
	}
	copy(record.Vector, vector)

	c.records[id] = record

	// Insert into index if enabled
	if c.index != nil {
		if err := c.index.Insert(id, vector); err != nil {
			// Rollback record insertion on index failure
			delete(c.records, id)
			c.nextID--
			return 0, err
		}
	}

	// Index for text search
	if c.textIndex != nil {
		c.textIndex.indexRecord(id, payload, record.Content)
	}

	return id, nil
}

// CleanupExpired removes all expired records from the collection.
// Returns the number of records removed.
func (c *Collection) CleanupExpired() (int, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}

	c.mu.Lock()

	deleted := 0
	now := time.Now()
	for id, record := range c.records {
		if !record.ExpiresAt.IsZero() && now.After(record.ExpiresAt) {
			// Delete from index first
			if c.index != nil {
				_ = c.index.Delete(id)
			}
			if c.textIndex != nil {
				c.textIndex.removeRecord(id)
			}
			delete(c.records, id)
			deleted++
		}
	}

	c.mu.Unlock()

	if deleted > 0 {
		c.syncIfNeeded()
	}
	return deleted, nil
}

// CountExpired returns the number of expired records in the collection.
func (c *Collection) CountExpired() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, record := range c.records {
		if !record.ExpiresAt.IsZero() && now.After(record.ExpiresAt) {
			count++
		}
	}
	return count
}

// insertLocked performs the insert while holding the collection lock.
func (c *Collection) insertLocked(vector []float32, payload map[string]any) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vector)
		c.initHNSWIfNeeded()
	} else if len(vector) != c.dimension {
		return 0, &DimensionError{Expected: c.dimension, Got: len(vector)}
	}

	// Create record
	now := time.Now()
	id := c.nextID
	c.nextID++

	record := &Record{
		ID:        id,
		Vector:    make([]float32, len(vector)),
		Payload:   payload,
		CreatedAt: now,
		UpdatedAt: now,
	}
	copy(record.Vector, vector)

	c.records[id] = record

	// Insert into index if enabled
	if c.index != nil {
		if err := c.index.Insert(id, vector); err != nil {
			// Rollback record insertion on index failure
			delete(c.records, id)
			c.nextID--
			return 0, err
		}
	}

	// Index for text search
	if c.textIndex != nil {
		c.textIndex.indexRecord(id, payload, record.Content)
	}

	return id, nil
}

// InsertBatch adds multiple vectors with payloads to the collection.
// Returns the assigned record IDs.
// If payloads is nil or shorter than vectors, missing payloads are treated as nil.
func (c *Collection) InsertBatch(vectors [][]float32, payloads []map[string]any) ([]uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return []uint64{}, nil
	}

	// Validate all vectors first
	for i, v := range vectors {
		if len(v) == 0 {
			return nil, ErrEmptyVector
		}
		if i > 0 && len(v) != len(vectors[0]) {
			return nil, ErrBatchSizeMismatch
		}
	}

	ids, err := c.insertBatchLocked(vectors, payloads)
	if err != nil {
		return nil, err
	}

	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return ids, nil
}

// insertBatchLocked performs the batch insert while holding the collection lock.
func (c *Collection) insertBatchLocked(vectors [][]float32, payloads []map[string]any) ([]uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vectors[0])
		c.initHNSWIfNeeded()
	} else if len(vectors[0]) != c.dimension {
		return nil, &DimensionError{Expected: c.dimension, Got: len(vectors[0])}
	}

	// Insert all records
	ids := make([]uint64, len(vectors))
	now := time.Now()

	for i, vector := range vectors {
		id := c.nextID
		c.nextID++

		var payload map[string]any
		if payloads != nil && i < len(payloads) {
			payload = payloads[i]
		}

		record := &Record{
			ID:        id,
			Vector:    make([]float32, len(vector)),
			Payload:   payload,
			CreatedAt: now,
			UpdatedAt: now,
		}
		copy(record.Vector, vector)

		c.records[id] = record
		ids[i] = id

		// Insert into index if enabled
		if c.index != nil {
			if err := c.index.Insert(id, vector); err != nil {
				// On failure, rollback this and all previous insertions
				for j := 0; j <= i; j++ {
					delete(c.records, ids[j])
					if c.index != nil {
						_ = c.index.Delete(ids[j])
					}
					if c.textIndex != nil {
						c.textIndex.removeRecord(ids[j])
					}
				}
				c.nextID -= uint64(i + 1)
				return nil, err
			}
		}

		// Index for text search
		if c.textIndex != nil {
			c.textIndex.indexRecord(id, payload, record.Content)
		}
	}

	return ids, nil
}

// Get retrieves a record by ID.
func (c *Collection) Get(id uint64) (*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, ok := c.records[id]
	if !ok {
		return nil, &NotFoundError{Type: "record", ID: strconv.FormatUint(id, 10)}
	}

	return record.Clone(), nil
}

// GetVector retrieves just the vector for a record.
func (c *Collection) GetVector(id uint64) ([]float32, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, ok := c.records[id]
	if !ok {
		return nil, &NotFoundError{Type: "record", ID: strconv.FormatUint(id, 10)}
	}

	result := make([]float32, len(record.Vector))
	copy(result, record.Vector)
	return result, nil
}

// Delete removes a record by ID.
func (c *Collection) Delete(id uint64) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()

	if _, ok := c.records[id]; !ok {
		c.mu.Unlock()
		return &NotFoundError{Type: "record", ID: strconv.FormatUint(id, 10)}
	}

	// Delete from index first (soft delete)
	if c.index != nil {
		_ = c.index.Delete(id)
	}
	if c.textIndex != nil {
		c.textIndex.removeRecord(id)
	}

	delete(c.records, id)
	c.mu.Unlock()

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordDelete()
	}
	c.syncIfNeeded()
	return nil
}

// DeleteWhere removes all records matching the filters.
// Returns the number of deleted records.
func (c *Collection) DeleteWhere(filters ...Filter) (int, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}
	if len(filters) == 0 {
		return 0, nil
	}

	c.mu.Lock()

	deleted := 0
	for id, record := range c.records {
		match := true
		for _, f := range filters {
			if !f.Match(record) {
				match = false
				break
			}
		}
		if match {
			// Delete from index first
			if c.index != nil {
				_ = c.index.Delete(id)
			}
			if c.textIndex != nil {
				c.textIndex.removeRecord(id)
			}
			delete(c.records, id)
			deleted++
		}
	}

	c.mu.Unlock()

	if deleted > 0 {
		c.syncIfNeeded()
	}
	return deleted, nil
}

// Update updates the payload for a record.
func (c *Collection) Update(id uint64, payload map[string]any) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()

	record, ok := c.records[id]
	if !ok {
		c.mu.Unlock()
		return &NotFoundError{Type: "record", ID: strconv.FormatUint(id, 10)}
	}

	record.Payload = payload
	record.UpdatedAt = time.Now()
	c.reindexRecordLocked(record)
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// UpdateDocument updates the content and payload for a record without changing its vector.
func (c *Collection) UpdateDocument(id uint64, content string, payload map[string]any) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()

	record, ok := c.records[id]
	if !ok {
		c.mu.Unlock()
		return &NotFoundError{Type: "record", ID: strconv.FormatUint(id, 10)}
	}

	record.Content = content
	record.Payload = payload
	record.UpdatedAt = time.Now()
	c.reindexRecordLocked(record)
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// UpdateVector updates the vector for a record.
func (c *Collection) UpdateVector(id uint64, vector []float32) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}
	if len(vector) == 0 {
		return ErrEmptyVector
	}

	c.mu.Lock()

	record, ok := c.records[id]
	if !ok {
		c.mu.Unlock()
		return &NotFoundError{Type: "record", ID: strconv.FormatUint(id, 10)}
	}

	if c.dimension == 0 {
		c.dimension = len(vector)
		c.initHNSWIfNeeded()
	} else if len(vector) != c.dimension {
		c.mu.Unlock()
		return &DimensionError{Expected: c.dimension, Got: len(vector)}
	}

	// Update index if present
	if c.index != nil {
		if len(record.Vector) > 0 {
			// Hard delete old vector from index (needed for re-insertion with same ID)
			c.hardDeleteFromIndex(id)
		}
		// Insert new vector
		if err := c.index.Insert(id, vector); err != nil {
			// Re-insert old vector on failure
			if len(record.Vector) > 0 {
				_ = c.index.Insert(id, record.Vector)
			}
			c.mu.Unlock()
			return err
		}
	}

	record.Vector = make([]float32, len(vector))
	copy(record.Vector, vector)
	record.UpdatedAt = time.Now()
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// hardDeleteFromIndex removes a vector from the index completely.
// This is needed for update operations where we re-insert with the same ID.
func (c *Collection) hardDeleteFromIndex(id uint64) {
	if c.index == nil {
		return
	}
	// Try hard delete for HNSW index
	if hnswIdx, ok := c.index.(*hnswIndex); ok {
		_ = hnswIdx.hardDelete(id)
		return
	}
	// Fall back to regular delete for other index types
	_ = c.index.Delete(id)
}

// Upsert inserts a new record or updates an existing one by ID.
// If the ID is 0, a new record is created with an auto-generated ID.
// If the ID exists, the vector and payload are updated.
// If the ID doesn't exist, a new record is created with that ID.
// Returns the record ID (either the provided one or newly generated).
func (c *Collection) Upsert(id uint64, vector []float32, payload map[string]any) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}
	if len(vector) == 0 {
		return 0, ErrEmptyVector
	}

	id, err := c.upsertLocked(id, vector, payload)
	if err != nil {
		return 0, err
	}

	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return id, nil
}

// upsertLocked performs the upsert while holding the collection lock.
func (c *Collection) upsertLocked(id uint64, vector []float32, payload map[string]any) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If ID is 0, behave like Insert
	if id == 0 {
		return c.insertUnlocked(vector, payload)
	}

	// Check if record exists
	record, exists := c.records[id]
	if exists {
		// Update existing record
		if c.dimension == 0 {
			c.dimension = len(vector)
			c.initHNSWIfNeeded()
		} else if len(vector) != c.dimension {
			return 0, &DimensionError{Expected: c.dimension, Got: len(vector)}
		}

		// Update index if present
		if c.index != nil {
			if len(record.Vector) > 0 {
				c.hardDeleteFromIndex(id)
			}
			if err := c.index.Insert(id, vector); err != nil {
				if len(record.Vector) > 0 {
					_ = c.index.Insert(id, record.Vector)
				}
				return 0, err
			}
		}

		record.Vector = make([]float32, len(vector))
		copy(record.Vector, vector)
		record.Payload = payload
		record.UpdatedAt = time.Now()
		c.reindexRecordLocked(record)
		return id, nil
	}

	// Insert with specified ID
	return c.insertWithIDUnlocked(id, vector, payload)
}

// UpsertByKey inserts a new record or updates an existing one based on a key field.
// If a record with payload[keyField] == keyValue exists, it is updated.
// Otherwise, a new record is inserted.
// Returns the record ID and whether it was an insert (true) or update (false).
func (c *Collection) UpsertByKey(keyField string, keyValue any, vector []float32, payload map[string]any) (uint64, bool, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, false, err
	}
	if len(vector) == 0 {
		return 0, false, ErrEmptyVector
	}

	id, inserted, err := c.upsertByKeyLocked(keyField, keyValue, vector, payload)
	if err != nil {
		return 0, false, err
	}

	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return id, inserted, nil
}

// upsertByKeyLocked performs the upsert-by-key while holding the collection lock.
func (c *Collection) upsertByKeyLocked(keyField string, keyValue any, vector []float32, payload map[string]any) (uint64, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find existing record with matching key
	var existingID uint64
	var existingRecord *Record
	for id, record := range c.records {
		if record.Payload != nil {
			if v, ok := record.Payload[keyField]; ok && compareValues(v, keyValue) {
				existingID = id
				existingRecord = record
				break
			}
		}
	}

	if existingRecord != nil {
		// Update existing record
		if c.dimension == 0 {
			c.dimension = len(vector)
			c.initHNSWIfNeeded()
		} else if len(vector) != c.dimension {
			return 0, false, &DimensionError{Expected: c.dimension, Got: len(vector)}
		}

		// Update index if present
		if c.index != nil {
			if len(existingRecord.Vector) > 0 {
				c.hardDeleteFromIndex(existingID)
			}
			if err := c.index.Insert(existingID, vector); err != nil {
				if len(existingRecord.Vector) > 0 {
					_ = c.index.Insert(existingID, existingRecord.Vector)
				}
				return 0, false, err
			}
		}

		existingRecord.Vector = make([]float32, len(vector))
		copy(existingRecord.Vector, vector)
		existingRecord.Payload = payload
		existingRecord.UpdatedAt = time.Now()
		c.reindexRecordLocked(existingRecord)
		return existingID, false, nil
	}

	// Insert new record
	id, err := c.insertUnlocked(vector, payload)
	return id, true, err
}

// insertUnlocked inserts a record without acquiring the lock.
// Caller must hold c.mu.Lock().
func (c *Collection) insertUnlocked(vector []float32, payload map[string]any) (uint64, error) {
	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vector)
		c.initHNSWIfNeeded()
	} else if len(vector) != c.dimension {
		return 0, &DimensionError{Expected: c.dimension, Got: len(vector)}
	}

	now := time.Now()
	id := c.nextID
	c.nextID++

	record := &Record{
		ID:        id,
		Vector:    make([]float32, len(vector)),
		Payload:   payload,
		CreatedAt: now,
		UpdatedAt: now,
	}
	copy(record.Vector, vector)

	c.records[id] = record

	if c.index != nil {
		if err := c.index.Insert(id, vector); err != nil {
			delete(c.records, id)
			c.nextID--
			return 0, err
		}
	}

	c.reindexRecordLocked(record)

	return id, nil
}

// insertWithIDUnlocked inserts a record with a specific ID without acquiring the lock.
// Caller must hold c.mu.Lock().
func (c *Collection) insertWithIDUnlocked(id uint64, vector []float32, payload map[string]any) (uint64, error) {
	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vector)
		c.initHNSWIfNeeded()
	} else if len(vector) != c.dimension {
		return 0, &DimensionError{Expected: c.dimension, Got: len(vector)}
	}

	now := time.Now()
	record := &Record{
		ID:        id,
		Vector:    make([]float32, len(vector)),
		Payload:   payload,
		CreatedAt: now,
		UpdatedAt: now,
	}
	copy(record.Vector, vector)

	c.records[id] = record

	// Update nextID if necessary
	if id >= c.nextID {
		c.nextID = id + 1
	}

	if c.index != nil {
		if err := c.index.Insert(id, vector); err != nil {
			delete(c.records, id)
			return 0, err
		}
	}

	c.reindexRecordLocked(record)

	return id, nil
}

// Search finds the most similar vectors to the query vector.
func (c *Collection) Search(query []float32, opts ...SearchOption) ([]Result, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}

	searchStart := time.Now()
	defer func() {
		if c.db != nil && c.db.metrics != nil {
			c.db.metrics.recordSearch(time.Since(searchStart))
		}
	}()

	c.mu.RLock()

	if c.dimension > 0 && len(query) != c.dimension {
		c.mu.RUnlock()
		return nil, &DimensionError{Expected: c.dimension, Got: len(query)}
	}

	// Apply options
	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	var results []Result
	var err error

	// Use HNSW index if available
	if c.index != nil {
		results, err = c.searchWithIndex(query, config)
		if err != nil {
			c.mu.RUnlock()
			return nil, err
		}
		// If we got enough results (or no filters were applied), continue.
		// Otherwise, fall back to brute force for completeness.
		if len(results) < config.effectiveTopK() && len(config.filters) > 0 {
			results, err = c.searchBruteForce(query, config)
		}
	} else {
		// Brute force search
		results, err = c.searchBruteForce(query, config)
	}

	if err != nil {
		c.mu.RUnlock()
		return nil, err
	}

	// Apply score modifiers (decay and importance boost)
	if config.decay != nil || config.importanceBoost > 0 {
		results = c.applyScoreModifiersToResults(results, config)
	}

	c.mu.RUnlock()

	// Track access for returned results if enabled
	if config.accessTracking && len(results) > 0 {
		c.trackAccess(results)
	}

	return config.applyPagination(results), nil
}

// applyScoreModifiersToResults applies decay and importance boost to search results.
func (c *Collection) applyScoreModifiersToResults(results []Result, config *searchConfig) []Result {
	for i := range results {
		results[i].Score = applyScoreModifiers(results[i].Score, results[i].Record, config)
	}

	// Re-sort after applying modifiers
	if c.higherBetter {
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			return results[i].Record.ID < results[j].Record.ID
		})
	} else {
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score < results[j].Score
			}
			return results[i].Record.ID < results[j].Record.ID
		})
	}

	return results
}

// trackAccess updates access tracking for the given results.
func (c *Collection) trackAccess(results []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, r := range results {
		if record, ok := c.records[r.Record.ID]; ok {
			record.AccessCount++
			record.LastAccessedAt = now
		}
	}
}

// searchWithIndex performs search using the HNSW index.
// When filters are present, uses adaptive over-fetching: starts with 4x,
// and if too few results pass the filter, retries with larger multipliers.
func (c *Collection) searchWithIndex(query []float32, config *searchConfig) ([]Result, error) {
	if c.index.Count() == 0 {
		return nil, nil
	}

	ef := config.efSearch
	if ef == 0 && c.hnswConfig != nil {
		ef = c.hnswConfig.EfSearch
	}

	effectiveK := config.effectiveTopK()
	hasFilters := len(config.filters) > 0

	if !hasFilters && config.threshold == nil {
		// No filters — straightforward search
		var indexResults []IndexResult
		var err error
		if ef > 0 {
			indexResults, err = c.index.SearchWithEf(query, effectiveK, ef)
		} else {
			indexResults, err = c.index.Search(query, effectiveK)
		}
		if err != nil {
			return nil, err
		}
		return c.indexResultsToResults(indexResults, config, effectiveK), nil
	}

	// Adaptive over-fetching for filtered searches.
	// Start with 4x, then try 8x, 16x, up to a max multiplier.
	multipliers := []int{4, 8, 16}
	maxResults := max(effectiveK*32, 500)

	var results []Result
	for _, mult := range multipliers {
		requestK := max(effectiveK*mult, 100)
		if requestK > maxResults {
			requestK = maxResults
		}

		var indexResults []IndexResult
		var err error
		if ef > 0 {
			indexResults, err = c.index.SearchWithEf(query, requestK, ef)
		} else {
			indexResults, err = c.index.Search(query, requestK)
		}
		if err != nil {
			return nil, err
		}

		results = c.filterIndexResults(indexResults, config, effectiveK)
		if len(results) >= effectiveK || requestK >= maxResults {
			break
		}
	}

	return results, nil
}

// filterIndexResults converts index results to Results, applying filters and thresholds.
func (c *Collection) filterIndexResults(indexResults []IndexResult, config *searchConfig, effectiveK int) []Result {
	results := make([]Result, 0, len(indexResults))
	for _, ir := range indexResults {
		record, ok := c.records[ir.ID]
		if !ok {
			continue
		}

		if !config.matchesFilters(record) {
			continue
		}

		if config.threshold != nil {
			if c.higherBetter && ir.Distance < *config.threshold {
				continue
			}
			if !c.higherBetter && ir.Distance > *config.threshold {
				continue
			}
		}

		results = append(results, Result{
			Record: config.cloneRecordForResult(record),
			Score:  ir.Distance,
		})

		if len(results) >= effectiveK {
			break
		}
	}
	return results
}

// indexResultsToResults converts index results to Results (no filtering).
func (c *Collection) indexResultsToResults(indexResults []IndexResult, config *searchConfig, effectiveK int) []Result {
	results := make([]Result, 0, len(indexResults))
	for _, ir := range indexResults {
		record, ok := c.records[ir.ID]
		if !ok {
			continue
		}

		if config.threshold != nil {
			if c.higherBetter && ir.Distance < *config.threshold {
				continue
			}
			if !c.higherBetter && ir.Distance > *config.threshold {
				continue
			}
		}

		results = append(results, Result{
			Record: config.cloneRecordForResult(record),
			Score:  ir.Distance,
		})

		if len(results) >= effectiveK {
			break
		}
	}
	return results
}

// searchBruteForce performs brute-force search.
func (c *Collection) searchBruteForce(query []float32, config *searchConfig) ([]Result, error) {
	// Collect matching results
	results := make([]Result, 0)
	for _, record := range c.records {
		if len(record.Vector) == 0 {
			continue
		}

		// Apply filters
		if !config.matchesFilters(record) {
			continue
		}

		score := c.distanceFunc(query, record.Vector)

		// Apply threshold
		if config.threshold != nil {
			if c.higherBetter && score < *config.threshold {
				continue
			}
			if !c.higherBetter && score > *config.threshold {
				continue
			}
		}

		results = append(results, Result{
			Record: config.cloneRecordForResult(record),
			Score:  score,
		})
	}

	// Sort results (stable sort with ID tiebreaker for deterministic pagination)
	if c.higherBetter {
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			return results[i].Record.ID < results[j].Record.ID
		})
	} else {
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score < results[j].Score
			}
			return results[i].Record.ID < results[j].Record.ID
		})
	}

	// Apply topK (including offset for later pagination)
	effectiveK := config.effectiveTopK()
	if len(results) > effectiveK {
		results = results[:effectiveK]
	}

	return results, nil
}

// Find retrieves all records matching the filters.
func (c *Collection) Find(filters ...Filter) ([]*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make([]*Record, 0)
	for _, record := range c.records {
		match := true
		for _, f := range filters {
			if !f.Match(record) {
				match = false
				break
			}
		}
		if match {
			results = append(results, record.Clone())
		}
	}

	return results, nil
}

// FindOne retrieves the first record matching the filters.
func (c *Collection) FindOne(filters ...Filter) (*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, record := range c.records {
		match := true
		for _, f := range filters {
			if !f.Match(record) {
				match = false
				break
			}
		}
		if match {
			return record.Clone(), nil
		}
	}

	return nil, ErrNotFound
}

// Stats returns statistics about the collection.
func (c *Collection) Stats() CollectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	vectorCount := 0
	for _, record := range c.records {
		if len(record.Vector) > 0 {
			vectorCount++
		}
	}

	return CollectionStats{
		Name:          c.name,
		Count:         len(c.records),
		VectorCount:   vectorCount,
		TextOnlyCount: len(c.records) - vectorCount,
		Dimension:     c.dimension,
		DistanceType:  string(c.distanceType),
		IndexType:     string(c.indexType),
	}
}

// IndexType returns the index type for this collection.
func (c *Collection) IndexType() IndexType {
	return c.indexType
}

// HasIndex returns true if this collection has an index.
func (c *Collection) HasIndex() bool {
	return c.index != nil
}

// snapshot creates a serializable snapshot of the collection.
func (c *Collection) snapshot() *storage.CollectionSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := &storage.CollectionSnapshot{
		Name:             c.name,
		Metadata:         deepCopyMap(c.metadata),
		Dimension:        c.dimension,
		DistanceType:     c.distanceType,
		NextID:           c.nextID,
		Records:          make([]*storage.RecordSnapshot, 0, len(c.records)),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		IndexType:        string(c.indexType),
		HNSWConfig:       c.hnswConfig,
		EmbeddingProfile: c.profile.toSnapshot(),
	}

	for _, record := range c.records {
		var vectors map[string][]float32
		if len(record.Vectors) > 0 {
			vectors = make(map[string][]float32, len(record.Vectors))
			for name, vec := range record.Vectors {
				cp := make([]float32, len(vec))
				copy(cp, vec)
				vectors[name] = cp
			}
		}
		snapshot.Records = append(snapshot.Records, &storage.RecordSnapshot{
			ID:             record.ID,
			Vector:         record.Vector,
			Vectors:        vectors,
			Payload:        record.Payload,
			Content:        record.Content,
			CreatedAt:      record.CreatedAt,
			UpdatedAt:      record.UpdatedAt,
			ExpiresAt:      record.ExpiresAt,
			Importance:     record.Importance,
			AccessCount:    record.AccessCount,
			LastAccessedAt: record.LastAccessedAt,
		})
	}

	// Snapshot additional named vector spaces (the default space is captured by
	// the collection-level Dimension/DistanceType/IndexType/HNSW fields above).
	if len(c.spaces) > 0 {
		snapshot.VectorSpaces = make([]*storage.VectorSpaceSnapshot, 0, len(c.spaces))
		for _, sp := range c.spaces {
			vss := &storage.VectorSpaceSnapshot{
				Name:         sp.name,
				Dimension:    sp.dimension,
				DistanceType: sp.distanceType,
				Modality:     sp.modality,
				Provider:     sp.provider,
				Model:        sp.model,
				IndexType:    string(sp.indexType),
				HNSWConfig:   sp.hnswConfig,
				Profile:      sp.profile.toSnapshot(),
			}
			if sp.index != nil && sp.indexType == IndexTypeHNSW {
				if hnswIdx, ok := sp.index.(*hnswIndex); ok {
					vss.HNSWSnapshot = hnswIdx.internal().Snapshot()
				}
			}
			snapshot.VectorSpaces = append(snapshot.VectorSpaces, vss)
		}
	}

	// Snapshot HNSW index if present
	if c.index != nil && c.indexType == IndexTypeHNSW {
		if hnswIdx, ok := c.index.(*hnswIndex); ok {
			snapshot.HNSWSnapshot = hnswIdx.internal().Snapshot()
		}
	}

	// Snapshot text index if present
	if c.textIndex != nil {
		snapshot.TextIndexSnapshot = c.textIndex.snapshot()
	}

	return snapshot
}

// loadFromSnapshot restores the collection from a snapshot.
func (c *Collection) loadFromSnapshot(snapshot *storage.CollectionSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dimension = snapshot.Dimension
	c.metadata = deepCopyMap(snapshot.Metadata)
	if c.metadata == nil {
		c.metadata = make(map[string]any)
	}
	c.distanceType = snapshot.DistanceType
	c.distanceFunc = floats.GetDistanceFunc(snapshot.DistanceType)
	c.higherBetter = floats.IsHigherBetter(snapshot.DistanceType)
	c.nextID = snapshot.NextID
	c.indexType = IndexType(snapshot.IndexType)
	c.hnswConfig = snapshot.HNSWConfig
	c.profile = profileFromSnapshot(snapshot.EmbeddingProfile)
	c.records = make(map[uint64]*Record, len(snapshot.Records))

	for _, rs := range snapshot.Records {
		var vectors map[string][]float32
		if len(rs.Vectors) > 0 {
			vectors = make(map[string][]float32, len(rs.Vectors))
			for name, vec := range rs.Vectors {
				vectors[name] = vec
			}
		}
		c.records[rs.ID] = &Record{
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

	// Restore additional named vector spaces and rebuild their indexes.
	c.spaces = make(map[string]*vectorSpace, len(snapshot.VectorSpaces))
	for _, vss := range snapshot.VectorSpaces {
		if vss == nil || vss.Name == "" || vss.Name == DefaultVectorSpace {
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
		if IndexType(vss.IndexType) == IndexTypeHNSW && vss.HNSWSnapshot != nil {
			idx := hnsw.LoadFromSnapshot(vss.HNSWSnapshot, vss.DistanceType)
			sp.index = &hnswIndex{idx: idx}
			c.setupSpaceVectorProvider(sp)
			idx.ClearInternalVectors()
		}
		c.spaces[sp.name] = sp
	}

	// Restore HNSW index if present
	if IndexType(snapshot.IndexType) == IndexTypeHNSW && snapshot.HNSWSnapshot != nil {
		idx := hnsw.LoadFromSnapshot(snapshot.HNSWSnapshot, snapshot.DistanceType)
		c.index = &hnswIndex{idx: idx}
		c.setupVectorProvider()
		// Clear the internal vectors map since the provider is now active
		idx.ClearInternalVectors()
	}

	// Restore text index if present
	if snapshot.TextIndexSnapshot != nil {
		c.textIndex = loadInvertedIndexFromSnapshot(snapshot.TextIndexSnapshot)
	}
}

// All returns all records in the collection.
func (c *Collection) All() []*Record {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make([]*Record, 0, len(c.records))
	for _, record := range c.records {
		results = append(results, record.Clone())
	}
	return results
}

// SearchFunc is a callback for streaming search results.
// Return false to stop receiving results.
type SearchFunc func(result Result) bool

// SearchStream performs a search and streams results to the callback function.
// For brute-force searches, results are yielded as they are found, enabling
// early termination without computing all scores. For HNSW searches, the
// full result set is fetched first since the index returns ordered results.
func (c *Collection) SearchStream(query []float32, fn SearchFunc, opts ...SearchOption) error {
	if len(query) == 0 {
		return ErrEmptyVector
	}

	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	effectiveK := config.effectiveTopK()

	// For brute-force, stream results as they are computed
	if c.index == nil || config.effectiveTopK() <= 0 {
		return c.searchStreamBruteForce(query, fn, config, effectiveK)
	}

	// For HNSW, fetch all results then stream them
	results, err := c.Search(query, opts...)
	if err != nil {
		return err
	}

	for _, r := range results {
		if !fn(r) {
			break
		}
	}
	return nil
}

// searchStreamBruteForce streams results from brute-force search without
// collecting all results first.
func (c *Collection) searchStreamBruteForce(query []float32, fn SearchFunc, config *searchConfig, effectiveK int) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.dimension != 0 && len(query) != c.dimension {
		return &DimensionError{Expected: c.dimension, Got: len(query)}
	}

	count := 0
	for _, record := range c.records {
		if len(record.Vector) == 0 {
			continue
		}

		if config.effectiveTopK() > 0 && count >= effectiveK {
			break
		}

		if !config.matchesFilters(record) {
			continue
		}

		score := c.distanceFunc(query, record.Vector)

		if config.threshold != nil {
			if c.higherBetter && score < *config.threshold {
				continue
			}
			if !c.higherBetter && score > *config.threshold {
				continue
			}
		}

		result := Result{
			Record: config.cloneRecordForResult(record),
			Score:  score,
		}

		if !fn(result) {
			return nil
		}
		count++
	}

	return nil
}

// TextSearch performs BM25 full-text search over indexed fields.
// Requires text indexing to be enabled via WithTextIndex.
func (c *Collection) TextSearch(query string, opts ...SearchOption) ([]Result, error) {
	if c.textIndex == nil {
		return nil, errors.New("veclite: text index not enabled on this collection")
	}
	if query == "" {
		return nil, errors.New("veclite: empty text query")
	}

	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	effectiveK := config.effectiveTopK()
	textResults := c.textIndex.search(query, effectiveK)

	results := make([]Result, 0, len(textResults))
	for _, tr := range textResults {
		record, ok := c.records[tr.id]
		if !ok {
			continue
		}

		// Apply payload filters
		if !config.matchesFilters(record) {
			continue
		}

		results = append(results, Result{
			Record: config.cloneRecordForResult(record),
			Score:  float32(tr.score),
		})
	}

	return config.applyPagination(results), nil
}

// HybridSearch performs both vector search and BM25 text search, then fuses
// results using Reciprocal Rank Fusion (RRF) with k=60.
// Requires text indexing to be enabled via WithTextIndex.
// Use WithVectorWeight and WithTextWeight to control the balance.
func (c *Collection) HybridSearch(query []float32, text string, opts ...SearchOption) ([]Result, error) {
	if c.textIndex == nil {
		return nil, errors.New("veclite: text index not enabled on this collection")
	}
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}
	if text == "" {
		return nil, errors.New("veclite: empty text query for hybrid search")
	}

	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	// Fetch more results from each source for better fusion
	fetchK := config.effectiveTopK() * 2
	if fetchK < 20 {
		fetchK = 20
	}

	// Run vector search
	vectorOpts := []SearchOption{TopK(fetchK)}
	vectorOpts = append(vectorOpts, WithContent(config.includeContent))
	if config.threshold != nil {
		vectorOpts = append(vectorOpts, Threshold(*config.threshold))
	}
	for _, f := range config.filters {
		vectorOpts = append(vectorOpts, WithFilter(f))
	}
	if config.efSearch > 0 {
		vectorOpts = append(vectorOpts, WithEfSearch(config.efSearch))
	}
	vectorResults, err := c.Search(query, vectorOpts...)
	if err != nil {
		return nil, err
	}

	// Run text search
	textOpts := []SearchOption{TopK(fetchK)}
	textOpts = append(textOpts, WithContent(config.includeContent))
	for _, f := range config.filters {
		textOpts = append(textOpts, WithFilter(f))
	}
	textResults, err := c.TextSearch(text, textOpts...)
	if err != nil {
		return nil, err
	}

	// Determine weights
	vectorWeight := 1.0
	textWeight := 1.0
	if config.vectorWeight > 0 {
		vectorWeight = config.vectorWeight
	}
	if config.textWeight > 0 {
		textWeight = config.textWeight
	}

	// Fuse with RRF
	fused := reciprocalRankFusion(
		[][]Result{vectorResults, textResults},
		60,
		[]float64{vectorWeight, textWeight},
	)

	// Apply pagination
	effectiveK := config.effectiveTopK()
	if len(fused) > effectiveK {
		fused = fused[:effectiveK]
	}

	return config.applyPagination(fused), nil
}

// InsertDocument inserts a vector with content text and payload.
// Content is automatically indexed for BM25 text search when text indexing is enabled.
func (c *Collection) InsertDocument(vector []float32, content string, payload map[string]any) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}
	if len(vector) == 0 {
		return 0, ErrEmptyVector
	}

	c.mu.Lock()

	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vector)
		c.initHNSWIfNeeded()
	} else if len(vector) != c.dimension {
		c.mu.Unlock()
		return 0, &DimensionError{Expected: c.dimension, Got: len(vector)}
	}

	now := time.Now()
	id := c.nextID
	c.nextID++

	record := &Record{
		ID:        id,
		Vector:    make([]float32, len(vector)),
		Payload:   payload,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
	copy(record.Vector, vector)

	c.records[id] = record

	if c.index != nil {
		if err := c.index.Insert(id, vector); err != nil {
			delete(c.records, id)
			c.nextID--
			c.mu.Unlock()
			return 0, err
		}
	}

	if c.textIndex != nil {
		c.textIndex.indexRecord(id, payload, content)
	}

	c.mu.Unlock()
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return id, nil
}

// InsertTextDocument inserts content and payload without a vector.
// Text-only records are indexed by BM25 when text indexing is enabled and are
// skipped by vector search, hybrid vector search, and vector subscriptions.
func (c *Collection) InsertTextDocument(content string, payload map[string]any) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}

	id := c.insertTextDocumentLocked(0, content, payload, nil)

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordInsert()
	}
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return id, nil
}

// InsertTextDocumentWithOptions inserts content and payload without a vector,
// applying insert options such as TTL and importance.
func (c *Collection) InsertTextDocumentWithOptions(content string, payload map[string]any, opts ...InsertOption) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}

	config := defaultInsertConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	id := c.insertTextDocumentLocked(0, content, payload, config)

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordInsert()
	}
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return id, nil
}

// UpsertTextDocument inserts or updates a text-only document by ID.
// If id is 0, a new record is created with an auto-generated ID.
func (c *Collection) UpsertTextDocument(id uint64, content string, payload map[string]any) (uint64, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, err
	}

	c.mu.Lock()
	if id != 0 {
		if record, ok := c.records[id]; ok {
			if c.index != nil && len(record.Vector) > 0 {
				c.hardDeleteFromIndex(id)
			}
			record.Vector = nil
			record.Content = content
			record.Payload = payload
			record.UpdatedAt = time.Now()
			c.reindexRecordLocked(record)
			c.mu.Unlock()
			c.syncIfNeeded()
			return id, nil
		}
	}
	id = c.insertTextDocumentUnlocked(id, content, payload, nil)
	c.mu.Unlock()

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordInsert()
	}
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return id, nil
}

// UpsertTextDocumentByKey inserts or updates a text-only document by payload key.
func (c *Collection) UpsertTextDocumentByKey(keyField string, keyValue any, content string, payload map[string]any) (uint64, bool, error) {
	if err := c.checkReadOnly(); err != nil {
		return 0, false, err
	}

	c.mu.Lock()
	for id, record := range c.records {
		if record.Payload == nil {
			continue
		}
		if v, ok := record.Payload[keyField]; ok && compareValues(v, keyValue) {
			if c.index != nil && len(record.Vector) > 0 {
				c.hardDeleteFromIndex(id)
			}
			record.Vector = nil
			record.Content = content
			record.Payload = payload
			record.UpdatedAt = time.Now()
			c.reindexRecordLocked(record)
			c.mu.Unlock()
			c.syncIfNeeded()
			return id, false, nil
		}
	}
	id := c.insertTextDocumentUnlocked(0, content, payload, nil)
	c.mu.Unlock()

	if c.db != nil && c.db.metrics != nil {
		c.db.metrics.recordInsert()
	}
	c.enforceMemoryLimitIfConfigured()
	c.syncIfNeeded()
	return id, true, nil
}

// insertTextDocumentLocked inserts a text-only record.
func (c *Collection) insertTextDocumentLocked(id uint64, content string, payload map[string]any, config *insertConfig) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.insertTextDocumentUnlocked(id, content, payload, config)
}

// insertTextDocumentUnlocked inserts a text-only record.
// Caller must hold c.mu.Lock().
func (c *Collection) insertTextDocumentUnlocked(id uint64, content string, payload map[string]any, config *insertConfig) uint64 {
	now := time.Now()
	if id == 0 {
		id = c.nextID
		c.nextID++
	} else if id >= c.nextID {
		c.nextID = id + 1
	}

	expiresAt := time.Time{}
	importance := float32(0)
	if config != nil {
		if config.content != "" {
			content = config.content
		}
		expiresAt = config.computeExpiresAt()
		importance = config.importance
	}

	record := &Record{
		ID:         id,
		Payload:    payload,
		Content:    content,
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  expiresAt,
		Importance: importance,
	}

	c.records[id] = record
	c.reindexRecordLocked(record)
	return id
}

// InsertText embeds the text using the configured embedder and inserts the result.
// Requires an embedder to be set via WithEmbedder.
func (c *Collection) InsertText(text string, payload map[string]any) (uint64, error) {
	if c.embedder == nil {
		return 0, ErrNoEmbedder
	}

	vector, err := c.embedder.Embed(text)
	if err != nil {
		return 0, err
	}

	return c.InsertDocument(vector, text, payload)
}

// SearchText embeds the text query using the configured embedder and searches.
// Requires an embedder to be set via WithEmbedder.
func (c *Collection) SearchText(text string, opts ...SearchOption) ([]Result, error) {
	if c.embedder == nil {
		return nil, ErrNoEmbedder
	}

	vector, err := c.embedder.Embed(text)
	if err != nil {
		return nil, err
	}

	return c.Search(vector, opts...)
}

// ForEach iterates over all records in the collection, calling fn for each.
// If fn returns false, iteration stops early.
// Records are cloned before being passed to fn.
func (c *Collection) ForEach(fn func(*Record) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, record := range c.records {
		if !fn(record.Clone()) {
			return
		}
	}
}

// IterOption configures the iterator.
type IterOption interface {
	apply(*iterConfig)
}

type iterConfig struct {
	offset int
	limit  int
}

type iterOptionFunc func(*iterConfig)

func (f iterOptionFunc) apply(c *iterConfig) {
	f(c)
}

// IterOffset sets the number of records to skip.
func IterOffset(n int) IterOption {
	return iterOptionFunc(func(c *iterConfig) {
		if n >= 0 {
			c.offset = n
		}
	})
}

// IterLimit sets the maximum number of records to return.
func IterLimit(n int) IterOption {
	return iterOptionFunc(func(c *iterConfig) {
		if n > 0 {
			c.limit = n
		}
	})
}

// Iterator allows iterating over collection records one at a time.
type Iterator struct {
	records []*Record
	pos     int
}

// Next returns the next record and true, or nil and false if done.
func (it *Iterator) Next() (*Record, bool) {
	if it.pos >= len(it.records) {
		return nil, false
	}
	r := it.records[it.pos]
	it.pos++
	return r, true
}

// Close releases resources held by the iterator.
func (it *Iterator) Close() {
	it.records = nil
	it.pos = 0
}

// Iterate returns an iterator over collection records.
// Options can control offset and limit for pagination.
func (c *Collection) Iterate(opts ...IterOption) *Iterator {
	cfg := &iterConfig{}
	for _, opt := range opts {
		opt.apply(cfg)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Collect and sort records by ID for deterministic order
	all := make([]*Record, 0, len(c.records))
	for _, record := range c.records {
		all = append(all, record.Clone())
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].ID < all[j].ID
	})

	// Apply offset
	if cfg.offset > 0 {
		if cfg.offset >= len(all) {
			return &Iterator{}
		}
		all = all[cfg.offset:]
	}

	// Apply limit
	if cfg.limit > 0 && cfg.limit < len(all) {
		all = all[:cfg.limit]
	}

	return &Iterator{records: all}
}

// Clear removes all records from the collection.
// It preserves the nextID counter to avoid ID reuse after reinsertion.
// Use Reset if you want to also reset the ID counter.
// Returns an error if the database is read-only.
func (c *Collection) Clear() error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()
	c.records = make(map[uint64]*Record)
	if c.index != nil {
		c.index.Clear()
	}
	if c.textIndex != nil {
		c.textIndex = newInvertedIndex(c.textIndex.fields)
	}
	// Keep dimension locked, don't reset nextID to avoid ID reuse
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// Reset removes all records from the collection and resets the ID counter to 1.
// Unlike Clear, this allows ID reuse which may be desirable for testing or
// when the collection is being fully repopulated.
// Returns an error if the database is read-only.
func (c *Collection) Reset() error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()
	c.records = make(map[uint64]*Record)
	if c.index != nil {
		c.index.Clear()
	}
	if c.textIndex != nil {
		c.textIndex = newInvertedIndex(c.textIndex.fields)
	}
	c.nextID = 1
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}
