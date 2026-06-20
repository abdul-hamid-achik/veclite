package storage

import "testing"

// TestMigrateUpgradesEveryOlderVersion verifies Migrate brings v1, v2, and v3
// snapshots to the current version additively, initialising the structures each
// version introduced without rewriting record data.
func TestMigrateUpgradesEveryOlderVersion(t *testing.T) {
	for _, from := range []uint32{1, 2, 3} {
		s := &DatabaseSnapshot{
			Version: from,
			Collections: map[string]*CollectionSnapshot{
				"c": {
					Name:      "c",
					Dimension: 3,
					Records:   []*RecordSnapshot{{ID: 1, Vector: []float32{1, 2, 3}}},
				},
			},
		}

		got := Migrate(s)
		if got.Version != CurrentVersion {
			t.Fatalf("from v%d: version = %d, want %d", from, got.Version, CurrentVersion)
		}
		coll := got.Collections["c"]
		// VectorSpaces (the v4 addition) is initialised for every pre-v4 snapshot.
		if coll.VectorSpaces == nil {
			t.Errorf("from v%d: VectorSpaces should be initialised", from)
		}
		// The legacy single vector is preserved untouched (it is the default space).
		if v := coll.Records[0].Vector; len(v) != 3 || v[0] != 1 {
			t.Errorf("from v%d: record vector altered: %v", from, v)
		}
		// Each version only fills in what versions newer than it introduced, mirroring
		// the on-disk migrateSnapshot. A full v1 upgrade fills every earlier structure.
		if from < 2 {
			if got.KnowledgeGraphs == nil || got.EpisodeStores == nil {
				t.Errorf("from v%d: graph/episode maps should be initialised", from)
			}
		}
		if from < 3 {
			if got.Metadata == nil || coll.Metadata == nil {
				t.Errorf("from v%d: metadata maps should be initialised", from)
			}
		}
	}
}

// TestMigrateIsIdempotentAtCurrentVersion verifies a current-version snapshot is
// returned unchanged.
func TestMigrateIsIdempotentAtCurrentVersion(t *testing.T) {
	s := NewDatabaseSnapshot()
	v := s.Version
	got := Migrate(s)
	if got.Version != v {
		t.Errorf("version changed on a current snapshot: %d -> %d", v, got.Version)
	}
	if Migrate(got).Version != v {
		t.Errorf("Migrate should be idempotent")
	}
}

// TestMigrateNilSafe verifies Migrate tolerates nil.
func TestMigrateNilSafe(t *testing.T) {
	if Migrate(nil) != nil {
		t.Error("Migrate(nil) should return nil")
	}
}
