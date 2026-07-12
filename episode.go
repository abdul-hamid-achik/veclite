package veclite

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// Episode represents a coherent group of related memories forming a discrete experience.
type Episode struct {
	// ID is the unique identifier for this episode.
	ID string
	// Title is a human-readable summary of the episode.
	Title string
	// Vector is the embedding that represents the episode (centroid of contained records).
	Vector []float32
	// TimeRange is the span from the first to last record in the episode.
	TimeRange TimeRange
	// RecordIDs are the IDs of records that belong to this episode.
	RecordIDs []uint64
	// CreatedAt is when the episode was created.
	CreatedAt time.Time
	// Metadata contains additional episode information.
	Metadata map[string]any
}

// Duration returns the duration of the episode.
func (e *Episode) Duration() time.Duration {
	return e.TimeRange.Duration()
}

// RecordCount returns the number of records in the episode.
func (e *Episode) RecordCount() int {
	return len(e.RecordIDs)
}

// EpisodeConfig configures episode detection.
type EpisodeConfig struct {
	// TimeGapThreshold is the maximum time gap between records in the same episode.
	// Records separated by more than this are considered part of different episodes.
	TimeGapThreshold time.Duration
	// MinRecords is the minimum number of records to form an episode.
	MinRecords int
	// MaxRecords is the maximum number of records in an episode.
	MaxRecords int
	// SimilarityThreshold is the minimum similarity for records to be grouped.
	// Used when temporal clustering alone isn't sufficient.
	SimilarityThreshold float32
	// Filters can be used to limit which records are considered for episodes.
	Filters []Filter
}

// EpisodeResult represents a search result that includes episode context.
type EpisodeResult struct {
	// Result is the underlying search result.
	Result Result
	// Episode is the episode containing this result (if any).
	Episode *Episode
	// EpisodeRecords are other records in the same episode.
	EpisodeRecords []*Record
}

// EpisodeStore manages episodes for a collection.
type EpisodeStore struct {
	// collection is the underlying memories collection.
	collection *Collection
	// episodes stores detected episodes.
	episodes map[string]*Episode
	// mu protects the episodes map.
	mu sync.RWMutex
	// distanceFunc is used for similarity calculations.
	distanceFunc floats.DistanceFunc
	// higherBetter indicates if higher scores are better.
	higherBetter bool
}

// CreateEpisodeStore creates a new episode store for a collection.
func (db *DB) CreateEpisodeStore(memoriesCollectionName string) (*EpisodeStore, error) {
	coll := db.Collection(memoriesCollectionName)
	if coll == nil {
		return nil, fmt.Errorf("veclite: collection %q not found", memoriesCollectionName)
	}

	es := &EpisodeStore{
		collection:   coll,
		episodes:     make(map[string]*Episode),
		distanceFunc: floats.GetDistanceFunc(coll.distanceType),
		higherBetter: floats.IsHigherBetter(coll.distanceType),
	}

	// Register with DB for persistence
	db.mu.Lock()
	db.episodeStores[memoriesCollectionName] = es
	db.mu.Unlock()

	// Log the (empty) store so it survives a crash before its first episode.
	db.walAppendEpisodeStore(memoriesCollectionName, es)

	return es, nil
}

// syncIfNeeded persists a completed episode mutation according to the DB's
// durability mode (full save on syncOnWrite, WAL append when the log is
// active). It must be called with no episode-store lock held: the
// syncOnWrite path re-acquires es.mu through the DB snapshot.
func (es *EpisodeStore) syncIfNeeded() {
	coll := es.collection
	if coll == nil || coll.db == nil || coll.db.config == nil {
		return
	}
	db := coll.db
	if db.config.syncOnWrite {
		_ = db.Sync()
		return
	}
	db.walAppendEpisodeStore(coll.name, es)
}

func (db *DB) GetEpisodeStore(name string) (*EpisodeStore, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	es, ok := db.episodeStores[name]
	if !ok {
		return nil, &NotFoundError{Type: "episode store", ID: name}
	}
	return es, nil
}

// CreateEpisode manually creates an episode from a set of record IDs.
func (es *EpisodeStore) CreateEpisode(recordIDs []uint64, title string) (*Episode, error) {
	if len(recordIDs) == 0 {
		return nil, fmt.Errorf("veclite: episode requires at least one record")
	}

	// Verify all records exist and collect them
	var records []*Record
	es.collection.mu.RLock()
	for _, id := range recordIDs {
		if r, ok := es.collection.records[id]; ok {
			records = append(records, r)
		} else {
			es.collection.mu.RUnlock()
			return nil, &NotFoundError{Type: "record", ID: fmt.Sprintf("%d", id)}
		}
	}
	es.collection.mu.RUnlock()

	// Generate episode ID
	episodeID := generateEpisodeID()

	episode := &Episode{
		ID:        episodeID,
		Title:     title,
		Vector:    computeCentroid(records),
		TimeRange: computeTimeRange(records),
		RecordIDs: recordIDs,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}

	es.mu.Lock()
	es.episodes[episodeID] = episode
	es.mu.Unlock()

	es.syncIfNeeded()
	return episode, nil
}

// DetectEpisodes automatically detects episodes using temporal and similarity clustering.
func (es *EpisodeStore) DetectEpisodes(config EpisodeConfig) ([]*Episode, error) {
	// Set defaults
	if config.TimeGapThreshold == 0 {
		config.TimeGapThreshold = 30 * time.Minute
	}
	if config.MinRecords <= 0 {
		config.MinRecords = 2
	}
	if config.MaxRecords <= 0 {
		config.MaxRecords = 100
	}

	// Get all eligible records
	es.collection.mu.RLock()
	var records []*Record
	for _, r := range es.collection.records {
		// Skip archived
		if r.Payload != nil {
			if archived, ok := r.Payload[PayloadKeyArchived].(bool); ok && archived {
				continue
			}
		}

		// Apply filters
		if len(config.Filters) > 0 {
			matches := true
			for _, f := range config.Filters {
				if !f.Match(r) {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
		}

		records = append(records, r)
	}
	es.collection.mu.RUnlock()

	if len(records) < config.MinRecords {
		return nil, nil
	}

	// Sort by creation time
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})

	// Temporal clustering: group records that are close in time
	var episodes []*Episode
	var currentGroup []*Record
	currentGroup = append(currentGroup, records[0])

	for i := 1; i < len(records); i++ {
		gap := records[i].CreatedAt.Sub(records[i-1].CreatedAt)

		if gap <= config.TimeGapThreshold && len(currentGroup) < config.MaxRecords {
			currentGroup = append(currentGroup, records[i])
		} else {
			// Finalize current group if it meets minimum size
			if len(currentGroup) >= config.MinRecords {
				episode := es.createEpisodeFromGroup(currentGroup)
				episodes = append(episodes, episode)
			}
			// Start new group
			currentGroup = []*Record{records[i]}
		}
	}

	// Don't forget the last group
	if len(currentGroup) >= config.MinRecords {
		episode := es.createEpisodeFromGroup(currentGroup)
		episodes = append(episodes, episode)
	}

	// Store detected episodes
	es.mu.Lock()
	for _, ep := range episodes {
		es.episodes[ep.ID] = ep
	}
	es.mu.Unlock()

	if len(episodes) > 0 {
		es.syncIfNeeded()
	}
	return episodes, nil
}

// createEpisodeFromGroup creates an episode from a group of records.
func (es *EpisodeStore) createEpisodeFromGroup(records []*Record) *Episode {
	recordIDs := make([]uint64, len(records))
	for i, r := range records {
		recordIDs[i] = r.ID
	}

	return &Episode{
		ID:        generateEpisodeID(),
		Vector:    computeCentroid(records),
		TimeRange: computeTimeRange(records),
		RecordIDs: recordIDs,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// ExpandEpisode retrieves all records belonging to an episode.
func (es *EpisodeStore) ExpandEpisode(episodeID string) ([]*Record, error) {
	es.mu.RLock()
	episode, ok := es.episodes[episodeID]
	es.mu.RUnlock()

	if !ok {
		return nil, &NotFoundError{Type: "episode", ID: episodeID}
	}

	var records []*Record
	es.collection.mu.RLock()
	for _, id := range episode.RecordIDs {
		if r, ok := es.collection.records[id]; ok {
			records = append(records, r.Clone())
		}
	}
	es.collection.mu.RUnlock()

	// Sort by creation time
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})

	return records, nil
}

// GetEpisode retrieves an episode by ID.
func (es *EpisodeStore) GetEpisode(episodeID string) (*Episode, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	episode, ok := es.episodes[episodeID]
	if !ok {
		return nil, &NotFoundError{Type: "episode", ID: episodeID}
	}

	// Return a copy
	return &Episode{
		ID:        episode.ID,
		Title:     episode.Title,
		Vector:    append([]float32(nil), episode.Vector...),
		TimeRange: episode.TimeRange,
		RecordIDs: append([]uint64(nil), episode.RecordIDs...),
		CreatedAt: episode.CreatedAt,
		Metadata:  copyMap(episode.Metadata),
	}, nil
}

// ListEpisodes returns all episodes.
func (es *EpisodeStore) ListEpisodes() []*Episode {
	es.mu.RLock()
	defer es.mu.RUnlock()

	episodes := make([]*Episode, 0, len(es.episodes))
	for _, ep := range es.episodes {
		episodes = append(episodes, ep)
	}

	// Sort by creation time
	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].CreatedAt.Before(episodes[j].CreatedAt)
	})

	return episodes
}

// DeleteEpisode removes an episode (does not delete the underlying records).
func (es *EpisodeStore) DeleteEpisode(episodeID string) error {
	es.mu.Lock()
	if _, ok := es.episodes[episodeID]; !ok {
		es.mu.Unlock()
		return &NotFoundError{Type: "episode", ID: episodeID}
	}
	delete(es.episodes, episodeID)
	es.mu.Unlock()

	es.syncIfNeeded()
	return nil
}

// SearchWithEpisodeExpansion performs a search and includes episode context for results.
func (es *EpisodeStore) SearchWithEpisodeExpansion(query []float32, opts ...SearchOption) ([]EpisodeResult, error) {
	// Perform regular search
	results, err := es.collection.Search(query, opts...)
	if err != nil {
		return nil, err
	}

	// Build a map of record ID to episode
	es.mu.RLock()
	recordToEpisode := make(map[uint64]*Episode)
	for _, ep := range es.episodes {
		for _, rid := range ep.RecordIDs {
			recordToEpisode[rid] = ep
		}
	}
	es.mu.RUnlock()

	// Enhance results with episode context
	episodeResults := make([]EpisodeResult, len(results))
	for i, result := range results {
		episodeResults[i] = EpisodeResult{
			Result: result,
		}

		if episode, ok := recordToEpisode[result.Record.ID]; ok {
			episodeResults[i].Episode = episode

			// Expand episode records
			es.collection.mu.RLock()
			for _, rid := range episode.RecordIDs {
				if rid != result.Record.ID {
					if r, ok := es.collection.records[rid]; ok {
						episodeResults[i].EpisodeRecords = append(episodeResults[i].EpisodeRecords, r.Clone())
					}
				}
			}
			es.collection.mu.RUnlock()
		}
	}

	return episodeResults, nil
}

// SearchEpisodes searches for episodes by their vector representation.
func (es *EpisodeStore) SearchEpisodes(query []float32, limit int) ([]*Episode, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}
	if limit <= 0 {
		limit = 10
	}

	es.mu.RLock()
	defer es.mu.RUnlock()

	type scoredEpisode struct {
		episode *Episode
		score   float32
	}

	var scored []scoredEpisode
	for _, ep := range es.episodes {
		if len(ep.Vector) != len(query) {
			continue
		}
		score := es.distanceFunc(query, ep.Vector)
		scored = append(scored, scoredEpisode{episode: ep, score: score})
	}

	// Sort by score
	if es.higherBetter {
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].score > scored[j].score
		})
	} else {
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].score < scored[j].score
		})
	}

	// Limit results
	if len(scored) > limit {
		scored = scored[:limit]
	}

	episodes := make([]*Episode, len(scored))
	for i, s := range scored {
		episodes[i] = s.episode
	}

	return episodes, nil
}

// FindRecordEpisode finds the episode containing a specific record (if any).
func (es *EpisodeStore) FindRecordEpisode(recordID uint64) (*Episode, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	for _, ep := range es.episodes {
		for _, rid := range ep.RecordIDs {
			if rid == recordID {
				return ep, nil
			}
		}
	}

	return nil, nil // No episode found (not an error)
}

// Episode ID generation
var episodeCounter uint64
var episodeCounterMu sync.Mutex

func generateEpisodeID() string {
	episodeCounterMu.Lock()
	episodeCounter++
	id := episodeCounter
	episodeCounterMu.Unlock()

	return fmt.Sprintf("ep_%s_%d", time.Now().Format("20060102150405"), id)
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	return deepCopyMap(m)
}

// snapshot creates a serializable snapshot of the episode store.
func (es *EpisodeStore) snapshot() *EpisodeStoreSnapshot {
	es.mu.RLock()
	defer es.mu.RUnlock()

	snap := &EpisodeStoreSnapshot{
		CollectionName: es.collection.name,
		Episodes:       make([]EpisodeSnapshot, 0, len(es.episodes)),
	}

	for _, ep := range es.episodes {
		snap.Episodes = append(snap.Episodes, EpisodeSnapshot{
			ID:        ep.ID,
			Title:     ep.Title,
			Vector:    append([]float32(nil), ep.Vector...),
			TimeRange: TimeRangeSnapshot{Start: ep.TimeRange.Start, End: ep.TimeRange.End},
			RecordIDs: append([]uint64(nil), ep.RecordIDs...),
			CreatedAt: ep.CreatedAt,
			Metadata:  copyMap(ep.Metadata),
		})
	}

	return snap
}

// loadFromSnapshot restores the episode store from a snapshot.
func (es *EpisodeStore) loadFromSnapshot(snap *EpisodeStoreSnapshot) {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.episodes = make(map[string]*Episode, len(snap.Episodes))

	for _, ep := range snap.Episodes {
		es.episodes[ep.ID] = &Episode{
			ID:        ep.ID,
			Title:     ep.Title,
			Vector:    append([]float32(nil), ep.Vector...),
			TimeRange: TimeRange{Start: ep.TimeRange.Start, End: ep.TimeRange.End},
			RecordIDs: append([]uint64(nil), ep.RecordIDs...),
			CreatedAt: ep.CreatedAt,
			Metadata:  copyMap(ep.Metadata),
		}
	}
}
