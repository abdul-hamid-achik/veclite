package veclite

import (
	"testing"
	"time"
)

// TC-GRAPH-001: Create knowledge graph and add entities
func TestKnowledgeGraphAddEntity(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, err := db.CreateKnowledgeGraph("test")
	if err != nil {
		t.Fatalf("Failed to create knowledge graph: %v", err)
	}

	// Add entity
	err = kg.AddEntity(Entity{
		ID:   "person1",
		Type: "person",
		Name: "Alice",
		Properties: map[string]any{
			"age": 30,
		},
	})
	if err != nil {
		t.Fatalf("Failed to add entity: %v", err)
	}

	// Retrieve entity
	entity, err := kg.GetEntity("person1")
	if err != nil {
		t.Fatalf("Failed to get entity: %v", err)
	}

	if entity.Name != "Alice" {
		t.Errorf("Expected name 'Alice', got '%s'", entity.Name)
	}
	if entity.Type != "person" {
		t.Errorf("Expected type 'person', got '%s'", entity.Type)
	}
}

// TC-GRAPH-002: Duplicate entity ID fails
func TestKnowledgeGraphDuplicateEntity(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	_ = kg.AddEntity(Entity{ID: "e1", Type: "test"})

	err := kg.AddEntity(Entity{ID: "e1", Type: "test"})
	if err == nil {
		t.Error("Expected error for duplicate entity ID")
	}
}

// TC-GRAPH-003: Add relationships between entities
func TestKnowledgeGraphAddRelationship(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	_ = kg.AddEntity(Entity{ID: "alice", Type: "person", Name: "Alice"})
	_ = kg.AddEntity(Entity{ID: "bob", Type: "person", Name: "Bob"})

	err := kg.AddRelationship(Relationship{
		ID:       "rel1",
		SourceID: "alice",
		TargetID: "bob",
		Type:     "knows",
		Weight:   0.8,
	})
	if err != nil {
		t.Fatalf("Failed to add relationship: %v", err)
	}

	rels := kg.GetRelationships("alice", "outgoing")
	if len(rels) != 1 {
		t.Fatalf("Expected 1 relationship, got %d", len(rels))
	}

	if rels[0].TargetID != "bob" {
		t.Errorf("Expected target 'bob', got '%s'", rels[0].TargetID)
	}
}

// TC-GRAPH-004: Relationship with missing entity fails
func TestKnowledgeGraphRelationshipMissingEntity(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")
	_ = kg.AddEntity(Entity{ID: "alice", Type: "person"})

	err := kg.AddRelationship(Relationship{
		ID:       "rel1",
		SourceID: "alice",
		TargetID: "nobody", // doesn't exist
		Type:     "knows",
	})

	if err == nil {
		t.Error("Expected error for relationship with missing entity")
	}
}

// TC-GRAPH-005: Graph traversal BFS
func TestKnowledgeGraphTraversalBFS(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	// Create a chain: A -> B -> C -> D
	_ = kg.AddEntity(Entity{ID: "a", Type: "node"})
	_ = kg.AddEntity(Entity{ID: "b", Type: "node"})
	_ = kg.AddEntity(Entity{ID: "c", Type: "node"})
	_ = kg.AddEntity(Entity{ID: "d", Type: "node"})

	_ = kg.AddRelationship(Relationship{ID: "r1", SourceID: "a", TargetID: "b", Type: "link"})
	_ = kg.AddRelationship(Relationship{ID: "r2", SourceID: "b", TargetID: "c", Type: "link"})
	_ = kg.AddRelationship(Relationship{ID: "r3", SourceID: "c", TargetID: "d", Type: "link"})

	// Traverse from A with depth 2
	result, err := kg.Traverse([]string{"a"}, TraversalConfig{
		MaxDepth:  2,
		MaxNodes:  10,
		Direction: "outgoing",
	})
	if err != nil {
		t.Fatalf("Traversal failed: %v", err)
	}

	// Should find a, b, c (not d, as it's at depth 3)
	if len(result.Entities) != 3 {
		t.Errorf("Expected 3 entities, got %d", len(result.Entities))
	}

	// Check depths
	if result.Depths["a"] != 0 {
		t.Errorf("Expected depth 0 for 'a', got %d", result.Depths["a"])
	}
	if result.Depths["b"] != 1 {
		t.Errorf("Expected depth 1 for 'b', got %d", result.Depths["b"])
	}
	if result.Depths["c"] != 2 {
		t.Errorf("Expected depth 2 for 'c', got %d", result.Depths["c"])
	}
}

// TC-CONV-001: Add and retrieve conversation turns
func TestConversationAddTurn(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("conv", WithDimension(3))

	id, err := coll.InsertTurn(ConversationTurn{
		SessionID:  "session1",
		TurnNumber: 1,
		Role:       "user",
		Content:    "Hello",
		Vector:     []float32{1, 0, 0},
	})
	if err != nil {
		t.Fatalf("Failed to add turn: %v", err)
	}

	if id == 0 {
		t.Error("Expected non-zero ID")
	}

	// Add assistant response
	_, err = coll.InsertTurn(ConversationTurn{
		SessionID:  "session1",
		TurnNumber: 2,
		Role:       "assistant",
		Content:    "Hi there!",
		Vector:     []float32{0, 1, 0},
	})
	if err != nil {
		t.Fatalf("Failed to add second turn: %v", err)
	}

	// Get session
	records, err := coll.GetSession("session1")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 turns, got %d", len(records))
	}
}

// TC-CONV-002: List sessions
func TestConversationListSessions(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("conv", WithDimension(3))

	// Add turns to different sessions
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "msg1", Vector: []float32{1, 0, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s2", Content: "msg2", Vector: []float32{0, 1, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s3", Content: "msg3", Vector: []float32{0, 0, 1}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "msg4", Vector: []float32{1, 1, 0}})

	sessions := coll.ListSessions()

	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}
}

// TC-CONV-003: Search within session
func TestConversationSearchInSession(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("conv", WithDimension(3))

	// Add turns to different sessions
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "apple", Vector: []float32{1, 0, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "banana", Vector: []float32{0.9, 0.1, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s2", Content: "cherry", Vector: []float32{0.8, 0.2, 0}})

	// Search in s1 only
	results, err := coll.SearchInSession("s1", []float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should only return results from s1
	if len(results) != 2 {
		t.Errorf("Expected 2 results from session s1, got %d", len(results))
	}

	for _, r := range results {
		sid, _ := r.Record.Payload[PayloadKeySessionID].(string)
		if sid != "s1" {
			t.Errorf("Result from wrong session: %s", sid)
		}
	}
}

// TC-EPISODE-001: Manually create episode
func TestEpisodeCreate(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("memories", WithDimension(3))

	// Insert some records
	var ids []uint64
	for i := 0; i < 5; i++ {
		id, _ := coll.Insert([]float32{float32(i), 0, 0}, nil)
		ids = append(ids, id)
	}

	es, err := db.CreateEpisodeStore("memories")
	if err != nil {
		t.Fatalf("Failed to create episode store: %v", err)
	}

	episode, err := es.CreateEpisode(ids[:3], "Test Episode")
	if err != nil {
		t.Fatalf("Failed to create episode: %v", err)
	}

	if episode.Title != "Test Episode" {
		t.Errorf("Expected title 'Test Episode', got '%s'", episode.Title)
	}
	if len(episode.RecordIDs) != 3 {
		t.Errorf("Expected 3 record IDs, got %d", len(episode.RecordIDs))
	}
}

// TC-EPISODE-002: Get episode
func TestEpisodeGet(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("memories", WithDimension(3))
	id1, _ := coll.Insert([]float32{1, 0, 0}, nil)
	id2, _ := coll.Insert([]float32{0, 1, 0}, nil)

	es, _ := db.CreateEpisodeStore("memories")
	created, _ := es.CreateEpisode([]uint64{id1, id2}, "My Episode")

	retrieved, err := es.GetEpisode(created.ID)
	if err != nil {
		t.Fatalf("Failed to get episode: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Episode ID mismatch")
	}
	if retrieved.Title != "My Episode" {
		t.Errorf("Episode title mismatch")
	}
}

// TC-EPISODE-003: List episodes
func TestEpisodeList(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("memories", WithDimension(3))
	id1, _ := coll.Insert([]float32{1, 0, 0}, nil)
	id2, _ := coll.Insert([]float32{0, 1, 0}, nil)
	id3, _ := coll.Insert([]float32{0, 0, 1}, nil)

	es, _ := db.CreateEpisodeStore("memories")
	_, _ = es.CreateEpisode([]uint64{id1}, "Episode 1")
	_, _ = es.CreateEpisode([]uint64{id2}, "Episode 2")
	_, _ = es.CreateEpisode([]uint64{id3}, "Episode 3")

	episodes := es.ListEpisodes()
	if len(episodes) != 3 {
		t.Errorf("Expected 3 episodes, got %d", len(episodes))
	}
}

// TC-EPISODE-004: Expand episode returns records
func TestEpisodeExpand(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("memories", WithDimension(3))
	id1, _ := coll.InsertWithOptions([]float32{1, 0, 0}, nil, WithContentOption("First"))
	id2, _ := coll.InsertWithOptions([]float32{0, 1, 0}, nil, WithContentOption("Second"))

	es, _ := db.CreateEpisodeStore("memories")
	ep, _ := es.CreateEpisode([]uint64{id1, id2}, "Test")

	records, err := es.ExpandEpisode(ep.ID)
	if err != nil {
		t.Fatalf("Failed to expand episode: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}
}

// TC-CONSOLIDATION-001: Find similar clusters
func TestConsolidationFindClusters(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Create clustered data
	// Cluster 1: near (1, 0, 0)
	_, _ = coll.Insert([]float32{1, 0, 0}, nil)
	_, _ = coll.Insert([]float32{0.99, 0.01, 0}, nil)
	_, _ = coll.Insert([]float32{0.98, 0.02, 0}, nil)

	// Cluster 2: near (0, 1, 0)
	_, _ = coll.Insert([]float32{0, 1, 0}, nil)
	_, _ = coll.Insert([]float32{0.01, 0.99, 0}, nil)
	_, _ = coll.Insert([]float32{0.02, 0.98, 0}, nil)

	clusters, err := coll.FindSimilarClusters(ConsolidationConfig{
		SimilarityThreshold: 0.95,
		MinGroupSize:        2,
		MaxGroupSize:        10,
	})
	if err != nil {
		t.Fatalf("Failed to find clusters: %v", err)
	}

	if len(clusters) < 2 {
		t.Errorf("Expected at least 2 clusters, got %d", len(clusters))
	}
}

// TC-CONSOLIDATION-002: Archive and unarchive records
func TestConsolidationArchive(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))
	id, _ := coll.Insert([]float32{1, 0, 0}, nil)

	// Archive
	err := coll.ArchiveRecord(id)
	if err != nil {
		t.Fatalf("Failed to archive: %v", err)
	}

	// Check archived
	archived, _ := coll.GetArchived()
	if len(archived) != 1 {
		t.Errorf("Expected 1 archived, got %d", len(archived))
	}

	// Unarchive
	err = coll.UnarchiveRecord(id)
	if err != nil {
		t.Fatalf("Failed to unarchive: %v", err)
	}

	archived, _ = coll.GetArchived()
	if len(archived) != 0 {
		t.Errorf("Expected 0 archived after unarchive, got %d", len(archived))
	}
}

// TC-CONSOLIDATION-003: Archive non-existent record fails
func TestConsolidationArchiveNotFound(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	err := coll.ArchiveRecord(999)
	if err == nil {
		t.Error("Expected error for archiving non-existent record")
	}
}

// TC-CRUD-001: Get record by ID
func TestCollectionGet(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))
	id, _ := coll.Insert([]float32{1, 0, 0}, map[string]any{"name": "test"})

	record, err := coll.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if record.ID != id {
		t.Errorf("Expected ID %d, got %d", id, record.ID)
	}
	if record.Payload["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", record.Payload["name"])
	}
}

// TC-CRUD-002: Get non-existent record fails
func TestCollectionGetNotFound(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	_, err := coll.Get(999)
	if err == nil {
		t.Error("Expected error for non-existent record")
	}
}

// TC-CRUD-003: Update record payload
func TestCollectionUpdatePayload(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))
	id, _ := coll.Insert([]float32{1, 0, 0}, map[string]any{"name": "old"})

	err := coll.Update(id, map[string]any{"name": "new", "extra": "field"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	record, _ := coll.Get(id)
	if record.Payload["name"] != "new" {
		t.Errorf("Expected name 'new', got %v", record.Payload["name"])
	}
	if record.Payload["extra"] != "field" {
		t.Errorf("Expected extra 'field', got %v", record.Payload["extra"])
	}
}

// TC-CRUD-004: Update non-existent record fails
func TestCollectionUpdateNotFound(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	err := coll.Update(999, map[string]any{"name": "test"})
	if err == nil {
		t.Error("Expected error for non-existent record")
	}
}

// TC-CRUD-005: Delete record by ID
func TestCollectionDeleteByID(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))
	id, _ := coll.Insert([]float32{1, 0, 0}, nil)

	err := coll.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = coll.Get(id)
	if err == nil {
		t.Error("Expected error after delete")
	}
}

// TC-CRUD-006: Delete non-existent record fails
func TestCollectionDeleteNotFound(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	err := coll.Delete(999)
	if err == nil {
		t.Error("Expected error for non-existent record")
	}
}

// TC-GRAPH-006: Delete entity removes relationships
func TestKnowledgeGraphDeleteEntityCleansRelationships(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	_ = kg.AddEntity(Entity{ID: "alice", Type: "person"})
	_ = kg.AddEntity(Entity{ID: "bob", Type: "person"})
	_ = kg.AddRelationship(Relationship{
		ID: "rel1", SourceID: "alice", TargetID: "bob", Type: "knows",
	})

	// Delete bob - relationship should be cleaned up
	err := kg.DeleteEntity("bob")
	if err != nil {
		t.Fatalf("DeleteEntity failed: %v", err)
	}

	// Alice's relationships should be empty
	rels := kg.GetRelationships("alice", "outgoing")
	if len(rels) != 0 {
		t.Errorf("Expected 0 relationships after deleting target, got %d", len(rels))
	}
}

// TC-GRAPH-007: Delete relationship
func TestKnowledgeGraphDeleteRelationship(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	_ = kg.AddEntity(Entity{ID: "alice", Type: "person"})
	_ = kg.AddEntity(Entity{ID: "bob", Type: "person"})
	_ = kg.AddRelationship(Relationship{
		ID: "rel1", SourceID: "alice", TargetID: "bob", Type: "knows",
	})

	err := kg.DeleteRelationship("rel1")
	if err != nil {
		t.Fatalf("DeleteRelationship failed: %v", err)
	}

	rels := kg.GetRelationships("alice", "outgoing")
	if len(rels) != 0 {
		t.Errorf("Expected 0 relationships after delete, got %d", len(rels))
	}
}

// TC-GRAPH-008: Delete non-existent relationship fails
func TestKnowledgeGraphDeleteRelationshipNotFound(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	err := kg.DeleteRelationship("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent relationship")
	}
}

// TC-GRAPH-009: List entities by type
func TestKnowledgeGraphListEntitiesByType(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	_ = kg.AddEntity(Entity{ID: "p1", Type: "person", Name: "Alice"})
	_ = kg.AddEntity(Entity{ID: "p2", Type: "person", Name: "Bob"})
	_ = kg.AddEntity(Entity{ID: "c1", Type: "company", Name: "Acme"})

	persons := kg.ListEntities("person")
	if len(persons) != 2 {
		t.Errorf("Expected 2 persons, got %d", len(persons))
	}

	companies := kg.ListEntities("company")
	if len(companies) != 1 {
		t.Errorf("Expected 1 company, got %d", len(companies))
	}

	all := kg.ListEntities("")
	if len(all) != 3 {
		t.Errorf("Expected 3 total entities, got %d", len(all))
	}
}

// TC-CLEANUP-001: Count expired records
func TestCountExpiredMCP(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert record with expired TTL (use WithExpiresAt with past time)
	_, _ = coll.InsertWithOptions(
		[]float32{1, 0, 0},
		nil,
		WithExpiresAt(time.Now().Add(-1*time.Hour)), // Already expired
	)

	// Insert record without TTL
	_, _ = coll.Insert([]float32{0, 1, 0}, nil)

	count := coll.CountExpired()
	if count != 1 {
		t.Errorf("Expected 1 expired record, got %d", count)
	}
}

// TC-CLEANUP-002: Cleanup expired records
func TestCleanupExpiredMCP(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert record with expired TTL (use WithExpiresAt with past time)
	_, _ = coll.InsertWithOptions(
		[]float32{1, 0, 0},
		nil,
		WithExpiresAt(time.Now().Add(-1*time.Hour)), // Already expired
	)

	// Insert record without TTL
	id2, _ := coll.Insert([]float32{0, 1, 0}, nil)

	deleted, err := coll.CleanupExpired()
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleted)
	}

	// Non-expired record should still exist
	_, err = coll.Get(id2)
	if err != nil {
		t.Error("Non-expired record should still exist")
	}
}

// TC-MEMORY-001: Enforce memory limit
func TestEnforceMemoryLimitMCP(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert 10 records
	for i := 0; i < 10; i++ {
		_, _ = coll.Insert([]float32{float32(i), 0, 0}, nil)
	}

	// Enforce limit of 5 - need EvictionBatchSize to be set appropriately
	evicted := coll.EnforceMemoryLimit(MemoryConfig{
		MaxRecords:        5,
		EvictionPolicy:    "fifo",
		EvictionBatchSize: 10, // Must be large enough to evict all at once
	})

	if evicted != 5 {
		t.Errorf("Expected 5 evicted, got %d", evicted)
	}

	if coll.Count() != 5 {
		t.Errorf("Expected 5 remaining, got %d", coll.Count())
	}
}

// TC-CONSOLIDATION-004: Expand consolidation record
func TestExpandConsolidationRecord(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("test", WithDimension(3))

	// Insert some records
	id1, _ := coll.Insert([]float32{1, 0, 0}, nil)
	id2, _ := coll.Insert([]float32{0.9, 0.1, 0}, nil)

	// Create a consolidation record manually
	_, _ = coll.Insert([]float32{0.95, 0.05, 0}, map[string]any{
		PayloadKeyIsConsolidation:   true,
		PayloadKeyConsolidatedFrom:  []uint64{id1, id2},
		PayloadKeyConsolidationGroup: "group1",
	})

	// Find the consolidation record
	consolidations, _ := coll.GetConsolidations()
	if len(consolidations) != 1 {
		t.Fatalf("Expected 1 consolidation, got %d", len(consolidations))
	}

	// Expand it
	records, err := coll.ExpandConsolidation(consolidations[0].ID)
	if err != nil {
		t.Fatalf("ExpandConsolidation failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 original records, got %d", len(records))
	}
}

// TC-CONV-004: Delete session
func TestConversationDeleteSession(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("conv", WithDimension(3))

	// Add turns to two sessions
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "msg1", Vector: []float32{1, 0, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "msg2", Vector: []float32{0.9, 0.1, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s2", Content: "msg3", Vector: []float32{0, 1, 0}})

	// Get session s1 records and delete them
	records, _ := coll.GetSession("s1")
	for _, r := range records {
		_ = coll.Delete(r.ID)
	}

	// Verify s1 is deleted
	records, _ = coll.GetSession("s1")
	if len(records) != 0 {
		t.Errorf("Expected 0 records in deleted session, got %d", len(records))
	}

	// Verify s2 still exists
	records, _ = coll.GetSession("s2")
	if len(records) != 1 {
		t.Errorf("Expected 1 record in s2, got %d", len(records))
	}
}

// TC-CONV-005: Get session stats
func TestConversationSessionStats(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("conv", WithDimension(3))

	// Add turns with different roles
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Role: "user", Content: "Hello", Vector: []float32{1, 0, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Role: "assistant", Content: "Hi", Vector: []float32{0.9, 0.1, 0}})
	_, _ = coll.InsertTurn(ConversationTurn{SessionID: "s1", Role: "user", Content: "How are you?", Vector: []float32{0.8, 0.2, 0}})

	stats, err := coll.GetSessionStats("s1")
	if err != nil {
		t.Fatalf("GetSessionStats failed: %v", err)
	}

	if stats.TurnCount != 3 {
		t.Errorf("Expected 3 turns, got %d", stats.TurnCount)
	}
	if stats.Roles["user"] != 2 {
		t.Errorf("Expected 2 user turns, got %d", stats.Roles["user"])
	}
	if stats.Roles["assistant"] != 1 {
		t.Errorf("Expected 1 assistant turn, got %d", stats.Roles["assistant"])
	}
}

// TC-COLLECTION-001: Create collection with options
func TestCreateCollectionWithOptions(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, err := db.CreateCollection("test",
		WithDimension(128),
		WithDistanceType(DistanceDot),
		WithHNSW(16, 200),
	)
	if err != nil {
		t.Fatalf("CreateCollection failed: %v", err)
	}

	if coll.Dimension() != 128 {
		t.Errorf("Expected dimension 128, got %d", coll.Dimension())
	}
	if coll.DistanceType() != DistanceDot {
		t.Errorf("Expected distance type 'dot', got %v", coll.DistanceType())
	}
}

// TC-COLLECTION-002: Drop collection
func TestDropCollectionMCP(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	_, _ = db.CreateCollection("test", WithDimension(3))

	err := db.DropCollection("test")
	if err != nil {
		t.Fatalf("DropCollection failed: %v", err)
	}

	_, err = db.GetCollection("test")
	if err == nil {
		t.Error("Expected error for dropped collection")
	}
}

// TC-COLLECTION-003: Drop non-existent collection fails
func TestDropCollectionNotFound(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	err := db.DropCollection("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent collection")
	}
}

// TC-DB-001: Sync database
func TestDatabaseSync(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	_, _ = coll.Insert([]float32{1, 0, 0}, nil)

	err := db.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
}

// TC-DB-002: Get metrics
func TestDatabaseMetrics(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll := db.Collection("test")
	_, _ = coll.Insert([]float32{1, 0, 0}, nil)
	_, _ = coll.Insert([]float32{0, 1, 0}, nil)

	metrics := db.Metrics()
	if metrics.InsertCount < 2 {
		t.Errorf("Expected at least 2 inserts, got %d", metrics.InsertCount)
	}
}

// TC-INTEGRATION-001: Full agent memory workflow
func TestAgentMemoryWorkflow(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	coll, _ := db.CreateCollection("memories",
		WithDimension(3),
		WithTextIndex("content"),
	)

	// Remember (insert with importance and TTL)
	id1, _ := coll.InsertWithOptions(
		[]float32{1, 0, 0},
		map[string]any{"topic": "coding"},
		WithContentOption("User prefers Go"),
		WithImportance(0.9),
		WithTTL(24*time.Hour),
	)

	_, _ = coll.InsertWithOptions(
		[]float32{0.9, 0.1, 0},
		map[string]any{"topic": "coding"},
		WithContentOption("User uses VSCode"),
		WithImportance(0.5),
	)

	_, _ = coll.InsertWithOptions(
		[]float32{0, 1, 0},
		map[string]any{"topic": "music"},
		WithContentOption("User likes jazz"),
		WithImportance(0.3),
	)

	// Recall (search with filters)
	results, err := coll.Search(
		[]float32{1, 0, 0},
		TopK(10),
		WithFilter(ImportanceAbove(0.4)),
	)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should return id1 and id2 (importance >= 0.4), not id3
	if len(results) != 2 {
		t.Errorf("Expected 2 results with importance >= 0.4, got %d", len(results))
	}

	// Test text search
	textResults, err := coll.TextSearch("Go", TopK(10))
	if err != nil {
		t.Fatalf("Text search failed: %v", err)
	}

	if len(textResults) == 0 {
		t.Error("Expected at least 1 text search result for 'Go'")
	}

	// Check has TTL
	record, _ := coll.Get(id1)
	if !record.HasTTL() {
		t.Error("Record should have TTL")
	}
}
