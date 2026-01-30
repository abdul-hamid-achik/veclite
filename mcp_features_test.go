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

	kg.AddEntity(Entity{ID: "e1", Type: "test"})

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

	kg.AddEntity(Entity{ID: "alice", Type: "person", Name: "Alice"})
	kg.AddEntity(Entity{ID: "bob", Type: "person", Name: "Bob"})

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
	kg.AddEntity(Entity{ID: "alice", Type: "person"})

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
	kg.AddEntity(Entity{ID: "a", Type: "node"})
	kg.AddEntity(Entity{ID: "b", Type: "node"})
	kg.AddEntity(Entity{ID: "c", Type: "node"})
	kg.AddEntity(Entity{ID: "d", Type: "node"})

	kg.AddRelationship(Relationship{ID: "r1", SourceID: "a", TargetID: "b", Type: "link"})
	kg.AddRelationship(Relationship{ID: "r2", SourceID: "b", TargetID: "c", Type: "link"})
	kg.AddRelationship(Relationship{ID: "r3", SourceID: "c", TargetID: "d", Type: "link"})

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
	coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "msg1", Vector: []float32{1, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "s2", Content: "msg2", Vector: []float32{0, 1, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "s3", Content: "msg3", Vector: []float32{0, 0, 1}})
	coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "msg4", Vector: []float32{1, 1, 0}})

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
	coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "apple", Vector: []float32{1, 0, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "s1", Content: "banana", Vector: []float32{0.9, 0.1, 0}})
	coll.InsertTurn(ConversationTurn{SessionID: "s2", Content: "cherry", Vector: []float32{0.8, 0.2, 0}})

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
	es.CreateEpisode([]uint64{id1}, "Episode 1")
	es.CreateEpisode([]uint64{id2}, "Episode 2")
	es.CreateEpisode([]uint64{id3}, "Episode 3")

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
	coll.Insert([]float32{1, 0, 0}, nil)
	coll.Insert([]float32{0.99, 0.01, 0}, nil)
	coll.Insert([]float32{0.98, 0.02, 0}, nil)

	// Cluster 2: near (0, 1, 0)
	coll.Insert([]float32{0, 1, 0}, nil)
	coll.Insert([]float32{0.01, 0.99, 0}, nil)
	coll.Insert([]float32{0.02, 0.98, 0}, nil)

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
