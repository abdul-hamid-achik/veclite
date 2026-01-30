// Package openclaw provides a high-level agent memory interface built on VecLite.
// It is designed for use with OpenClaw and similar AI assistants.
package openclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/veclite"
)

const (
	// DefaultCollection is the default collection name for memories.
	DefaultCollection = "memories"
)

// Config holds configuration for the Memory instance.
type Config struct {
	// DBPath is the path to the VecLite database file.
	// Use ":memory:" for an in-memory database.
	DBPath string

	// Embedder is the embedding function for text-to-vector conversion.
	// Required for text-based operations.
	Embedder veclite.Embedder

	// Collection is the name of the collection to use for memories.
	// Defaults to "memories".
	Collection string

	// DefaultTTL is the default time-to-live for memories.
	// Zero means no expiration.
	DefaultTTL time.Duration

	// DefaultImportance is the default importance score for new memories.
	// Defaults to 0.5.
	DefaultImportance float32
}

// Memory provides a high-level interface for agent memory operations.
type Memory struct {
	db         *veclite.DB
	collection *veclite.Collection
	embedder   veclite.Embedder
	config     Config
}

// New creates a new Memory instance with the given configuration.
func New(cfg Config) (*Memory, error) {
	if cfg.Collection == "" {
		cfg.Collection = DefaultCollection
	}
	if cfg.DefaultImportance <= 0 || cfg.DefaultImportance > 1 {
		cfg.DefaultImportance = 0.5
	}

	db, err := veclite.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("openclaw: failed to open database: %w", err)
	}

	coll := db.Collection(cfg.Collection)

	return &Memory{
		db:         db,
		collection: coll,
		embedder:   cfg.Embedder,
		config:     cfg,
	}, nil
}

// Close closes the memory database.
func (m *Memory) Close() error {
	return m.db.Close()
}

// RememberOptions configures a remember operation.
type RememberOptions struct {
	// Importance is the importance score (0.0-1.0).
	// Zero uses the default importance.
	Importance float32

	// Tags are optional tags for categorization.
	Tags []string

	// TTL is the time-to-live for this memory.
	// Zero uses the default TTL (no expiration if default is also zero).
	TTL time.Duration

	// Metadata is additional metadata to store with the memory.
	Metadata map[string]any

	// Vector is an optional pre-computed embedding vector.
	// If provided, the embedder is not called.
	Vector []float32
}

// Remember stores a memory with the given content and options.
// Returns the memory ID.
func (m *Memory) Remember(content string, opts RememberOptions) (uint64, error) {
	if content == "" {
		return 0, fmt.Errorf("openclaw: content cannot be empty")
	}

	// Get or compute embedding
	var vector []float32
	if len(opts.Vector) > 0 {
		vector = opts.Vector
	} else if m.embedder != nil {
		var err error
		vector, err = m.embedder.Embed(content)
		if err != nil {
			return 0, fmt.Errorf("openclaw: embedding failed: %w", err)
		}
	} else {
		return 0, fmt.Errorf("openclaw: no embedder configured and no vector provided")
	}

	// Build payload
	payload := make(map[string]any)
	if opts.Metadata != nil {
		for k, v := range opts.Metadata {
			payload[k] = v
		}
	}
	if len(opts.Tags) > 0 {
		payload["_tags"] = opts.Tags
	}

	// Build insert options
	insertOpts := []veclite.InsertOption{
		veclite.WithContentOption(content),
	}

	// Set importance
	importance := opts.Importance
	if importance <= 0 {
		importance = m.config.DefaultImportance
	}
	insertOpts = append(insertOpts, veclite.WithImportance(importance))

	// Set TTL
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = m.config.DefaultTTL
	}
	if ttl > 0 {
		insertOpts = append(insertOpts, veclite.WithTTL(ttl))
	}

	id, err := m.collection.InsertWithOptions(vector, payload, insertOpts...)
	if err != nil {
		return 0, fmt.Errorf("openclaw: insert failed: %w", err)
	}

	_ = m.db.Sync()
	return id, nil
}

// RecallOptions configures a recall operation.
type RecallOptions struct {
	// Limit is the maximum number of memories to return.
	// Defaults to 10.
	Limit int

	// MinImportance filters memories by minimum importance.
	MinImportance float32

	// Tags filters memories that have any of these tags.
	Tags []string

	// Since filters memories created within this duration.
	Since time.Duration

	// IncludeExpired includes expired memories in results.
	IncludeExpired bool

	// DecayType applies temporal decay to scores.
	DecayType veclite.DecayType

	// DecayHalfLife is the half-life for decay calculations.
	DecayHalfLife time.Duration

	// ImportanceBoost boosts importance in scoring.
	ImportanceBoost float32

	// TrackAccess updates access counts for returned memories.
	TrackAccess bool

	// Vector is an optional pre-computed query vector.
	// If provided, the embedder is not called.
	Vector []float32
}

// MemoryEntry represents a recalled memory.
type MemoryEntry struct {
	// ID is the unique identifier.
	ID uint64

	// Content is the text content.
	Content string

	// Score is the similarity score.
	Score float32

	// Importance is the importance score.
	Importance float32

	// Tags are the memory's tags.
	Tags []string

	// Metadata is additional metadata.
	Metadata map[string]any

	// CreatedAt is when the memory was created.
	CreatedAt time.Time

	// ExpiresAt is when the memory expires (zero if no TTL).
	ExpiresAt time.Time

	// AccessCount is how many times this memory was accessed.
	AccessCount uint64

	// LastAccessedAt is when this memory was last accessed.
	LastAccessedAt time.Time
}

// Recall retrieves memories matching the query.
func (m *Memory) Recall(query string, opts RecallOptions) ([]MemoryEntry, error) {
	if query == "" && len(opts.Vector) == 0 {
		return nil, fmt.Errorf("openclaw: query or vector required")
	}

	// Get or compute embedding
	var vector []float32
	if len(opts.Vector) > 0 {
		vector = opts.Vector
	} else if m.embedder != nil {
		var err error
		vector, err = m.embedder.Embed(query)
		if err != nil {
			return nil, fmt.Errorf("openclaw: embedding failed: %w", err)
		}
	} else {
		return nil, fmt.Errorf("openclaw: no embedder configured and no vector provided")
	}

	// Build search options
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	searchOpts := []veclite.SearchOption{veclite.TopK(limit)}

	// Build filters
	var filters []veclite.Filter

	if opts.MinImportance > 0 {
		filters = append(filters, veclite.ImportanceAbove(opts.MinImportance))
	}

	if opts.Since > 0 {
		filters = append(filters, veclite.AgeNewerThan(opts.Since))
	}

	if !opts.IncludeExpired {
		filters = append(filters, veclite.NotExpired())
	}

	for _, f := range filters {
		searchOpts = append(searchOpts, veclite.WithFilter(f))
	}

	// Add decay and importance boost if specified
	if opts.DecayType != "" && opts.DecayType != veclite.DecayNone {
		searchOpts = append(searchOpts, veclite.WithDecay(opts.DecayType, opts.DecayHalfLife))
	}

	if opts.ImportanceBoost > 0 {
		searchOpts = append(searchOpts, veclite.WithImportanceBoost(opts.ImportanceBoost))
	}

	if opts.TrackAccess {
		searchOpts = append(searchOpts, veclite.WithAccessTracking(true))
	}

	// Perform search
	results, err := m.collection.Search(vector, searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("openclaw: search failed: %w", err)
	}

	// Post-filter by tags if specified
	if len(opts.Tags) > 0 {
		filtered := make([]veclite.Result, 0, len(results))
		for _, r := range results {
			if hasAnyTag(r.Record.Payload, opts.Tags) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Convert to MemoryEntry
	entries := make([]MemoryEntry, len(results))
	for i, r := range results {
		entry := MemoryEntry{
			ID:             r.Record.ID,
			Content:        r.Record.Content,
			Score:          r.Score,
			Importance:     r.Record.Importance,
			CreatedAt:      r.Record.CreatedAt,
			ExpiresAt:      r.Record.ExpiresAt,
			AccessCount:    r.Record.AccessCount,
			LastAccessedAt: r.Record.LastAccessedAt,
		}

		if r.Record.Payload != nil {
			if tags, ok := r.Record.Payload["_tags"]; ok {
				if tagSlice, ok := tags.([]string); ok {
					entry.Tags = tagSlice
				} else if tagAny, ok := tags.([]any); ok {
					entry.Tags = make([]string, 0, len(tagAny))
					for _, t := range tagAny {
						if s, ok := t.(string); ok {
							entry.Tags = append(entry.Tags, s)
						}
					}
				}
			}

			// Copy metadata excluding internal fields
			entry.Metadata = make(map[string]any)
			for k, v := range r.Record.Payload {
				if k != "_tags" {
					entry.Metadata[k] = v
				}
			}
		}

		entries[i] = entry
	}

	return entries, nil
}

// RecallRecent retrieves the most recent memories.
func (m *Memory) RecallRecent(limit int, opts RecallOptions) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build filters
	var filters []veclite.Filter

	if opts.MinImportance > 0 {
		filters = append(filters, veclite.ImportanceAbove(opts.MinImportance))
	}

	if !opts.IncludeExpired {
		filters = append(filters, veclite.NotExpired())
	}

	// Get all matching records
	records, err := m.collection.Find(filters...)
	if err != nil {
		return nil, fmt.Errorf("openclaw: find failed: %w", err)
	}

	// Sort by creation time (most recent first)
	for i := 0; i < len(records)-1; i++ {
		for j := i + 1; j < len(records); j++ {
			if records[j].CreatedAt.After(records[i].CreatedAt) {
				records[i], records[j] = records[j], records[i]
			}
		}
	}

	// Apply tag filter
	if len(opts.Tags) > 0 {
		filtered := make([]*veclite.Record, 0, len(records))
		for _, r := range records {
			if hasAnyTag(r.Payload, opts.Tags) {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}

	// Apply limit
	if len(records) > limit {
		records = records[:limit]
	}

	// Convert to MemoryEntry
	entries := make([]MemoryEntry, len(records))
	for i, r := range records {
		entry := MemoryEntry{
			ID:             r.ID,
			Content:        r.Content,
			Importance:     r.Importance,
			CreatedAt:      r.CreatedAt,
			ExpiresAt:      r.ExpiresAt,
			AccessCount:    r.AccessCount,
			LastAccessedAt: r.LastAccessedAt,
		}

		if r.Payload != nil {
			if tags, ok := r.Payload["_tags"]; ok {
				if tagSlice, ok := tags.([]string); ok {
					entry.Tags = tagSlice
				}
			}
			entry.Metadata = make(map[string]any)
			for k, v := range r.Payload {
				if k != "_tags" {
					entry.Metadata[k] = v
				}
			}
		}

		entries[i] = entry
	}

	return entries, nil
}

// ForgetOptions configures a forget operation.
type ForgetOptions struct {
	// OlderThan deletes memories older than this duration.
	OlderThan time.Duration

	// Tags deletes memories with any of these tags.
	Tags []string

	// ExpiredOnly only deletes expired memories.
	ExpiredOnly bool

	// BelowImportance deletes memories with importance below this threshold.
	BelowImportance float32
}

// Forget removes memories matching the criteria.
// Returns the number of memories deleted.
func (m *Memory) Forget(opts ForgetOptions) (int, error) {
	var filters []veclite.Filter

	if opts.ExpiredOnly {
		filters = append(filters, veclite.FilterFunc(func(r *veclite.Record) bool {
			return r.IsExpired()
		}))
	}

	if opts.OlderThan > 0 {
		filters = append(filters, veclite.AgeOlderThan(opts.OlderThan))
	}

	if opts.BelowImportance > 0 {
		filters = append(filters, veclite.ImportanceBelow(opts.BelowImportance))
	}

	if len(opts.Tags) > 0 {
		filters = append(filters, veclite.FilterFunc(func(r *veclite.Record) bool {
			return hasAnyTag(r.Payload, opts.Tags)
		}))
	}

	if len(filters) == 0 {
		return 0, fmt.Errorf("openclaw: at least one filter criteria is required")
	}

	deleted, err := m.collection.DeleteWhere(filters...)
	if err != nil {
		return 0, fmt.Errorf("openclaw: delete failed: %w", err)
	}

	if deleted > 0 {
		_ = m.db.Sync()
	}

	return deleted, nil
}

// CleanupExpired removes all expired memories.
// Returns the number of memories deleted.
func (m *Memory) CleanupExpired() (int, error) {
	deleted, err := m.collection.CleanupExpired()
	if err != nil {
		return 0, fmt.Errorf("openclaw: cleanup failed: %w", err)
	}
	return deleted, nil
}

// ExportMarkdown exports all memories to markdown files in the specified directory.
func (m *Memory) ExportMarkdown(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("openclaw: failed to create output directory: %w", err)
	}

	records := m.collection.All()

	for _, r := range records {
		filename := fmt.Sprintf("memory_%d.md", r.ID)
		path := filepath.Join(outputDir, filename)

		content := fmt.Sprintf("# Memory %d\n\n", r.ID)
		content += fmt.Sprintf("**Created:** %s\n\n", r.CreatedAt.Format(time.RFC3339))
		content += fmt.Sprintf("**Importance:** %.2f\n\n", r.Importance)

		if !r.ExpiresAt.IsZero() {
			content += fmt.Sprintf("**Expires:** %s\n\n", r.ExpiresAt.Format(time.RFC3339))
		}

		if r.Payload != nil {
			if tags, ok := r.Payload["_tags"]; ok {
				content += fmt.Sprintf("**Tags:** %v\n\n", tags)
			}
		}

		content += "## Content\n\n"
		content += r.Content + "\n"

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("openclaw: failed to write file %s: %w", path, err)
		}
	}

	return nil
}

// Stats returns statistics about the memory store.
type MemoryStats struct {
	// TotalMemories is the total number of memories.
	TotalMemories int

	// ExpiredMemories is the number of expired memories.
	ExpiredMemories int

	// AverageImportance is the average importance score.
	AverageImportance float32

	// TotalAccessCount is the total access count across all memories.
	TotalAccessCount uint64
}

// Stats returns statistics about the memory store.
func (m *Memory) Stats() MemoryStats {
	records := m.collection.All()

	stats := MemoryStats{
		TotalMemories: len(records),
	}

	var totalImportance float64
	for _, r := range records {
		if r.IsExpired() {
			stats.ExpiredMemories++
		}
		totalImportance += float64(r.Importance)
		stats.TotalAccessCount += r.AccessCount
	}

	if len(records) > 0 {
		stats.AverageImportance = float32(totalImportance / float64(len(records)))
	}

	return stats
}

// hasAnyTag checks if a payload has any of the specified tags.
func hasAnyTag(payload map[string]any, tags []string) bool {
	if payload == nil {
		return false
	}
	tagsVal, ok := payload["_tags"]
	if !ok {
		return false
	}

	switch t := tagsVal.(type) {
	case []string:
		for _, pt := range t {
			for _, tag := range tags {
				if pt == tag {
					return true
				}
			}
		}
	case []any:
		for _, pt := range t {
			if ptStr, ok := pt.(string); ok {
				for _, tag := range tags {
					if ptStr == tag {
						return true
					}
				}
			}
		}
	}
	return false
}
