package veclite

import (
	"sort"
	"sync"
	"time"
)

// TTLCleanerCallback is called after each cleanup cycle with the number of deleted records.
type TTLCleanerCallback func(collection string, deleted int)

// TTLCleaner periodically removes expired records from collections.
type TTLCleaner struct {
	db       *DB
	interval time.Duration
	callback TTLCleanerCallback
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// StartTTLCleaner starts a background goroutine that periodically cleans up expired records
// from all collections in the database. Returns a function to stop the cleaner.
func (db *DB) StartTTLCleaner(interval time.Duration, callback TTLCleanerCallback) func() {
	cleaner := &TTLCleaner{
		db:       db,
		interval: interval,
		callback: callback,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	cleaner.start()
	db.registerStopFunc(cleaner.Stop)

	return cleaner.Stop
}

// start begins the cleanup goroutine.
func (tc *TTLCleaner) start() {
	tc.mu.Lock()
	if tc.running {
		tc.mu.Unlock()
		return
	}
	tc.running = true
	tc.mu.Unlock()

	go tc.run()
}

// Stop stops the TTL cleaner and waits for it to finish.
func (tc *TTLCleaner) Stop() {
	tc.mu.Lock()
	if !tc.running {
		tc.mu.Unlock()
		return
	}
	tc.mu.Unlock()

	close(tc.stopCh)
	<-tc.doneCh

	tc.mu.Lock()
	tc.running = false
	tc.mu.Unlock()
}

// run is the main cleanup loop.
func (tc *TTLCleaner) run() {
	defer close(tc.doneCh)

	ticker := time.NewTicker(tc.interval)
	defer ticker.Stop()

	// Run initial cleanup
	tc.cleanup()

	for {
		select {
		case <-tc.stopCh:
			return
		case <-ticker.C:
			tc.cleanup()
		}
	}
}

// cleanup performs a single cleanup cycle across all collections.
func (tc *TTLCleaner) cleanup() {
	names := tc.db.Collections()
	for _, name := range names {
		coll, err := tc.db.GetCollection(name)
		if err != nil {
			continue
		}

		deleted, err := coll.CleanupExpired()
		if err != nil {
			continue
		}

		if deleted > 0 && tc.callback != nil {
			tc.callback(name, deleted)
		}
	}
}

// MemoryConfig configures memory pressure handling for a collection.
type MemoryConfig struct {
	// MaxRecords is the maximum number of records allowed in the collection.
	// When exceeded, eviction is triggered.
	MaxRecords int

	// EvictionPolicy determines how records are selected for eviction.
	// Supported values: "lru", "fifo", "importance"
	EvictionPolicy string

	// EvictionBatchSize is the number of records to evict at once when pressure is detected.
	// Default is 10% of MaxRecords.
	EvictionBatchSize int

	// CleanupInterval is how often to check for memory pressure.
	// If zero, cleanup only happens on insert.
	CleanupInterval time.Duration
}

// MemoryLimiter manages memory pressure for a collection.
type MemoryLimiter struct {
	collection *Collection
	config     MemoryConfig
	stopCh     chan struct{}
	doneCh     chan struct{}
	mu         sync.Mutex
	running    bool
}

// WithMemoryLimits returns a collection option that enables memory pressure handling.
// When the collection exceeds MaxRecords, older/less important records are evicted.
func WithMemoryLimits(config MemoryConfig) CollectionOption {
	return collectionOptionFunc(func(c *collectionConfig) {
		c.memoryConfig = &config
	})
}

// StartMemoryLimiter starts background memory pressure monitoring for a collection.
// Returns a function to stop the limiter.
func (c *Collection) StartMemoryLimiter(config MemoryConfig) func() {
	limiter := &MemoryLimiter{
		collection: c,
		config:     config,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	// Set default batch size
	if limiter.config.EvictionBatchSize == 0 {
		limiter.config.EvictionBatchSize = config.MaxRecords / 10
		if limiter.config.EvictionBatchSize < 1 {
			limiter.config.EvictionBatchSize = 1
		}
	}

	// Start background monitor if interval is set
	if config.CleanupInterval > 0 {
		limiter.start()
	}
	c.db.registerStopFunc(limiter.Stop)

	return limiter.Stop
}

// start begins the memory monitoring goroutine.
func (ml *MemoryLimiter) start() {
	ml.mu.Lock()
	if ml.running {
		ml.mu.Unlock()
		return
	}
	ml.running = true
	ml.mu.Unlock()

	go ml.run()
}

// Stop stops the memory limiter and waits for it to finish.
func (ml *MemoryLimiter) Stop() {
	ml.mu.Lock()
	if !ml.running {
		ml.mu.Unlock()
		return
	}
	ml.mu.Unlock()

	close(ml.stopCh)
	<-ml.doneCh

	ml.mu.Lock()
	ml.running = false
	ml.mu.Unlock()
}

// run is the main monitoring loop.
func (ml *MemoryLimiter) run() {
	defer close(ml.doneCh)

	ticker := time.NewTicker(ml.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ml.stopCh:
			return
		case <-ticker.C:
			ml.enforceLimit()
		}
	}
}

// enforceLimit checks if the collection exceeds MaxRecords and evicts if necessary.
func (ml *MemoryLimiter) enforceLimit() int {
	count := ml.collection.Count()
	if count <= ml.config.MaxRecords {
		return 0
	}

	toEvict := count - ml.config.MaxRecords
	if toEvict > ml.config.EvictionBatchSize {
		toEvict = ml.config.EvictionBatchSize
	}

	return ml.evict(toEvict)
}

// evict removes n records according to the eviction policy.
func (ml *MemoryLimiter) evict(n int) int {
	ml.collection.mu.Lock()
	defer ml.collection.mu.Unlock()

	if n <= 0 || len(ml.collection.records) == 0 {
		return 0
	}

	// Collect all records
	records := make([]*Record, 0, len(ml.collection.records))
	for _, r := range ml.collection.records {
		records = append(records, r)
	}

	// Sort according to eviction policy
	switch ml.config.EvictionPolicy {
	case "lru":
		// Evict least recently accessed first
		sort.Slice(records, func(i, j int) bool {
			// Never accessed records first
			if records[i].LastAccessedAt.IsZero() && !records[j].LastAccessedAt.IsZero() {
				return true
			}
			if !records[i].LastAccessedAt.IsZero() && records[j].LastAccessedAt.IsZero() {
				return false
			}
			// Then by last accessed time
			return records[i].LastAccessedAt.Before(records[j].LastAccessedAt)
		})
	case "importance":
		// Evict least important first
		sort.Slice(records, func(i, j int) bool {
			return records[i].Importance < records[j].Importance
		})
	case "fifo":
		fallthrough
	default:
		// Evict oldest first
		sort.Slice(records, func(i, j int) bool {
			return records[i].CreatedAt.Before(records[j].CreatedAt)
		})
	}

	// Evict up to n records
	evicted := 0
	for i := 0; i < n && i < len(records); i++ {
		r := records[i]

		// Skip archived records
		if r.Payload != nil {
			if archived, ok := r.Payload[PayloadKeyArchived].(bool); ok && archived {
				continue
			}
		}

		// Delete from index
		if ml.collection.index != nil {
			_ = ml.collection.index.Delete(r.ID)
		}
		if ml.collection.textIndex != nil {
			ml.collection.textIndex.removeRecord(r.ID)
		}

		delete(ml.collection.records, r.ID)
		evicted++
	}

	return evicted
}

// EnforceMemoryLimit checks and enforces the memory limit for a collection.
// Call this after inserts if you want immediate enforcement without background monitoring.
func (c *Collection) EnforceMemoryLimit(config MemoryConfig) int {
	limiter := &MemoryLimiter{
		collection: c,
		config:     config,
	}

	if limiter.config.EvictionBatchSize == 0 {
		limiter.config.EvictionBatchSize = config.MaxRecords / 10
		if limiter.config.EvictionBatchSize < 1 {
			limiter.config.EvictionBatchSize = 1
		}
	}

	return limiter.enforceLimit()
}
