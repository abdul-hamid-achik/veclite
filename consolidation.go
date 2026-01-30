package veclite

import (
	"fmt"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// Reserved payload keys for memory consolidation.
const (
	// PayloadKeyArchived indicates if a record has been archived.
	PayloadKeyArchived = "_archived"
	// PayloadKeyConsolidationGroup is the ID of the consolidation group.
	PayloadKeyConsolidationGroup = "_consolidation_group"
	// PayloadKeyConsolidatedFrom contains IDs of records this was consolidated from.
	PayloadKeyConsolidatedFrom = "_consolidated_from"
	// PayloadKeyIsConsolidation indicates this record is a consolidation of others.
	PayloadKeyIsConsolidation = "_is_consolidation"
)

// MemoryCluster represents a group of similar memories that could be consolidated.
type MemoryCluster struct {
	// ID is a unique identifier for this cluster.
	ID string
	// Records are the records in this cluster.
	Records []*Record
	// Centroid is the average vector of all records in the cluster.
	Centroid []float32
	// AverageImportance is the average importance score of records.
	AverageImportance float32
	// TimeRange is the span from oldest to newest record.
	TimeRange TimeRange
}

// TimeRange represents a time span.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Duration returns the duration of the time range.
func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// ConsolidationConfig configures memory consolidation.
type ConsolidationConfig struct {
	// SimilarityThreshold is the minimum similarity for records to be grouped (0.0-1.0).
	// Higher values mean stricter grouping.
	SimilarityThreshold float32
	// MinGroupSize is the minimum number of records to form a cluster.
	MinGroupSize int
	// MaxGroupSize is the maximum number of records in a cluster.
	MaxGroupSize int
	// SummaryGenerator is a function that creates a summary from a group of records.
	// It returns the summary text, additional payload, and any error.
	// If nil, no consolidation records are created (only clustering is performed).
	SummaryGenerator func([]*Record) (string, map[string]any, error)
	// Embedder is used to generate embeddings for consolidated summaries.
	// Required if SummaryGenerator is provided.
	Embedder Embedder
	// ArchiveOriginals determines whether to archive original records after consolidation.
	ArchiveOriginals bool
	// Filters can be used to limit which records are considered for consolidation.
	Filters []Filter
}

// ConsolidationResult contains the results of a consolidation operation.
type ConsolidationResult struct {
	// ClustersFound is the number of clusters identified.
	ClustersFound int
	// RecordsConsolidated is the total number of records that were consolidated.
	RecordsConsolidated int
	// ConsolidatedRecordIDs are the IDs of newly created consolidation records.
	ConsolidatedRecordIDs []uint64
	// ArchivedRecordIDs are the IDs of records that were archived.
	ArchivedRecordIDs []uint64
	// Clusters contains details about each cluster found.
	Clusters []MemoryCluster
}

// unionFind implements the Union-Find (Disjoint Set Union) data structure.
type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	uf := &unionFind{
		parent: make([]int, n),
		rank:   make([]int, n),
	}
	for i := range uf.parent {
		uf.parent[i] = i
	}
	return uf
}

func (uf *unionFind) find(x int) int {
	if uf.parent[x] != x {
		uf.parent[x] = uf.find(uf.parent[x]) // Path compression
	}
	return uf.parent[x]
}

func (uf *unionFind) union(x, y int) {
	rootX := uf.find(x)
	rootY := uf.find(y)
	if rootX == rootY {
		return
	}
	// Union by rank
	if uf.rank[rootX] < uf.rank[rootY] {
		uf.parent[rootX] = rootY
	} else if uf.rank[rootX] > uf.rank[rootY] {
		uf.parent[rootY] = rootX
	} else {
		uf.parent[rootY] = rootX
		uf.rank[rootX]++
	}
}

// FindSimilarClusters identifies clusters of similar memories using single-linkage clustering.
func (c *Collection) FindSimilarClusters(config ConsolidationConfig) ([]MemoryCluster, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Set defaults
	if config.SimilarityThreshold <= 0 {
		config.SimilarityThreshold = 0.85
	}
	if config.MinGroupSize <= 0 {
		config.MinGroupSize = 2
	}
	if config.MaxGroupSize <= 0 {
		config.MaxGroupSize = 10
	}

	// Collect eligible records
	var records []*Record
	for _, record := range c.records {
		// Skip archived records
		if record.Payload != nil {
			if archived, ok := record.Payload[PayloadKeyArchived].(bool); ok && archived {
				continue
			}
			// Skip consolidation records
			if isConsolidation, ok := record.Payload[PayloadKeyIsConsolidation].(bool); ok && isConsolidation {
				continue
			}
		}

		// Apply filters
		if len(config.Filters) > 0 {
			matches := true
			for _, f := range config.Filters {
				if !f.Match(record) {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
		}

		records = append(records, record)
	}

	if len(records) < config.MinGroupSize {
		return nil, nil
	}

	// Get distance function
	distFunc := floats.GetDistanceFunc(c.distanceType)
	higherBetter := floats.IsHigherBetter(c.distanceType)

	// Build Union-Find structure using single-linkage clustering
	uf := newUnionFind(len(records))

	for i := 0; i < len(records); i++ {
		for j := i + 1; j < len(records); j++ {
			if len(records[i].Vector) != len(records[j].Vector) {
				continue
			}

			similarity := distFunc(records[i].Vector, records[j].Vector)

			// Check if similar enough
			var isSimilar bool
			if higherBetter {
				isSimilar = similarity >= config.SimilarityThreshold
			} else {
				// For distance metrics (lower is better), convert threshold
				isSimilar = similarity <= (1 - config.SimilarityThreshold)
			}

			if isSimilar {
				uf.union(i, j)
			}
		}
	}

	// Group records by their root
	groups := make(map[int][]*Record)
	for i, record := range records {
		root := uf.find(i)
		groups[root] = append(groups[root], record)
	}

	// Build clusters from groups that meet size requirements
	var clusters []MemoryCluster
	clusterID := 0

	for _, group := range groups {
		if len(group) < config.MinGroupSize {
			continue
		}

		// Limit to max group size (take most important)
		if len(group) > config.MaxGroupSize {
			// Sort by importance (descending)
			sortByImportance(group)
			group = group[:config.MaxGroupSize]
		}

		cluster := MemoryCluster{
			ID:       fmt.Sprintf("cluster_%d", clusterID),
			Records:  group,
			Centroid: computeCentroid(group),
		}

		// Calculate average importance
		var totalImportance float32
		for _, r := range group {
			totalImportance += r.Importance
		}
		cluster.AverageImportance = totalImportance / float32(len(group))

		// Calculate time range
		cluster.TimeRange = computeTimeRange(group)

		clusters = append(clusters, cluster)
		clusterID++
	}

	return clusters, nil
}

// Consolidate finds similar memory clusters and optionally creates consolidated records.
func (c *Collection) Consolidate(config ConsolidationConfig) (*ConsolidationResult, error) {
	if err := c.checkReadOnly(); err != nil {
		return nil, err
	}

	// Find clusters
	clusters, err := c.FindSimilarClusters(config)
	if err != nil {
		return nil, err
	}

	result := &ConsolidationResult{
		ClustersFound: len(clusters),
		Clusters:      clusters,
	}

	if len(clusters) == 0 {
		return result, nil
	}

	// If we have a summary generator, create consolidation records
	if config.SummaryGenerator != nil {
		if config.Embedder == nil {
			return nil, fmt.Errorf("veclite: embedder required when SummaryGenerator is provided")
		}

		for _, cluster := range clusters {
			// Generate summary
			summary, extraPayload, err := config.SummaryGenerator(cluster.Records)
			if err != nil {
				return result, fmt.Errorf("veclite: summary generation failed: %w", err)
			}

			// Generate embedding for summary
			vector, err := config.Embedder.Embed(summary)
			if err != nil {
				return result, fmt.Errorf("veclite: embedding summary failed: %w", err)
			}

			// Build payload for consolidation record
			payload := make(map[string]any)
			for k, v := range extraPayload {
				payload[k] = v
			}

			// Add consolidation metadata
			payload[PayloadKeyIsConsolidation] = true
			payload[PayloadKeyConsolidationGroup] = cluster.ID

			// Store original record IDs
			originalIDs := make([]uint64, len(cluster.Records))
			for i, r := range cluster.Records {
				originalIDs[i] = r.ID
			}
			payload[PayloadKeyConsolidatedFrom] = originalIDs

			// Calculate combined importance (max of all records)
			var maxImportance float32
			for _, r := range cluster.Records {
				if r.Importance > maxImportance {
					maxImportance = r.Importance
				}
			}

			// Insert consolidation record
			id, err := c.InsertWithOptions(vector, payload,
				WithImportance(maxImportance),
				WithContentOption(summary),
			)
			if err != nil {
				return result, fmt.Errorf("veclite: inserting consolidation record failed: %w", err)
			}

			result.ConsolidatedRecordIDs = append(result.ConsolidatedRecordIDs, id)
			result.RecordsConsolidated += len(cluster.Records)

			// Archive originals if requested
			if config.ArchiveOriginals {
				for _, r := range cluster.Records {
					if err := c.ArchiveRecord(r.ID); err != nil {
						// Continue on archive errors
						continue
					}
					result.ArchivedRecordIDs = append(result.ArchivedRecordIDs, r.ID)
				}
			}
		}
	}

	return result, nil
}

// ArchiveRecord marks a record as archived.
// Archived records are excluded from normal searches but can be retrieved with GetArchived.
func (c *Collection) ArchiveRecord(id uint64) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()

	record, ok := c.records[id]
	if !ok {
		c.mu.Unlock()
		return &NotFoundError{Type: "record", ID: fmt.Sprintf("%d", id)}
	}

	if record.Payload == nil {
		record.Payload = make(map[string]any)
	}
	record.Payload[PayloadKeyArchived] = true
	record.UpdatedAt = time.Now()
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// UnarchiveRecord removes the archived flag from a record.
func (c *Collection) UnarchiveRecord(id uint64) error {
	if err := c.checkReadOnly(); err != nil {
		return err
	}

	c.mu.Lock()

	record, ok := c.records[id]
	if !ok {
		c.mu.Unlock()
		return &NotFoundError{Type: "record", ID: fmt.Sprintf("%d", id)}
	}

	if record.Payload != nil {
		delete(record.Payload, PayloadKeyArchived)
		record.UpdatedAt = time.Now()
	}
	c.mu.Unlock()

	c.syncIfNeeded()
	return nil
}

// GetArchived retrieves all archived records.
func (c *Collection) GetArchived() ([]*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var archived []*Record
	for _, record := range c.records {
		if record.Payload == nil {
			continue
		}
		if isArchived, ok := record.Payload[PayloadKeyArchived].(bool); ok && isArchived {
			archived = append(archived, record.Clone())
		}
	}

	return archived, nil
}

// GetConsolidations retrieves all consolidation records.
func (c *Collection) GetConsolidations() ([]*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var consolidations []*Record
	for _, record := range c.records {
		if record.Payload == nil {
			continue
		}
		if isConsolidation, ok := record.Payload[PayloadKeyIsConsolidation].(bool); ok && isConsolidation {
			consolidations = append(consolidations, record.Clone())
		}
	}

	return consolidations, nil
}

// ExpandConsolidation retrieves the original records that were consolidated into a consolidation record.
func (c *Collection) ExpandConsolidation(consolidationID uint64) ([]*Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, ok := c.records[consolidationID]
	if !ok {
		return nil, &NotFoundError{Type: "record", ID: fmt.Sprintf("%d", consolidationID)}
	}

	if record.Payload == nil {
		return nil, fmt.Errorf("veclite: record %d is not a consolidation", consolidationID)
	}

	isConsolidation, ok := record.Payload[PayloadKeyIsConsolidation].(bool)
	if !ok || !isConsolidation {
		return nil, fmt.Errorf("veclite: record %d is not a consolidation", consolidationID)
	}

	originalIDs := getConsolidatedFromIDs(record.Payload)
	if len(originalIDs) == 0 {
		return nil, nil
	}

	var records []*Record
	for _, id := range originalIDs {
		if r, ok := c.records[id]; ok {
			records = append(records, r.Clone())
		}
	}

	return records, nil
}

// Helper functions

func sortByImportance(records []*Record) {
	for i := 0; i < len(records)-1; i++ {
		for j := i + 1; j < len(records); j++ {
			if records[j].Importance > records[i].Importance {
				records[i], records[j] = records[j], records[i]
			}
		}
	}
}

func computeCentroid(records []*Record) []float32 {
	if len(records) == 0 {
		return nil
	}

	dim := len(records[0].Vector)
	if dim == 0 {
		return nil
	}

	centroid := make([]float32, dim)
	count := 0

	for _, r := range records {
		if len(r.Vector) != dim {
			continue
		}
		for i, v := range r.Vector {
			centroid[i] += v
		}
		count++
	}

	if count > 0 {
		for i := range centroid {
			centroid[i] /= float32(count)
		}
	}

	return centroid
}

func computeTimeRange(records []*Record) TimeRange {
	if len(records) == 0 {
		return TimeRange{}
	}

	tr := TimeRange{
		Start: records[0].CreatedAt,
		End:   records[0].CreatedAt,
	}

	for _, r := range records[1:] {
		if r.CreatedAt.Before(tr.Start) {
			tr.Start = r.CreatedAt
		}
		if r.CreatedAt.After(tr.End) {
			tr.End = r.CreatedAt
		}
	}

	return tr
}

func getConsolidatedFromIDs(payload map[string]any) []uint64 {
	if payload == nil {
		return nil
	}

	ids, ok := payload[PayloadKeyConsolidatedFrom]
	if !ok {
		return nil
	}

	switch v := ids.(type) {
	case []uint64:
		return v
	case []any:
		result := make([]uint64, 0, len(v))
		for _, id := range v {
			switch i := id.(type) {
			case uint64:
				result = append(result, i)
			case int64:
				result = append(result, uint64(i))
			case float64:
				result = append(result, uint64(i))
			case int:
				result = append(result, uint64(i))
			}
		}
		return result
	}

	return nil
}
