package veclite

import (
	"testing"
)

func TestKnowledgeGraphEntity(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	kg, err := db.CreateKnowledgeGraph("test")
	if err != nil {
		t.Fatalf("CreateKnowledgeGraph failed: %v", err)
	}

	t.Run("add entity", func(t *testing.T) {
		err := kg.AddEntity(Entity{
			ID:     "person-1",
			Type:   "person",
			Name:   "Alice",
			Vector: []float32{1, 0, 0, 0},
			Properties: map[string]any{
				"age": 30,
			},
		})
		if err != nil {
			t.Fatalf("AddEntity failed: %v", err)
		}
	})

	t.Run("add duplicate entity", func(t *testing.T) {
		err := kg.AddEntity(Entity{
			ID:   "person-1",
			Type: "person",
			Name: "Alice Duplicate",
		})
		if err == nil {
			t.Error("expected error for duplicate entity")
		}
	})

	t.Run("get entity", func(t *testing.T) {
		entity, err := kg.GetEntity("person-1")
		if err != nil {
			t.Fatalf("GetEntity failed: %v", err)
		}

		if entity.Name != "Alice" {
			t.Errorf("expected name 'Alice', got %q", entity.Name)
		}
		if entity.Properties["age"] != 30 {
			t.Errorf("expected age 30, got %v", entity.Properties["age"])
		}
	})

	t.Run("get non-existent entity", func(t *testing.T) {
		_, err := kg.GetEntity("non-existent")
		if err == nil {
			t.Error("expected error for non-existent entity")
		}
	})

	t.Run("update entity", func(t *testing.T) {
		err := kg.UpdateEntity(Entity{
			ID:     "person-1",
			Type:   "person",
			Name:   "Alice Updated",
			Vector: []float32{0.9, 0.1, 0, 0},
		})
		if err != nil {
			t.Fatalf("UpdateEntity failed: %v", err)
		}

		entity, _ := kg.GetEntity("person-1")
		if entity.Name != "Alice Updated" {
			t.Errorf("expected updated name, got %q", entity.Name)
		}
	})

	t.Run("list entities", func(t *testing.T) {
		_ = kg.AddEntity(Entity{ID: "company-1", Type: "company", Name: "Acme Corp"})
		_ = kg.AddEntity(Entity{ID: "person-2", Type: "person", Name: "Bob"})

		all := kg.ListEntities("")
		if len(all) != 3 {
			t.Errorf("expected 3 entities, got %d", len(all))
		}

		persons := kg.ListEntities("person")
		if len(persons) != 2 {
			t.Errorf("expected 2 persons, got %d", len(persons))
		}
	})

	t.Run("delete entity", func(t *testing.T) {
		err := kg.DeleteEntity("person-2")
		if err != nil {
			t.Fatalf("DeleteEntity failed: %v", err)
		}

		entities := kg.ListEntities("")
		if len(entities) != 2 {
			t.Errorf("expected 2 entities after delete, got %d", len(entities))
		}
	})
}

func TestKnowledgeGraphRelationship(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	// Add entities
	_ = kg.AddEntity(Entity{ID: "alice", Type: "person", Name: "Alice"})
	_ = kg.AddEntity(Entity{ID: "bob", Type: "person", Name: "Bob"})
	_ = kg.AddEntity(Entity{ID: "acme", Type: "company", Name: "Acme Corp"})

	t.Run("add relationship", func(t *testing.T) {
		err := kg.AddRelationship(Relationship{
			ID:       "rel-1",
			SourceID: "alice",
			TargetID: "acme",
			Type:     "works_at",
			Weight:   0.9,
		})
		if err != nil {
			t.Fatalf("AddRelationship failed: %v", err)
		}
	})

	t.Run("add relationship with invalid entity", func(t *testing.T) {
		err := kg.AddRelationship(Relationship{
			ID:       "rel-invalid",
			SourceID: "alice",
			TargetID: "non-existent",
			Type:     "knows",
		})
		if err == nil {
			t.Error("expected error for invalid entity")
		}
	})

	t.Run("add bidirectional relationship", func(t *testing.T) {
		err := kg.AddRelationship(Relationship{
			ID:            "rel-2",
			SourceID:      "alice",
			TargetID:      "bob",
			Type:          "knows",
			Weight:        0.8,
			Bidirectional: true,
		})
		if err != nil {
			t.Fatalf("AddRelationship failed: %v", err)
		}

		// Check both directions
		aliceRels := kg.GetRelationships("alice", "outgoing")
		bobRels := kg.GetRelationships("bob", "outgoing")

		foundAliceOut := false
		foundBobOut := false
		for _, r := range aliceRels {
			if r.ID == "rel-2" {
				foundAliceOut = true
			}
		}
		for _, r := range bobRels {
			if r.ID == "rel-2" {
				foundBobOut = true
			}
		}

		if !foundAliceOut || !foundBobOut {
			t.Error("bidirectional relationship should appear in both directions")
		}
	})

	t.Run("get relationship", func(t *testing.T) {
		rel, err := kg.GetRelationship("rel-1")
		if err != nil {
			t.Fatalf("GetRelationship failed: %v", err)
		}

		if rel.Type != "works_at" {
			t.Errorf("expected type 'works_at', got %q", rel.Type)
		}
	})

	t.Run("get relationships by direction", func(t *testing.T) {
		outgoing := kg.GetRelationships("alice", "outgoing")
		if len(outgoing) < 2 {
			t.Errorf("expected at least 2 outgoing relationships, got %d", len(outgoing))
		}

		incoming := kg.GetRelationships("acme", "incoming")
		if len(incoming) < 1 {
			t.Errorf("expected at least 1 incoming relationship, got %d", len(incoming))
		}
	})

	t.Run("delete relationship", func(t *testing.T) {
		err := kg.DeleteRelationship("rel-1")
		if err != nil {
			t.Fatalf("DeleteRelationship failed: %v", err)
		}

		_, err = kg.GetRelationship("rel-1")
		if err == nil {
			t.Error("expected error for deleted relationship")
		}
	})
}

func TestKnowledgeGraphTraversal(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	// Create a graph:
	// alice -> bob -> charlie
	//       -> acme <- bob
	_ = kg.AddEntity(Entity{ID: "alice", Type: "person", Name: "Alice"})
	_ = kg.AddEntity(Entity{ID: "bob", Type: "person", Name: "Bob"})
	_ = kg.AddEntity(Entity{ID: "charlie", Type: "person", Name: "Charlie"})
	_ = kg.AddEntity(Entity{ID: "acme", Type: "company", Name: "Acme"})

	_ = kg.AddRelationship(Relationship{ID: "r1", SourceID: "alice", TargetID: "bob", Type: "knows", Weight: 0.9})
	_ = kg.AddRelationship(Relationship{ID: "r2", SourceID: "bob", TargetID: "charlie", Type: "knows", Weight: 0.8})
	_ = kg.AddRelationship(Relationship{ID: "r3", SourceID: "alice", TargetID: "acme", Type: "works_at", Weight: 0.95})
	_ = kg.AddRelationship(Relationship{ID: "r4", SourceID: "bob", TargetID: "acme", Type: "works_at", Weight: 0.9})

	t.Run("basic traversal", func(t *testing.T) {
		result, err := kg.Traverse([]string{"alice"}, TraversalConfig{
			MaxDepth: 2,
		})
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		if len(result.Entities) < 3 {
			t.Errorf("expected at least 3 entities, got %d", len(result.Entities))
		}

		// Check depths
		if result.Depths["alice"] != 0 {
			t.Error("alice should be at depth 0")
		}
		if result.Depths["bob"] != 1 {
			t.Error("bob should be at depth 1")
		}
	})

	t.Run("traversal with depth limit", func(t *testing.T) {
		result, err := kg.Traverse([]string{"alice"}, TraversalConfig{
			MaxDepth: 1,
		})
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		// Should not reach charlie (depth 2)
		found := false
		for _, e := range result.Entities {
			if e.ID == "charlie" {
				found = true
			}
		}
		if found {
			t.Error("should not reach charlie with depth 1")
		}
	})

	t.Run("traversal with relationship type filter", func(t *testing.T) {
		result, err := kg.Traverse([]string{"alice"}, TraversalConfig{
			MaxDepth:          2,
			RelationshipTypes: []string{"knows"},
		})
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		// Should not find acme (only reachable via works_at)
		foundAcme := false
		for _, e := range result.Entities {
			if e.ID == "acme" {
				foundAcme = true
			}
		}
		if foundAcme {
			t.Error("should not find acme when filtering for 'knows' relationships only")
		}
	})

	t.Run("traversal with min weight", func(t *testing.T) {
		result, err := kg.Traverse([]string{"alice"}, TraversalConfig{
			MaxDepth:  2,
			MinWeight: 0.85,
		})
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		// charlie should not be reachable (weight of r2 is 0.8)
		foundCharlie := false
		for _, e := range result.Entities {
			if e.ID == "charlie" {
				foundCharlie = true
			}
		}
		if foundCharlie {
			t.Error("should not reach charlie when min weight is 0.85")
		}
	})

	t.Run("traversal outgoing only", func(t *testing.T) {
		result, err := kg.Traverse([]string{"acme"}, TraversalConfig{
			MaxDepth:  2,
			Direction: "outgoing",
		})
		if err != nil {
			t.Fatalf("Traverse failed: %v", err)
		}

		// Should only find acme (no outgoing edges)
		if len(result.Entities) != 1 {
			t.Errorf("expected 1 entity (acme only), got %d", len(result.Entities))
		}
	})
}

func TestKnowledgeGraphSearchWithExpansion(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	// Add entities with vectors
	_ = kg.AddEntity(Entity{ID: "alice", Type: "person", Name: "Alice", Vector: []float32{1, 0, 0, 0}})
	_ = kg.AddEntity(Entity{ID: "bob", Type: "person", Name: "Bob", Vector: []float32{0.9, 0.1, 0, 0}})
	_ = kg.AddEntity(Entity{ID: "acme", Type: "company", Name: "Acme", Vector: []float32{0, 1, 0, 0}})

	_ = kg.AddRelationship(Relationship{ID: "r1", SourceID: "alice", TargetID: "bob", Type: "knows", Weight: 0.9})
	_ = kg.AddRelationship(Relationship{ID: "r2", SourceID: "alice", TargetID: "acme", Type: "works_at", Weight: 0.8})

	t.Run("search with expansion", func(t *testing.T) {
		results, err := kg.SearchWithExpansion(
			[]float32{0.95, 0.05, 0, 0},
			TraversalConfig{MaxDepth: 1},
			WithLimit(5),
		)
		if err != nil {
			t.Fatalf("SearchWithExpansion failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("expected at least one result")
		}

		// First result should be Alice (closest vector)
		if results[0].Entity.ID != "alice" {
			t.Errorf("expected first result to be alice, got %s", results[0].Entity.ID)
		}

		// Should have related entities
		if len(results[0].RelatedEntities) != 2 {
			t.Errorf("expected 2 related entities, got %d", len(results[0].RelatedEntities))
		}

		// Should have relationships
		if len(results[0].Relationships) != 2 {
			t.Errorf("expected 2 relationships, got %d", len(results[0].Relationships))
		}
	})
}

func TestKnowledgeGraphStats(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")

	_ = kg.AddEntity(Entity{ID: "p1", Type: "person", Name: "Person 1"})
	_ = kg.AddEntity(Entity{ID: "p2", Type: "person", Name: "Person 2"})
	_ = kg.AddEntity(Entity{ID: "c1", Type: "company", Name: "Company 1"})

	_ = kg.AddRelationship(Relationship{ID: "r1", SourceID: "p1", TargetID: "p2", Type: "knows"})
	_ = kg.AddRelationship(Relationship{ID: "r2", SourceID: "p1", TargetID: "c1", Type: "works_at"})

	stats := kg.Stats()

	if stats.EntityCount != 3 {
		t.Errorf("expected 3 entities, got %d", stats.EntityCount)
	}
	if stats.RelationshipCount != 2 {
		t.Errorf("expected 2 relationships, got %d", stats.RelationshipCount)
	}
	if stats.EntityTypes["person"] != 2 {
		t.Errorf("expected 2 persons, got %d", stats.EntityTypes["person"])
	}
	if stats.RelationshipTypes["knows"] != 1 {
		t.Errorf("expected 1 'knows' relationship, got %d", stats.RelationshipTypes["knows"])
	}
}

func TestEntityClone(t *testing.T) {
	original := &Entity{
		ID:     "test",
		Type:   "person",
		Name:   "Test",
		Vector: []float32{1, 2, 3},
		Properties: map[string]any{
			"key": "value",
		},
	}

	clone := original.Clone()

	// Modify clone
	clone.Name = "Modified"
	clone.Vector[0] = 99

	// Original should be unchanged
	if original.Name != "Test" {
		t.Error("original name was modified")
	}
	if original.Vector[0] != 1 {
		t.Error("original vector was modified")
	}
}

func TestRelationshipClone(t *testing.T) {
	original := &Relationship{
		ID:            "test",
		SourceID:      "a",
		TargetID:      "b",
		Type:          "knows",
		Weight:        0.5,
		Bidirectional: true,
		Properties: map[string]any{
			"key": "value",
		},
	}

	clone := original.Clone()

	// Modify clone
	clone.Weight = 0.9

	// Original should be unchanged
	if original.Weight != 0.5 {
		t.Error("original weight was modified")
	}
}

func TestCreateKnowledgeGraphNoName(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	_, err = db.CreateKnowledgeGraph("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestAddEntityNoID(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")
	err = kg.AddEntity(Entity{Name: "No ID"})
	if err == nil {
		t.Error("expected error for entity without ID")
	}
}

func TestAddRelationshipNoID(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	kg, _ := db.CreateKnowledgeGraph("test")
	_ = kg.AddEntity(Entity{ID: "a"})
	_ = kg.AddEntity(Entity{ID: "b"})

	err = kg.AddRelationship(Relationship{SourceID: "a", TargetID: "b"})
	if err == nil {
		t.Error("expected error for relationship without ID")
	}
}
