package veclite

import (
	"sort"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
	"github.com/abdul-hamid-achik/veclite/internal/hnsw"
)

// Collection represents a collection of vectors with the same dimension.
type Collection struct {
	name         string
	dimension    int
	distanceType floats.DistanceType
	distanceFunc floats.DistanceFunc
	higherBetter bool
	indexType    IndexType
	hnswConfig   *HNSWConfig
	index        Index

	mu      sync.RWMutex
	records map[uint64]*Record
	nextID  uint64

	db *DB
}

// newCollection creates a new collection.
func newCollection(name string, config *collectionConfig, db *DB) *Collection {
	c := &Collection{
		name:         name,
		dimension:    config.dimension,
		distanceType: config.distanceType,
		distanceFunc: floats.GetDistanceFunc(config.distanceType),
		higherBetter: floats.IsHigherBetter(config.distanceType),
		indexType:    config.indexType,
		hnswConfig:   config.hnswConfig,
		records:      make(map[uint64]*Record),
		nextID:       1,
		db:           db,
	}

	// Initialize index if HNSW is configured
	if config.indexType == IndexTypeHNSW && config.hnswConfig != nil {
		c.index = NewHNSWIndex(
			config.dimension,
			config.distanceType,
			config.hnswConfig.M,
			config.hnswConfig.EfConstruction,
		)
		c.setupVectorProvider()
	}

	return c
}

// initHNSWIfNeeded initializes the HNSW index when the dimension becomes known.
// Must be called with the collection lock held.
func (c *Collection) initHNSWIfNeeded() {
	if c.indexType == IndexTypeHNSW && c.hnswConfig != nil && c.index == nil && c.dimension > 0 {
		c.index = NewHNSWIndex(
			c.dimension,
			c.distanceType,
			c.hnswConfig.M,
			c.hnswConfig.EfConstruction,
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
	hnswIdx, ok := c.index.(*HNSWIndex)
	if !ok {
		return
	}
	hnswIdx.Internal().SetVectorProvider(func(id uint64) ([]float32, bool) {
		rec, ok := c.records[id]
		if !ok {
			return nil, false
		}
		return rec.Vector, true
	})
}

// checkReadOnly returns ErrReadOnly if the database is in read-only mode.
func (c *Collection) checkReadOnly() error {
	if c.db != nil && c.db.config != nil && c.db.config.readOnly {
		return ErrReadOnly
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

// Name returns the collection name.
func (c *Collection) Name() string {
	return c.name
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

	c.syncIfNeeded()
	return id, nil
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
				}
				c.nextID -= uint64(i + 1)
				return nil, err
			}
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
		return nil, &NotFoundError{Type: "record", ID: string(rune(id))}
	}

	return record.Clone(), nil
}

// GetVector retrieves just the vector for a record.
func (c *Collection) GetVector(id uint64) ([]float32, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, ok := c.records[id]
	if !ok {
		return nil, &NotFoundError{Type: "record", ID: string(rune(id))}
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
		return &NotFoundError{Type: "record", ID: string(rune(id))}
	}

	// Delete from index first (soft delete)
	if c.index != nil {
		_ = c.index.Delete(id)
	}

	delete(c.records, id)
	c.mu.Unlock()

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
		return &NotFoundError{Type: "record", ID: string(rune(id))}
	}

	record.Payload = payload
	record.UpdatedAt = time.Now()
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
		return &NotFoundError{Type: "record", ID: string(rune(id))}
	}

	if len(vector) != c.dimension {
		c.mu.Unlock()
		return &DimensionError{Expected: c.dimension, Got: len(vector)}
	}

	// Update index if present
	if c.index != nil {
		// Hard delete old vector from index (needed for re-insertion with same ID)
		c.hardDeleteFromIndex(id)
		// Insert new vector
		if err := c.index.Insert(id, vector); err != nil {
			// Re-insert old vector on failure
			_ = c.index.Insert(id, record.Vector)
			c.mu.Unlock()
			return err
		}
	}

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
	if hnswIdx, ok := c.index.(*HNSWIndex); ok {
		_ = hnswIdx.HardDelete(id)
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
		if len(vector) != c.dimension {
			return 0, &DimensionError{Expected: c.dimension, Got: len(vector)}
		}

		// Update index if present
		if c.index != nil {
			c.hardDeleteFromIndex(id)
			if err := c.index.Insert(id, vector); err != nil {
				_ = c.index.Insert(id, record.Vector)
				return 0, err
			}
		}

		copy(record.Vector, vector)
		record.Payload = payload
		record.UpdatedAt = time.Now()
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
		if c.dimension > 0 && len(vector) != c.dimension {
			return 0, false, &DimensionError{Expected: c.dimension, Got: len(vector)}
		}

		// Update index if present
		if c.index != nil {
			c.hardDeleteFromIndex(existingID)
			if err := c.index.Insert(existingID, vector); err != nil {
				_ = c.index.Insert(existingID, existingRecord.Vector)
				return 0, false, err
			}
		}

		copy(existingRecord.Vector, vector)
		existingRecord.Payload = payload
		existingRecord.UpdatedAt = time.Now()
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
		if c.indexType == IndexTypeHNSW && c.hnswConfig != nil && c.index == nil {
			c.index = NewHNSWIndex(
				c.dimension,
				c.distanceType,
				c.hnswConfig.M,
				c.hnswConfig.EfConstruction,
			)
		}
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

	return id, nil
}

// insertWithIDUnlocked inserts a record with a specific ID without acquiring the lock.
// Caller must hold c.mu.Lock().
func (c *Collection) insertWithIDUnlocked(id uint64, vector []float32, payload map[string]any) (uint64, error) {
	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vector)
		if c.indexType == IndexTypeHNSW && c.hnswConfig != nil && c.index == nil {
			c.index = NewHNSWIndex(
				c.dimension,
				c.distanceType,
				c.hnswConfig.M,
				c.hnswConfig.EfConstruction,
			)
		}
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

	return id, nil
}

// Search finds the most similar vectors to the query vector.
func (c *Collection) Search(query []float32, opts ...SearchOption) ([]Result, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.dimension > 0 && len(query) != c.dimension {
		return nil, &DimensionError{Expected: c.dimension, Got: len(query)}
	}

	// Apply options
	config := defaultSearchConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	// Use HNSW index if available
	if c.index != nil {
		results, err := c.searchWithIndex(query, config)
		if err != nil {
			return nil, err
		}
		// If we got enough results (or no filters were applied), return them.
		// Otherwise, fall back to brute force for completeness.
		if len(results) >= config.topK || len(config.filters) == 0 {
			return results, nil
		}
	}

	// Fall back to brute force search (no index, or insufficient filtered results from HNSW)
	return c.searchBruteForce(query, config)
}

// searchWithIndex performs search using the HNSW index.
// When filters are present, over-fetches from HNSW and post-filters the results.
func (c *Collection) searchWithIndex(query []float32, config *searchConfig) ([]Result, error) {
	// Determine ef parameter
	ef := config.efSearch
	if ef == 0 && c.hnswConfig != nil {
		ef = c.hnswConfig.EfSearch
	}

	// Request more candidates when filters or threshold are set,
	// to ensure we get enough after post-filtering.
	requestK := config.topK
	hasFilters := len(config.filters) > 0
	if hasFilters || config.threshold != nil {
		requestK = max(config.topK*4, 100)
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

	// Convert index results to full results with records, applying post-filters
	results := make([]Result, 0, len(indexResults))
	for _, ir := range indexResults {
		record, ok := c.records[ir.ID]
		if !ok {
			continue // Record was deleted
		}

		// Apply payload filters (post-filter HNSW results)
		if !config.matchesFilters(record) {
			continue
		}

		// Apply threshold filter
		if config.threshold != nil {
			if c.higherBetter && ir.Distance < *config.threshold {
				continue
			}
			if !c.higherBetter && ir.Distance > *config.threshold {
				continue
			}
		}

		results = append(results, Result{
			Record: record.Clone(),
			Score:  ir.Distance,
		})

		// Stop once we have enough results
		if len(results) >= config.topK {
			break
		}
	}

	return results, nil
}

// searchBruteForce performs brute-force search.
func (c *Collection) searchBruteForce(query []float32, config *searchConfig) ([]Result, error) {
	// Collect matching results
	results := make([]Result, 0)
	for _, record := range c.records {
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
			Record: record.Clone(),
			Score:  score,
		})
	}

	// Sort results
	if c.higherBetter {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score < results[j].Score
		})
	}

	// Apply topK
	if len(results) > config.topK {
		results = results[:config.topK]
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

	return CollectionStats{
		Name:         c.name,
		Count:        len(c.records),
		Dimension:    c.dimension,
		DistanceType: string(c.distanceType),
		IndexType:    string(c.indexType),
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
func (c *Collection) snapshot() *CollectionSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := &CollectionSnapshot{
		Name:         c.name,
		Dimension:    c.dimension,
		DistanceType: c.distanceType,
		NextID:       c.nextID,
		Records:      make([]*RecordSnapshot, 0, len(c.records)),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		IndexType:    c.indexType,
		HNSWConfig:   c.hnswConfig,
	}

	for _, record := range c.records {
		snapshot.Records = append(snapshot.Records, &RecordSnapshot{
			ID:        record.ID,
			Vector:    record.Vector,
			Payload:   record.Payload,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
		})
	}

	// Snapshot HNSW index if present
	if c.index != nil && c.indexType == IndexTypeHNSW {
		if hnswIdx, ok := c.index.(*HNSWIndex); ok {
			snapshot.HNSWSnapshot = hnswIdx.Internal().Snapshot()
		}
	}

	return snapshot
}

// loadFromSnapshot restores the collection from a snapshot.
func (c *Collection) loadFromSnapshot(snapshot *CollectionSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dimension = snapshot.Dimension
	c.distanceType = snapshot.DistanceType
	c.distanceFunc = floats.GetDistanceFunc(snapshot.DistanceType)
	c.higherBetter = floats.IsHigherBetter(snapshot.DistanceType)
	c.nextID = snapshot.NextID
	c.indexType = snapshot.IndexType
	c.hnswConfig = snapshot.HNSWConfig
	c.records = make(map[uint64]*Record, len(snapshot.Records))

	for _, rs := range snapshot.Records {
		c.records[rs.ID] = &Record{
			ID:        rs.ID,
			Vector:    rs.Vector,
			Payload:   rs.Payload,
			CreatedAt: rs.CreatedAt,
			UpdatedAt: rs.UpdatedAt,
		}
	}

	// Restore HNSW index if present
	if snapshot.IndexType == IndexTypeHNSW && snapshot.HNSWSnapshot != nil {
		idx := hnsw.LoadFromSnapshot(snapshot.HNSWSnapshot, snapshot.DistanceType)
		c.index = &HNSWIndex{idx: idx}
		c.setupVectorProvider()
		// Clear the internal vectors map since the provider is now active
		idx.ClearInternalVectors()
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

// Clear removes all records from the collection.
// Returns an error if the database is read-only.
func (c *Collection) Clear() error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()
	c.records = make(map[uint64]*Record)
	// Keep dimension locked, don't reset nextID to avoid ID reuse
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}
