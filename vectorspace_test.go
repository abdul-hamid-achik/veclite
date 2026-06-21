package veclite

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

func TestEmbeddingProfileCompatible(t *testing.T) {
	base := EmbeddingProfile{Provider: "openai", Model: "text-embedding-3-small", Dimension: 1536, Distance: DistanceCosine, Normalize: true}

	if err := base.Compatible(base); err != nil {
		t.Fatalf("identical profiles should be compatible: %v", err)
	}

	// Partial profiles only compare fields that both set.
	if err := base.Compatible(EmbeddingProfile{Dimension: 1536, Normalize: true}); err != nil {
		t.Errorf("partial profile should be compatible: %v", err)
	}

	cases := []struct {
		name  string
		other EmbeddingProfile
	}{
		{"provider", EmbeddingProfile{Provider: "ollama", Normalize: true}},
		{"model", EmbeddingProfile{Model: "text-embedding-3-large", Normalize: true}},
		{"dimension", EmbeddingProfile{Dimension: 768, Normalize: true}},
		{"distance", EmbeddingProfile{Distance: DistanceEuclidean, Normalize: true}},
		{"normalize", EmbeddingProfile{Normalize: false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := base.Compatible(tc.other)
			if err == nil {
				t.Fatalf("expected mismatch on %s", tc.name)
			}
			if !errors.Is(err, ErrProfileMismatch) {
				t.Fatalf("expected ErrProfileMismatch, got %v", err)
			}
		})
	}
}

func TestDefaultVectorSpaceAlwaysExists(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	coll := db.Collection("c")
	if !coll.HasVectorSpace(DefaultVectorSpace) {
		t.Error("default space should always exist")
	}
	if !coll.HasVectorSpace("") {
		t.Error("empty name should resolve to default space")
	}
	spaces := coll.VectorSpaces()
	if len(spaces) != 1 || spaces[0].Name != DefaultVectorSpace {
		t.Fatalf("expected exactly the default space, got %+v", spaces)
	}
}

func TestAddVectorSpaceValidation(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll := db.Collection("c")

	if err := coll.AddVectorSpace(VectorSpaceConfig{Name: DefaultVectorSpace, Dimension: 3}); !errors.Is(err, ErrInvalidVectorSpace) {
		t.Errorf("redeclaring default should fail with ErrInvalidVectorSpace, got %v", err)
	}
	if err := coll.AddVectorSpace(VectorSpaceConfig{Name: "", Dimension: 3}); !errors.Is(err, ErrInvalidVectorSpace) {
		t.Errorf("empty name should fail, got %v", err)
	}
	if err := coll.AddVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 3}); err != nil {
		t.Fatalf("valid space should be added: %v", err)
	}
	if err := coll.AddVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 3}); !errors.Is(err, ErrVectorSpaceExists) {
		t.Errorf("duplicate space should fail with ErrVectorSpaceExists, got %v", err)
	}
}

func TestInsertRecordAndSearchSpaces(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	coll, err := db.CreateCollection("multi",
		WithDimension(3),
		WithVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 2, Distance: DistanceCosine, Modality: "image", HNSW: &HNSWConfig{M: 8, EfConstruction: 64, EfSearch: 32, UseHeuristic: true}}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// One logical record carrying both a default (text) vector and an image vector.
	id, err := coll.InsertRecord(RecordInput{
		Content: "a red apple",
		Payload: map[string]any{"label": "apple"},
		Vectors: map[string][]float32{
			DefaultVectorSpace: {1, 0, 0},
			"image":            {1, 0},
		},
	})
	if err != nil {
		t.Fatalf("InsertRecord: %v", err)
	}
	if _, err := coll.InsertRecord(RecordInput{
		Content: "a blue car",
		Payload: map[string]any{"label": "car"},
		Vectors: map[string][]float32{
			DefaultVectorSpace: {0, 1, 0},
			"image":            {0, 1},
		},
	}); err != nil {
		t.Fatalf("InsertRecord 2: %v", err)
	}

	rec, err := coll.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := rec.VectorFor("image"); !ok || len(got) != 2 {
		t.Fatalf("record should carry an image vector, got %v ok=%v", got, ok)
	}

	// Default-space search.
	res, err := coll.SearchSpace(DefaultVectorSpace, []float32{1, 0, 0}, TopK(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Record.Payload["label"] != "apple" {
		t.Fatalf("default-space search wrong result: %+v", res)
	}

	// Image-space search (HNSW-backed) should rank the car first for an image query near it.
	res, err = coll.SearchSpace("image", []float32{0, 1}, TopK(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Record.Payload["label"] != "car" {
		t.Fatalf("image-space search wrong result: %+v", res)
	}

	// Unknown space.
	if _, err := coll.SearchSpace("audio", []float32{1, 0}); !errors.Is(err, ErrVectorSpaceNotFound) {
		t.Errorf("expected ErrVectorSpaceNotFound, got %v", err)
	}

	// Dimension validation per space.
	if _, err := coll.InsertRecord(RecordInput{Vectors: map[string][]float32{"image": {1, 2, 3}}}); !errors.Is(err, ErrDimensionMismatch) {
		t.Errorf("wrong image dimension should fail, got %v", err)
	}
	// Unknown space on insert.
	if _, err := coll.InsertRecord(RecordInput{Vectors: map[string][]float32{"audio": {1, 2}}}); !errors.Is(err, ErrVectorSpaceNotFound) {
		t.Errorf("insert into unknown space should fail, got %v", err)
	}
}

func TestMultiSpaceSearchFusion(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll, _ := db.CreateCollection("m", WithDimension(2),
		WithVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 2}))

	mk := func(label string, text, image []float32) {
		if _, err := coll.InsertRecord(RecordInput{
			Payload: map[string]any{"label": label},
			Vectors: map[string][]float32{DefaultVectorSpace: text, "image": image},
		}); err != nil {
			t.Fatal(err)
		}
	}
	mk("a", []float32{1, 0}, []float32{1, 0})
	mk("b", []float32{0, 1}, []float32{1, 0})
	mk("c", []float32{1, 0}, []float32{0, 1})

	// "a" is top in both spaces, so fusion should rank it first.
	res, err := coll.MultiSpaceSearch(map[string][]float32{
		DefaultVectorSpace: {1, 0},
		"image":            {1, 0},
	}, TopK(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 || res[0].Record.Payload["label"] != "a" {
		t.Fatalf("fusion should rank 'a' first, got %+v", res)
	}
}

func TestEmbeddingProfileValidationOnInsert(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll, _ := db.CreateCollection("p",
		WithEmbeddingProfile(EmbeddingProfile{Provider: "openai", Model: "m", Dimension: 3, Distance: DistanceCosine}))

	if got, ok := coll.EmbeddingProfile(); !ok || got.Dimension != 3 {
		t.Fatalf("profile should be set, got %+v ok=%v", got, ok)
	}
	// Profile dimension propagates to collection dimension.
	if _, err := coll.InsertRecord(RecordInput{Vectors: map[string][]float32{DefaultVectorSpace: {1, 2}}}); !errors.Is(err, ErrDimensionMismatch) {
		t.Errorf("vector not matching profile dimension should fail, got %v", err)
	}
	if _, err := coll.InsertRecord(RecordInput{Vectors: map[string][]float32{DefaultVectorSpace: {1, 2, 3}}}); err != nil {
		t.Errorf("matching vector should insert, got %v", err)
	}
}

func TestNamedSpacePersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spaces.veclite")

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	coll, _ := db.CreateCollection("docs", WithDimension(3),
		WithVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 2, Modality: "image",
			HNSW: &HNSWConfig{M: 8, EfConstruction: 64, EfSearch: 32, UseHeuristic: true}}))
	if err := coll.SetEmbeddingProfile(EmbeddingProfile{Provider: "openai", Model: "text-3", Dimension: 3, Distance: DistanceCosine}); err != nil {
		t.Fatal(err)
	}
	if _, err := coll.InsertRecord(RecordInput{
		Content: "hello",
		Payload: map[string]any{"label": "x"},
		Vectors: map[string][]float32{DefaultVectorSpace: {1, 0, 0}, "image": {0, 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen and verify everything survived.
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("docs")
	if err != nil {
		t.Fatal(err)
	}

	if !coll2.HasVectorSpace("image") {
		t.Fatal("image space did not persist")
	}
	info, err := coll2.VectorSpace("image")
	if err != nil {
		t.Fatal(err)
	}
	if info.Modality != "image" || info.IndexType != string(IndexTypeHNSW) || info.Dimension != 2 {
		t.Fatalf("image space metadata not persisted: %+v", info)
	}
	if prof, ok := coll2.EmbeddingProfile(); !ok || prof.Model != "text-3" {
		t.Fatalf("profile not persisted: %+v ok=%v", prof, ok)
	}

	res, err := coll2.SearchSpace("image", []float32{0, 1}, TopK(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Record.Payload["label"] != "x" {
		t.Fatalf("image-space search after reload failed: %+v", res)
	}
}

func TestMigrateV3SnapshotToV4(t *testing.T) {
	// Craft a pre-v4 snapshot: single-vector collection, no VectorSpaces field.
	snap := NewDatabaseSnapshot()
	snap.Version = 3
	cs := NewCollectionSnapshot("legacy", 3, DistanceCosine)
	cs.VectorSpaces = nil // pre-v4 shape
	cs.Records = append(cs.Records, &storage.RecordSnapshot{ID: 1, Vector: []float32{1, 0, 0}})
	snap.Collections["legacy"] = cs

	migrated := storage.Migrate(snap)
	if migrated.Version != storage.CurrentVersion {
		t.Fatalf("expected version %d, got %d", storage.CurrentVersion, migrated.Version)
	}
	if migrated.Collections["legacy"].VectorSpaces == nil {
		t.Error("VectorSpaces should be initialized after migration")
	}
	// Original single vector is untouched and becomes the implicit default space.
	if got := migrated.Collections["legacy"].Records[0].Vector; len(got) != 3 || got[0] != 1 {
		t.Fatalf("default-space vector should be preserved, got %v", got)
	}
}

func TestFuseRRFPublic(t *testing.T) {
	mk := func(ids ...uint64) []Result {
		out := make([]Result, len(ids))
		for i, id := range ids {
			out[i] = Result{Record: &Record{ID: id}, Score: float32(len(ids) - i)}
		}
		return out
	}
	// id 2 appears high in both sets, so it should win.
	fused := FuseRRF([][]Result{mk(1, 2, 3), mk(2, 4)}, WithRRFK(60), WithFusionTopK(3))
	if len(fused) != 3 {
		t.Fatalf("expected 3 fused results, got %d", len(fused))
	}
	if fused[0].Record.ID != 2 {
		t.Fatalf("expected id 2 to rank first, got %d", fused[0].Record.ID)
	}
}

func TestDeleteRemovesNamedSpaceVectors(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll, _ := db.CreateCollection("d", WithDimension(2),
		WithVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 2,
			HNSW: &HNSWConfig{M: 8, EfConstruction: 64, EfSearch: 32, UseHeuristic: true}}))

	ids := make([]uint64, 0, 3)
	for _, label := range []string{"a", "b", "c"} {
		v := []float32{1, 0}
		if label == "b" {
			v = []float32{0, 1}
		}
		id, err := coll.InsertRecord(RecordInput{
			Payload: map[string]any{"label": label},
			Vectors: map[string][]float32{DefaultVectorSpace: v, "image": v},
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}

	sp := coll.spaces["image"]
	if got := sp.index.Count(); got != 3 {
		t.Fatalf("image index should hold 3 vectors, got %d", got)
	}

	// Delete the first-inserted record (often the HNSW graph entry point).
	if err := coll.Delete(ids[0]); err != nil {
		t.Fatal(err)
	}
	if got := sp.index.Count(); got != 2 {
		t.Fatalf("image index leaked: count = %d, want 2 after delete", got)
	}

	// The deleted record must not appear, and search must not be corrupted by a
	// dangling entry point.
	res, err := coll.SearchSpace("image", []float32{1, 0}, TopK(5))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Record.ID == ids[0] {
			t.Fatalf("deleted record still returned from image space: %+v", r)
		}
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 remaining results, got %d", len(res))
	}

	// DeleteWhere also cleans the space index.
	if _, err := coll.DeleteWhere(Equal("label", "c")); err != nil {
		t.Fatal(err)
	}
	if got := sp.index.Count(); got != 1 {
		t.Fatalf("DeleteWhere leaked: count = %d, want 1", got)
	}

	// Converting the last record to text-only removes its named-space vector too.
	if _, err := coll.UpsertTextDocument(ids[1], "now text only", map[string]any{"label": "b"}); err != nil {
		t.Fatal(err)
	}
	if got := sp.index.Count(); got != 0 {
		t.Fatalf("text-only conversion leaked: count = %d, want 0", got)
	}
}

func TestDefaultProfileValidatesPrimaryInsert(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	// Profile set AFTER creation must still be enforced by the primary Insert path.
	coll := db.Collection("p")
	if err := coll.SetEmbeddingProfile(EmbeddingProfile{Provider: "x", Model: "m", Dimension: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := coll.Insert([]float32{1, 2}, nil); !errors.Is(err, ErrDimensionMismatch) {
		t.Errorf("Insert of 2-d vector against dim-3 profile should fail, got %v", err)
	}
	if _, err := coll.Insert([]float32{1, 2, 3}, nil); err != nil {
		t.Errorf("Insert of matching 3-d vector should succeed, got %v", err)
	}
	if _, err := coll.Upsert(0, []float32{1, 2}, nil); !errors.Is(err, ErrDimensionMismatch) {
		t.Errorf("Upsert of 2-d vector against dim-3 profile should fail, got %v", err)
	}

	// A profile whose dimension conflicts with an established collection dimension is rejected.
	coll2, _ := db.CreateCollection("p2", WithDimension(2))
	if err := coll2.SetEmbeddingProfile(EmbeddingProfile{Dimension: 3}); !errors.Is(err, ErrProfileMismatch) {
		t.Errorf("conflicting profile dimension should fail with ErrProfileMismatch, got %v", err)
	}
}

func TestSetRecordVectorAddsSpace(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll, _ := db.CreateCollection("s", WithDimension(2),
		WithVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 2}))

	id, _ := coll.InsertRecord(RecordInput{Vectors: map[string][]float32{DefaultVectorSpace: {1, 0}}})
	// Record initially has no image vector.
	if _, err := coll.SearchSpace("image", []float32{1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := coll.SetRecordVector(id, "image", []float32{0, 1}); err != nil {
		t.Fatal(err)
	}
	res, err := coll.SearchSpace("image", []float32{0, 1}, TopK(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Record.ID != id {
		t.Fatalf("SetRecordVector should make the record searchable in the image space: %+v", res)
	}
}

func TestUpsertRecordByKeyInsertThenReplace(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	coll, err := db.CreateCollection("evidence",
		WithTextIndex("evidence_id"),
		WithVectorSpace(VectorSpaceConfig{Name: "text", Dimension: 3, Distance: DistanceCosine, Modality: "text"}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a new record by natural key.
	id1, inserted, err := coll.UpsertRecordByKey("evidence_id", "doc-1", RecordInput{
		Content: "clicking the task does nothing",
		Payload: map[string]any{"evidence_id": "doc-1", "bundle": "/b"},
		Vectors: map[string][]float32{"text": {1, 0, 0}},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !inserted {
		t.Fatalf("first upsert should report inserted=true")
	}
	if id1 == 0 {
		t.Fatal("inserted ID should be non-zero")
	}

	// Confirm it is searchable in the named text space.
	res, err := coll.SearchSpace("text", []float32{1, 0, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Record.ID != id1 {
		t.Fatalf("expected the inserted record, got %+v", res)
	}

	// Capture CreatedAt so we can assert it is preserved on replace.
	before, _ := coll.Get(id1)
	createdAt := before.CreatedAt

	// Replace the same natural key with new content, payload, and vector.
	id2, inserted2, err := coll.UpsertRecordByKey("evidence_id", "doc-1", RecordInput{
		Content: "clicking the task opens the wrong screen",
		Payload: map[string]any{"evidence_id": "doc-1", "bundle": "/b-revised"},
		Vectors: map[string][]float32{"text": {0, 1, 0}},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if inserted2 {
		t.Fatalf("replace upsert should report inserted=false")
	}
	if id2 != id1 {
		t.Fatalf("replace should reuse the same ID: got %d, want %d", id2, id1)
	}

	// No duplicate records by the natural key.
	dupes, _ := coll.Find(Equal("evidence_id", "doc-1"))
	if len(dupes) != 1 {
		t.Fatalf("expected exactly one record with evidence_id=doc-1, got %d", len(dupes))
	}

	// CreatedAt preserved, payload + content updated.
	after, _ := coll.Get(id2)
	if !after.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt should be preserved on replace: before %v, after %v", createdAt, after.CreatedAt)
	}
	if got := after.Payload["bundle"]; got != "/b-revised" {
		t.Fatalf("payload should be updated on replace: got %v", got)
	}
	if after.Content != "clicking the task opens the wrong screen" {
		t.Fatalf("content should be updated on replace: got %q", after.Content)
	}

	// The new vector is searchable; the collection must not have grown.
	resNew, err := coll.SearchSpace("text", []float32{0, 1, 0}, TopK(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(resNew) != 1 || resNew[0].Record.ID != id2 {
		t.Fatalf("expected the replaced record to be searchable with the new vector, got %+v", resNew)
	}
	if count := coll.Count(); count != 1 {
		t.Fatalf("replace should not grow the collection: got %d records", count)
	}
}

func TestUpsertRecordByKeyEmptyKeyFieldRejected(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll, _ := db.CreateCollection("c", WithTextIndex("k"))

	if _, _, err := coll.UpsertRecordByKey("", "v", RecordInput{Content: "x"}); !errors.Is(err, ErrInvalidVectorSpace) {
		t.Fatalf("empty keyField should return ErrInvalidVectorSpace, got %v", err)
	}
}

func TestUpsertRecordByKeyRejectsUnknownSpace(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll, _ := db.CreateCollection("c", WithTextIndex("evidence_id"))

	if _, _, err := coll.UpsertRecordByKey("evidence_id", "doc-1", RecordInput{
		Vectors: map[string][]float32{"unknown": {1, 0, 0}},
	}); !errors.Is(err, ErrVectorSpaceNotFound) {
		t.Fatalf("unknown space should return ErrVectorSpaceNotFound, got %v", err)
	}
}

func TestHybridSearchSpaceNamedSpace(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()

	coll, err := db.CreateCollection("evidence",
		WithTextIndex("kind"),
		WithVectorSpace(VectorSpaceConfig{Name: "text", Dimension: 3, Distance: DistanceCosine, Modality: "text"}),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Two records: one about "checkout failure", one about "login bug".
	if _, _, err := coll.UpsertRecordByKey("kind", "a", RecordInput{
		Content: "checkout fails when the cart is empty",
		Payload: map[string]any{"kind": "a"},
		Vectors: map[string][]float32{"text": {1, 0, 0}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := coll.UpsertRecordByKey("kind", "b", RecordInput{
		Content: "login screen throws a 500 on the second attempt",
		Payload: map[string]any{"kind": "b"},
		Vectors: map[string][]float32{"text": {0, 1, 0}},
	}); err != nil {
		t.Fatal(err)
	}

	// Hybrid query: a vector aligned with record "a" AND text mentioning "checkout".
	res, err := coll.HybridSearchSpace("text", []float32{1, 0, 0}, "checkout", TopK(10))
	if err != nil {
		t.Fatalf("HybridSearchSpace: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one hybrid result")
	}
	// Record "a" must be ranked first: it wins both the vector and the BM25 half.
	if res[0].Record.Payload["kind"] != "a" {
		t.Fatalf("expected record a to rank first, got %+v", res[0].Record.Payload)
	}

	// Sanity: HybridSearchSpace on DefaultVectorSpace behaves like a vector
	// search against the default space. With no default-space vectors present,
	// the vector half returns nothing and only BM25 contributes.
	bm25Only, err := coll.HybridSearchSpace(DefaultVectorSpace, []float32{1, 0, 0}, "login", TopK(10))
	if err != nil {
		t.Fatalf("HybridSearchSpace default space: %v", err)
	}
	if len(bm25Only) == 0 {
		t.Fatal("expected BM25-only results when the default space is empty")
	}
	if bm25Only[0].Record.Payload["kind"] != "b" {
		t.Fatalf("expected record b to win the BM25-only query, got %+v", bm25Only[0].Record.Payload)
	}
}

func TestHybridSearchSpaceRequiresTextIndex(t *testing.T) {
	db, _ := Open(":memory:")
	defer func() { _ = db.Close() }()
	coll, _ := db.CreateCollection("c",
		WithVectorSpace(VectorSpaceConfig{Name: "text", Dimension: 3}))

	if _, err := coll.HybridSearchSpace("text", []float32{1, 0, 0}, "q"); err == nil {
		t.Fatal("expected error when text index is not enabled")
	}
}
