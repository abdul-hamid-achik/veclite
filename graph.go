package veclite

import (
	"fmt"
	"sort"
	"sync"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// Entity represents a node in the knowledge graph.
type Entity struct {
	// ID is the unique identifier for this entity.
	ID string
	// Type categorizes the entity (e.g., "person", "company", "concept").
	Type string
	// Name is the human-readable name of the entity.
	Name string
	// Vector is the embedding that represents this entity.
	Vector []float32
	// Properties contains additional entity attributes.
	Properties map[string]any
}

// Clone creates a copy of the entity.
func (e *Entity) Clone() *Entity {
	return &Entity{
		ID:         e.ID,
		Type:       e.Type,
		Name:       e.Name,
		Vector:     append([]float32(nil), e.Vector...),
		Properties: copyMap(e.Properties),
	}
}

// Relationship represents an edge between two entities in the knowledge graph.
type Relationship struct {
	// ID is the unique identifier for this relationship.
	ID string
	// SourceID is the ID of the source entity.
	SourceID string
	// TargetID is the ID of the target entity.
	TargetID string
	// Type describes the relationship (e.g., "works_at", "knows", "related_to").
	Type string
	// Weight is the strength of the relationship (0.0-1.0).
	Weight float32
	// Properties contains additional relationship attributes.
	Properties map[string]any
	// Bidirectional indicates if the relationship goes both ways.
	Bidirectional bool
}

// Clone creates a copy of the relationship.
func (r *Relationship) Clone() *Relationship {
	return &Relationship{
		ID:            r.ID,
		SourceID:      r.SourceID,
		TargetID:      r.TargetID,
		Type:          r.Type,
		Weight:        r.Weight,
		Properties:    copyMap(r.Properties),
		Bidirectional: r.Bidirectional,
	}
}

// TraversalConfig configures graph traversal behavior.
type TraversalConfig struct {
	// MaxDepth is the maximum number of hops from start nodes.
	MaxDepth int
	// MaxNodes is the maximum number of nodes to visit.
	MaxNodes int
	// MinWeight is the minimum relationship weight to follow.
	MinWeight float32
	// RelationshipTypes limits traversal to specific relationship types.
	// Empty means all types.
	RelationshipTypes []string
	// EntityTypes limits traversal to specific entity types.
	// Empty means all types.
	EntityTypes []string
	// Direction controls traversal direction: "outgoing", "incoming", or "both".
	Direction string
}

// TraversalResult contains the results of a graph traversal.
type TraversalResult struct {
	// Entities are the entities found during traversal.
	Entities []*Entity
	// Relationships are the relationships traversed.
	Relationships []*Relationship
	// Depths maps entity IDs to their depth from start nodes.
	Depths map[string]int
}

// ExpandedSearchResult contains search results with graph context.
type ExpandedSearchResult struct {
	// Entity is the primary search result.
	Entity *Entity
	// Score is the similarity score.
	Score float32
	// RelatedEntities are entities connected to the primary result.
	RelatedEntities []*Entity
	// Relationships are the connections to related entities.
	Relationships []*Relationship
}

// KnowledgeGraph provides a graph-based knowledge base with vector search.
type KnowledgeGraph struct {
	// db is the underlying VecLite database.
	db *DB
	// name is the name of this knowledge graph.
	name string
	// entities maps entity IDs to entities.
	entities map[string]*Entity
	// relationships maps relationship IDs to relationships.
	relationships map[string]*Relationship
	// outgoing maps entity IDs to outgoing relationship IDs.
	outgoing map[string][]string
	// incoming maps entity IDs to incoming relationship IDs.
	incoming map[string][]string
	// collection stores entity vectors for similarity search.
	collection *Collection
	// mu protects the graph data structures.
	mu sync.RWMutex
	// distanceFunc is used for similarity calculations.
	distanceFunc floats.DistanceFunc
	// higherBetter indicates if higher scores are better.
	higherBetter bool
}

// CreateKnowledgeGraph creates a new knowledge graph.
func (db *DB) CreateKnowledgeGraph(name string) (*KnowledgeGraph, error) {
	if name == "" {
		return nil, fmt.Errorf("veclite: knowledge graph name required")
	}

	db.mu.RLock()
	if kg, ok := db.knowledgeGraphs[name]; ok {
		db.mu.RUnlock()
		return kg, nil
	}
	db.mu.RUnlock()

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

	// Register with DB for persistence
	db.mu.Lock()
	db.knowledgeGraphs[name] = kg
	db.mu.Unlock()

	return kg, nil
}

func (db *DB) GetKnowledgeGraph(name string) (*KnowledgeGraph, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	kg, ok := db.knowledgeGraphs[name]
	if !ok {
		return nil, &NotFoundError{Type: "knowledge graph", ID: name}
	}
	return kg, nil
}

// Name returns the name of the knowledge graph.
func (kg *KnowledgeGraph) Name() string {
	return kg.name
}

// AddEntity adds an entity to the knowledge graph.
func (kg *KnowledgeGraph) AddEntity(entity Entity) error {
	if entity.ID == "" {
		return fmt.Errorf("veclite: entity ID required")
	}

	kg.mu.Lock()
	defer kg.mu.Unlock()

	if _, exists := kg.entities[entity.ID]; exists {
		return fmt.Errorf("veclite: entity %q already exists", entity.ID)
	}

	// Store the entity
	kg.entities[entity.ID] = entity.Clone()

	// If entity has a vector, store it in the collection for search
	if len(entity.Vector) > 0 {
		payload := map[string]any{
			"_entity_id":   entity.ID,
			"_entity_type": entity.Type,
			"_entity_name": entity.Name,
		}
		if entity.Properties != nil {
			for k, v := range entity.Properties {
				payload[k] = v
			}
		}
		_, err := kg.collection.Insert(entity.Vector, payload)
		if err != nil {
			delete(kg.entities, entity.ID)
			return fmt.Errorf("veclite: failed to index entity: %w", err)
		}
	}

	return nil
}

// UpdateEntity updates an existing entity.
func (kg *KnowledgeGraph) UpdateEntity(entity Entity) error {
	if entity.ID == "" {
		return fmt.Errorf("veclite: entity ID required")
	}

	kg.mu.Lock()
	defer kg.mu.Unlock()

	existing, exists := kg.entities[entity.ID]
	if !exists {
		return &NotFoundError{Type: "entity", ID: entity.ID}
	}

	// Update the entity
	kg.entities[entity.ID] = entity.Clone()

	// Update vector in collection if changed
	if len(entity.Vector) > 0 && !vectorsEqual(existing.Vector, entity.Vector) {
		// Find and update the record in collection
		records, _ := kg.collection.Find(Equal("_entity_id", entity.ID))
		if len(records) > 0 {
			if err := kg.collection.UpdateVector(records[0].ID, entity.Vector); err != nil {
				return fmt.Errorf("veclite: failed to update entity vector: %w", err)
			}
			payload := map[string]any{
				"_entity_id":   entity.ID,
				"_entity_type": entity.Type,
				"_entity_name": entity.Name,
			}
			if entity.Properties != nil {
				for k, v := range entity.Properties {
					payload[k] = v
				}
			}
			if err := kg.collection.Update(records[0].ID, payload); err != nil {
				return fmt.Errorf("veclite: failed to update entity payload: %w", err)
			}
		}
	}

	return nil
}

// GetEntity retrieves an entity by ID.
func (kg *KnowledgeGraph) GetEntity(entityID string) (*Entity, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	entity, ok := kg.entities[entityID]
	if !ok {
		return nil, &NotFoundError{Type: "entity", ID: entityID}
	}

	return entity.Clone(), nil
}

// DeleteEntity removes an entity and all its relationships.
func (kg *KnowledgeGraph) DeleteEntity(entityID string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if _, exists := kg.entities[entityID]; !exists {
		return &NotFoundError{Type: "entity", ID: entityID}
	}

	// Remove all relationships involving this entity (ignore errors during cleanup)
	for _, relID := range kg.outgoing[entityID] {
		_ = kg.deleteRelationshipInternal(relID)
	}
	for _, relID := range kg.incoming[entityID] {
		_ = kg.deleteRelationshipInternal(relID)
	}

	// Remove from collection (ignore error during cleanup)
	records, _ := kg.collection.Find(Equal("_entity_id", entityID))
	if len(records) > 0 {
		_ = kg.collection.Delete(records[0].ID)
	}

	// Remove the entity
	delete(kg.entities, entityID)
	delete(kg.outgoing, entityID)
	delete(kg.incoming, entityID)

	return nil
}

// ListEntities returns all entities, optionally filtered by type.
func (kg *KnowledgeGraph) ListEntities(entityType string) []*Entity {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	entities := make([]*Entity, 0, len(kg.entities))
	for _, e := range kg.entities {
		if entityType == "" || e.Type == entityType {
			entities = append(entities, e.Clone())
		}
	}

	sort.Slice(entities, func(i, j int) bool {
		return entities[i].ID < entities[j].ID
	})

	return entities
}

// AddRelationship adds a relationship between two entities.
func (kg *KnowledgeGraph) AddRelationship(rel Relationship) error {
	if rel.ID == "" {
		return fmt.Errorf("veclite: relationship ID required")
	}
	if rel.SourceID == "" || rel.TargetID == "" {
		return fmt.Errorf("veclite: relationship requires source and target")
	}

	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Verify entities exist
	if _, exists := kg.entities[rel.SourceID]; !exists {
		return &NotFoundError{Type: "entity", ID: rel.SourceID}
	}
	if _, exists := kg.entities[rel.TargetID]; !exists {
		return &NotFoundError{Type: "entity", ID: rel.TargetID}
	}

	if _, exists := kg.relationships[rel.ID]; exists {
		return fmt.Errorf("veclite: relationship %q already exists", rel.ID)
	}

	// Store the relationship
	kg.relationships[rel.ID] = rel.Clone()

	// Update adjacency lists
	kg.outgoing[rel.SourceID] = append(kg.outgoing[rel.SourceID], rel.ID)
	kg.incoming[rel.TargetID] = append(kg.incoming[rel.TargetID], rel.ID)

	// For bidirectional relationships, add reverse edge
	if rel.Bidirectional {
		kg.outgoing[rel.TargetID] = append(kg.outgoing[rel.TargetID], rel.ID)
		kg.incoming[rel.SourceID] = append(kg.incoming[rel.SourceID], rel.ID)
	}

	return nil
}

// GetRelationship retrieves a relationship by ID.
func (kg *KnowledgeGraph) GetRelationship(relID string) (*Relationship, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	rel, ok := kg.relationships[relID]
	if !ok {
		return nil, &NotFoundError{Type: "relationship", ID: relID}
	}

	return rel.Clone(), nil
}

// DeleteRelationship removes a relationship.
func (kg *KnowledgeGraph) DeleteRelationship(relID string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	return kg.deleteRelationshipInternal(relID)
}

// deleteRelationshipInternal removes a relationship (must be called with lock held).
func (kg *KnowledgeGraph) deleteRelationshipInternal(relID string) error {
	rel, exists := kg.relationships[relID]
	if !exists {
		return &NotFoundError{Type: "relationship", ID: relID}
	}

	// Remove from adjacency lists
	kg.outgoing[rel.SourceID] = removeString(kg.outgoing[rel.SourceID], relID)
	kg.incoming[rel.TargetID] = removeString(kg.incoming[rel.TargetID], relID)

	if rel.Bidirectional {
		kg.outgoing[rel.TargetID] = removeString(kg.outgoing[rel.TargetID], relID)
		kg.incoming[rel.SourceID] = removeString(kg.incoming[rel.SourceID], relID)
	}

	delete(kg.relationships, relID)
	return nil
}

// GetRelationships returns all relationships for an entity.
func (kg *KnowledgeGraph) GetRelationships(entityID string, direction string) []*Relationship {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	var relIDs []string

	switch direction {
	case "outgoing":
		relIDs = kg.outgoing[entityID]
	case "incoming":
		relIDs = kg.incoming[entityID]
	default: // "both"
		relIDs = append(relIDs, kg.outgoing[entityID]...)
		relIDs = append(relIDs, kg.incoming[entityID]...)
	}

	// Deduplicate
	seen := make(map[string]bool)
	rels := make([]*Relationship, 0, len(relIDs))
	for _, id := range relIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if rel, ok := kg.relationships[id]; ok {
			rels = append(rels, rel.Clone())
		}
	}

	return rels
}

// Traverse performs a graph traversal starting from the given entity IDs.
func (kg *KnowledgeGraph) Traverse(startIDs []string, config TraversalConfig) (*TraversalResult, error) {
	if len(startIDs) == 0 {
		return nil, fmt.Errorf("veclite: at least one start node required")
	}

	// Set defaults
	if config.MaxDepth <= 0 {
		config.MaxDepth = 3
	}
	if config.MaxNodes <= 0 {
		config.MaxNodes = 100
	}
	if config.Direction == "" {
		config.Direction = "both"
	}

	kg.mu.RLock()
	defer kg.mu.RUnlock()

	// Validate start nodes
	for _, id := range startIDs {
		if _, exists := kg.entities[id]; !exists {
			return nil, &NotFoundError{Type: "entity", ID: id}
		}
	}

	result := &TraversalResult{
		Depths: make(map[string]int),
	}

	// BFS traversal
	visited := make(map[string]bool)
	visitedRels := make(map[string]bool)
	queue := make([]struct {
		id    string
		depth int
	}, 0)

	// Initialize with start nodes
	for _, id := range startIDs {
		queue = append(queue, struct {
			id    string
			depth int
		}{id, 0})
		visited[id] = true
		result.Depths[id] = 0
	}

	for len(queue) > 0 && len(result.Entities) < config.MaxNodes {
		current := queue[0]
		queue = queue[1:]

		// Add entity to result
		if entity, ok := kg.entities[current.id]; ok {
			if kg.matchesEntityType(entity, config.EntityTypes) {
				result.Entities = append(result.Entities, entity.Clone())
			}
		}

		// Don't expand beyond max depth
		if current.depth >= config.MaxDepth {
			continue
		}

		// Get relationships to traverse
		var relIDs []string
		switch config.Direction {
		case "outgoing":
			relIDs = kg.outgoing[current.id]
		case "incoming":
			relIDs = kg.incoming[current.id]
		default:
			relIDs = append(relIDs, kg.outgoing[current.id]...)
			relIDs = append(relIDs, kg.incoming[current.id]...)
		}

		for _, relID := range relIDs {
			rel, ok := kg.relationships[relID]
			if !ok {
				continue
			}

			// Check relationship filters
			if rel.Weight < config.MinWeight {
				continue
			}
			if !kg.matchesRelType(rel, config.RelationshipTypes) {
				continue
			}

			// Add relationship to result
			if !visitedRels[relID] {
				visitedRels[relID] = true
				result.Relationships = append(result.Relationships, rel.Clone())
			}

			// Determine next node
			var nextID string
			if rel.SourceID == current.id {
				nextID = rel.TargetID
			} else if rel.TargetID == current.id {
				nextID = rel.SourceID
			} else {
				continue
			}

			// Visit next node if not already visited
			if !visited[nextID] {
				visited[nextID] = true
				result.Depths[nextID] = current.depth + 1
				queue = append(queue, struct {
					id    string
					depth int
				}{nextID, current.depth + 1})
			}
		}
	}

	return result, nil
}

// SearchWithExpansion searches for similar entities and expands results with graph context.
func (kg *KnowledgeGraph) SearchWithExpansion(query []float32, traversalConfig TraversalConfig, opts ...SearchOption) ([]ExpandedSearchResult, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}

	// Search for similar entities using vector search
	searchResults, err := kg.collection.Search(query, opts...)
	if err != nil {
		return nil, err
	}

	results := make([]ExpandedSearchResult, 0, len(searchResults))

	kg.mu.RLock()
	defer kg.mu.RUnlock()

	for _, sr := range searchResults {
		// Get entity ID from the record
		entityID, ok := sr.Record.Payload["_entity_id"].(string)
		if !ok {
			continue
		}

		entity, exists := kg.entities[entityID]
		if !exists {
			continue
		}

		expandedResult := ExpandedSearchResult{
			Entity: entity.Clone(),
			Score:  sr.Score,
		}

		// Expand with graph context
		if traversalConfig.MaxDepth > 0 {
			// Set default MaxNodes if not specified
			maxNodes := traversalConfig.MaxNodes
			if maxNodes <= 0 {
				maxNodes = 100
			}
			// Get immediate neighbors
			traversal, err := kg.traverseInternal([]string{entityID}, TraversalConfig{
				MaxDepth:          1, // Just immediate neighbors
				MaxNodes:          maxNodes,
				MinWeight:         traversalConfig.MinWeight,
				RelationshipTypes: traversalConfig.RelationshipTypes,
				EntityTypes:       traversalConfig.EntityTypes,
				Direction:         traversalConfig.Direction,
			})
			if err == nil {
				for _, e := range traversal.Entities {
					if e.ID != entityID {
						expandedResult.RelatedEntities = append(expandedResult.RelatedEntities, e)
					}
				}
				expandedResult.Relationships = traversal.Relationships
			}
		}

		results = append(results, expandedResult)
	}

	return results, nil
}

// traverseInternal performs traversal without acquiring locks (lock must be held).
func (kg *KnowledgeGraph) traverseInternal(startIDs []string, config TraversalConfig) (*TraversalResult, error) {
	result := &TraversalResult{
		Depths: make(map[string]int),
	}

	visited := make(map[string]bool)
	visitedRels := make(map[string]bool)
	queue := make([]struct {
		id    string
		depth int
	}, 0)

	for _, id := range startIDs {
		queue = append(queue, struct {
			id    string
			depth int
		}{id, 0})
		visited[id] = true
		result.Depths[id] = 0
	}

	for len(queue) > 0 && len(result.Entities) < config.MaxNodes {
		current := queue[0]
		queue = queue[1:]

		if entity, ok := kg.entities[current.id]; ok {
			if kg.matchesEntityType(entity, config.EntityTypes) {
				result.Entities = append(result.Entities, entity.Clone())
			}
		}

		if current.depth >= config.MaxDepth {
			continue
		}

		var relIDs []string
		switch config.Direction {
		case "outgoing":
			relIDs = kg.outgoing[current.id]
		case "incoming":
			relIDs = kg.incoming[current.id]
		default:
			relIDs = append(relIDs, kg.outgoing[current.id]...)
			relIDs = append(relIDs, kg.incoming[current.id]...)
		}

		for _, relID := range relIDs {
			rel, ok := kg.relationships[relID]
			if !ok || rel.Weight < config.MinWeight {
				continue
			}
			if !kg.matchesRelType(rel, config.RelationshipTypes) {
				continue
			}

			if !visitedRels[relID] {
				visitedRels[relID] = true
				result.Relationships = append(result.Relationships, rel.Clone())
			}

			var nextID string
			if rel.SourceID == current.id {
				nextID = rel.TargetID
			} else {
				nextID = rel.SourceID
			}

			if !visited[nextID] {
				visited[nextID] = true
				result.Depths[nextID] = current.depth + 1
				queue = append(queue, struct {
					id    string
					depth int
				}{nextID, current.depth + 1})
			}
		}
	}

	return result, nil
}

// Stats returns statistics about the knowledge graph.
func (kg *KnowledgeGraph) Stats() KnowledgeGraphStats {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	stats := KnowledgeGraphStats{
		EntityCount:       len(kg.entities),
		RelationshipCount: len(kg.relationships),
		EntityTypes:       make(map[string]int),
		RelationshipTypes: make(map[string]int),
	}

	for _, e := range kg.entities {
		stats.EntityTypes[e.Type]++
	}

	for _, r := range kg.relationships {
		stats.RelationshipTypes[r.Type]++
	}

	return stats
}

// KnowledgeGraphStats contains statistics about a knowledge graph.
type KnowledgeGraphStats struct {
	// EntityCount is the total number of entities.
	EntityCount int
	// RelationshipCount is the total number of relationships.
	RelationshipCount int
	// EntityTypes maps entity types to counts.
	EntityTypes map[string]int
	// RelationshipTypes maps relationship types to counts.
	RelationshipTypes map[string]int
}

// Helper functions

func (kg *KnowledgeGraph) matchesEntityType(entity *Entity, types []string) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if entity.Type == t {
			return true
		}
	}
	return false
}

func (kg *KnowledgeGraph) matchesRelType(rel *Relationship, types []string) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if rel.Type == t {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// snapshot creates a serializable snapshot of the knowledge graph.
func (kg *KnowledgeGraph) snapshot() *GraphSnapshot {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	snap := &GraphSnapshot{
		Name:          kg.name,
		Entities:      make([]EntitySnapshot, 0, len(kg.entities)),
		Relationships: make([]RelationshipSnapshot, 0, len(kg.relationships)),
		Outgoing:      make(map[string][]string, len(kg.outgoing)),
		Incoming:      make(map[string][]string, len(kg.incoming)),
	}

	for _, e := range kg.entities {
		snap.Entities = append(snap.Entities, EntitySnapshot{
			ID:         e.ID,
			Type:       e.Type,
			Name:       e.Name,
			Vector:     append([]float32(nil), e.Vector...),
			Properties: copyMap(e.Properties),
		})
	}

	for _, r := range kg.relationships {
		snap.Relationships = append(snap.Relationships, RelationshipSnapshot{
			ID:            r.ID,
			SourceID:      r.SourceID,
			TargetID:      r.TargetID,
			Type:          r.Type,
			Weight:        r.Weight,
			Properties:    copyMap(r.Properties),
			Bidirectional: r.Bidirectional,
		})
	}

	for k, v := range kg.outgoing {
		snap.Outgoing[k] = append([]string(nil), v...)
	}
	for k, v := range kg.incoming {
		snap.Incoming[k] = append([]string(nil), v...)
	}

	return snap
}

// loadFromSnapshot restores the knowledge graph from a snapshot.
func (kg *KnowledgeGraph) loadFromSnapshot(snap *GraphSnapshot) {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	kg.entities = make(map[string]*Entity, len(snap.Entities))
	kg.relationships = make(map[string]*Relationship, len(snap.Relationships))
	kg.outgoing = make(map[string][]string, len(snap.Outgoing))
	kg.incoming = make(map[string][]string, len(snap.Incoming))

	for _, e := range snap.Entities {
		kg.entities[e.ID] = &Entity{
			ID:         e.ID,
			Type:       e.Type,
			Name:       e.Name,
			Vector:     append([]float32(nil), e.Vector...),
			Properties: copyMap(e.Properties),
		}
	}

	for _, r := range snap.Relationships {
		kg.relationships[r.ID] = &Relationship{
			ID:            r.ID,
			SourceID:      r.SourceID,
			TargetID:      r.TargetID,
			Type:          r.Type,
			Weight:        r.Weight,
			Properties:    copyMap(r.Properties),
			Bidirectional: r.Bidirectional,
		}
	}

	for k, v := range snap.Outgoing {
		kg.outgoing[k] = append([]string(nil), v...)
	}
	for k, v := range snap.Incoming {
		kg.incoming[k] = append([]string(nil), v...)
	}
}

func vectorsEqual(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
