package veclite

import (
	"sort"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// Collection represents a collection of vectors with the same dimension.
type Collection struct {
	name         string
	dimension    int
	distanceType floats.DistanceType
	distanceFunc floats.DistanceFunc
	higherBetter bool

	mu      sync.RWMutex
	records map[uint64]*Record
	nextID  uint64

	db *DB
}

// newCollection creates a new collection.
func newCollection(name string, config *collectionConfig, db *DB) *Collection {
	return &Collection{
		name:         name,
		dimension:    config.dimension,
		distanceType: config.distanceType,
		distanceFunc: floats.GetDistanceFunc(config.distanceType),
		higherBetter: floats.IsHigherBetter(config.distanceType),
		records:      make(map[uint64]*Record),
		nextID:       1,
		db:           db,
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
	if len(vector) == 0 {
		return 0, ErrEmptyVector
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vector)
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

	return id, nil
}

// InsertBatch adds multiple vectors with payloads to the collection.
// Returns the assigned record IDs.
// If payloads is nil or shorter than vectors, missing payloads are treated as nil.
func (c *Collection) InsertBatch(vectors [][]float32, payloads []map[string]any) ([]uint64, error) {
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

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check/set dimension
	if c.dimension == 0 {
		c.dimension = len(vectors[0])
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.records[id]; !ok {
		return &NotFoundError{Type: "record", ID: string(rune(id))}
	}

	delete(c.records, id)
	return nil
}

// DeleteWhere removes all records matching the filters.
// Returns the number of deleted records.
func (c *Collection) DeleteWhere(filters ...Filter) (int, error) {
	if len(filters) == 0 {
		return 0, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

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
			delete(c.records, id)
			deleted++
		}
	}

	return deleted, nil
}

// Update updates the payload for a record.
func (c *Collection) Update(id uint64, payload map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, ok := c.records[id]
	if !ok {
		return &NotFoundError{Type: "record", ID: string(rune(id))}
	}

	record.Payload = payload
	record.UpdatedAt = time.Now()
	return nil
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
	}
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
func (c *Collection) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = make(map[uint64]*Record)
	// Keep dimension locked, don't reset nextID to avoid ID reuse
}
