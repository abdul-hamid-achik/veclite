package veclite

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// crashDB abandons a database without saving, the way a killed process would:
// background workers stop, the file lock and WAL handle are released (the
// kernel would do this on process death), but no snapshot is written.
func crashDB(t *testing.T, db *DB) {
	t.Helper()

	db.stopMu.Lock()
	stops := append([]func(){}, db.stopFuncs...)
	db.stopFuncs = nil
	db.stopMu.Unlock()
	for _, stop := range stops {
		stop()
	}

	db.mu.Lock()
	db.closed = true
	db.mu.Unlock()

	if err := db.storage.Close(); err != nil {
		t.Fatalf("release storage: %v", err)
	}
	if db.wal != nil {
		if err := db.wal.Close(); err != nil {
			t.Fatalf("release wal: %v", err)
		}
	}
}

func walTestPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "wal_test.veclite")
}

func TestWALCrashRecovery(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	coll, err := db.CreateCollection("docs",
		WithDimension(3),
		WithHNSW(16, 200),
		WithTextIndex("title"),
		WithVectorSpace(VectorSpaceConfig{Name: "image", Dimension: 2}),
	)
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}

	id1, err := coll.Insert([]float32{1, 0, 0}, map[string]any{"title": "alpha document"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id2, err := coll.Insert([]float32{0, 1, 0}, map[string]any{"title": "beta document"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id3, err := coll.InsertRecord(RecordInput{
		Vectors: map[string][]float32{
			DefaultVectorSpace: {0, 0, 1},
			"image":            {0.5, 0.5},
		},
		Payload: map[string]any{"title": "gamma multimodal"},
	})
	if err != nil {
		t.Fatalf("insert record: %v", err)
	}

	// Mutations beyond plain inserts: update, delete, metadata.
	if err := coll.Update(id2, map[string]any{"title": "beta updated"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := coll.Delete(id1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.SetMetadataValue("owner", "wal-test"); err != nil {
		t.Fatalf("set metadata: %v", err)
	}

	// An empty collection created but never written to must also survive.
	if db.Collection("empty") == nil {
		t.Fatal("create empty collection")
	}

	crashDB(t, db)

	// Nothing was ever synced: without the WAL the file wouldn't even exist.
	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()

	coll2, err := db2.GetCollection("docs")
	if err != nil {
		t.Fatalf("collection lost after crash: %v", err)
	}
	if got := coll2.Count(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if _, err := coll2.Get(id1); err == nil {
		t.Fatal("deleted record resurrected by replay")
	}

	rec2, err := coll2.Get(id2)
	if err != nil {
		t.Fatalf("get id2: %v", err)
	}
	if rec2.Payload["title"] != "beta updated" {
		t.Fatalf("update lost: title = %v", rec2.Payload["title"])
	}

	// Vector search must work on the replayed HNSW index.
	results, err := coll2.Search([]float32{0, 0, 1}, TopK(1))
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Record.ID != id3 {
		t.Fatalf("search results = %+v, want id %d", results, id3)
	}

	// The named space must have been replayed too.
	spaceResults, err := coll2.SearchSpace("image", []float32{0.5, 0.5}, TopK(1))
	if err != nil {
		t.Fatalf("search space: %v", err)
	}
	if len(spaceResults) != 1 || spaceResults[0].Record.ID != id3 {
		t.Fatalf("space results = %+v, want id %d", spaceResults, id3)
	}

	// BM25 text index is rebuilt during replay.
	textResults, err := coll2.TextSearch("gamma", TopK(5))
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(textResults) != 1 || textResults[0].Record.ID != id3 {
		t.Fatalf("text results = %+v, want id %d", textResults, id3)
	}

	if got := db2.Metadata()["owner"]; got != "wal-test" {
		t.Fatalf("db metadata lost: owner = %v", got)
	}
	if !db2.HasCollection("empty") {
		t.Fatal("empty collection lost after crash")
	}

	// Recovery folds the log into a snapshot and truncates it.
	info, err := os.Stat(storage.WALPath(path))
	if err != nil {
		t.Fatalf("stat wal: %v", err)
	}
	if info.Size() != 16 {
		t.Fatalf("wal size after fold = %d, want header only (16)", info.Size())
	}
}

func TestWALRecoveryWithoutOption(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll := db.Collection("notes")
	if _, err := coll.Insert([]float32{1, 2}, map[string]any{"n": 1}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	crashDB(t, db)

	// Reopen WITHOUT WithWAL: the leftover log must still be recovered,
	// folded into a snapshot, and removed.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	coll2, err := db2.GetCollection("notes")
	if err != nil {
		t.Fatalf("collection lost: %v", err)
	}
	if coll2.Count() != 1 {
		t.Fatalf("count = %d, want 1", coll2.Count())
	}
	if _, err := os.Stat(storage.WALPath(path)); !os.IsNotExist(err) {
		t.Fatalf("wal file should be removed after fold (err=%v)", err)
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Third open: data must be in the snapshot now.
	db3, err := Open(path)
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	defer func() { _ = db3.Close() }()
	coll3, err := db3.GetCollection("notes")
	if err != nil {
		t.Fatalf("collection lost from snapshot: %v", err)
	}
	if coll3.Count() != 1 {
		t.Fatalf("count = %d, want 1", coll3.Count())
	}
}

func TestWALTruncatedOnSync(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll := db.Collection("c")
	if _, err := coll.Insert([]float32{1}, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}

	walPath := storage.WALPath(path)
	info, err := os.Stat(walPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() <= 16 {
		t.Fatalf("wal should contain entries before sync, size = %d", info.Size())
	}

	if err := db.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	info, err = os.Stat(walPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 16 {
		t.Fatalf("wal size after sync = %d, want 16", info.Size())
	}

	// Post-sync writes land in the log again; a crash must lose nothing.
	if _, err := coll.Insert([]float32{2}, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("get collection: %v", err)
	}
	if coll2.Count() != 2 {
		t.Fatalf("count = %d, want 2", coll2.Count())
	}
}

func TestWALDropCollectionReplay(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	keep := db.Collection("keep")
	if _, err := keep.Insert([]float32{1}, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	gone := db.Collection("gone")
	if _, err := gone.Insert([]float32{1}, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	// Drop after the snapshot: only the WAL knows.
	if err := db.DropCollection("gone"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	if db2.HasCollection("gone") {
		t.Fatal("dropped collection resurrected by replay")
	}
	if !db2.HasCollection("keep") {
		t.Fatal("kept collection lost")
	}
}

func TestWALReadOnlyReplayLeavesLogIntact(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll := db.Collection("c")
	if _, err := coll.Insert([]float32{1, 2, 3}, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	crashDB(t, db)

	walPath := storage.WALPath(path)
	before, err := os.Stat(walPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	ro, err := Open(path, WithReadOnly(true))
	if err != nil {
		t.Fatalf("read-only open: %v", err)
	}
	roColl, err := ro.GetCollection("c")
	if err != nil {
		t.Fatalf("read-only replay missed collection: %v", err)
	}
	if roColl.Count() != 1 {
		t.Fatalf("read-only count = %d, want 1", roColl.Count())
	}
	if err := ro.Close(); err != nil {
		t.Fatalf("close read-only: %v", err)
	}

	after, err := os.Stat(walPath)
	if err != nil {
		t.Fatalf("wal file vanished after read-only open: %v", err)
	}
	if after.Size() != before.Size() {
		t.Fatalf("read-only open modified the wal: %d -> %d", before.Size(), after.Size())
	}

	// A writer must still be able to recover everything afterwards.
	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("writer reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("writer recovery: %v", err)
	}
	if coll2.Count() != 1 {
		t.Fatalf("writer count = %d, want 1", coll2.Count())
	}
}

func TestWALTornTailIgnoredOnRecovery(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll := db.Collection("c")
	for i := 0; i < 5; i++ {
		if _, err := coll.Insert([]float32{float32(i)}, nil); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	crashDB(t, db)

	// Simulate a crash mid-append on top of the crash: garbage tail.
	f, err := os.OpenFile(storage.WALPath(path), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	if _, err := f.Write([]byte{0x01, 0x02}); err != nil {
		t.Fatalf("append garbage: %v", err)
	}
	_ = f.Close()

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("get collection: %v", err)
	}
	if coll2.Count() != 5 {
		t.Fatalf("count = %d, want 5", coll2.Count())
	}
}

func TestWALConcurrentWriters(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll, err := db.CreateCollection("c", WithDimension(4), WithHNSW(16, 200))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	const goroutines = 8
	const perGoroutine = 40
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				vec := []float32{float32(g), float32(i), 1, 2}
				if _, err := coll.Insert(vec, map[string]any{"g": g, "i": i}); err != nil {
					t.Errorf("insert: %v", err)
					return
				}
			}
		}(g)
	}
	// Concurrent syncs exercise the snapshot/truncate vs append interleaving.
	for s := 0; s < 3; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = db.Sync()
		}()
	}
	wg.Wait()
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("get collection: %v", err)
	}
	if got := coll2.Count(); got != goroutines*perGoroutine {
		t.Fatalf("count after crash = %d, want %d", got, goroutines*perGoroutine)
	}
}

func TestWALUpsertAndVectorUpdateReplay(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll, err := db.CreateCollection("c", WithDimension(2), WithHNSW(16, 200))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id, err := coll.Insert([]float32{1, 0}, map[string]any{"v": 1})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	// Post-snapshot vector update: replay must fix both record and index.
	if err := coll.UpdateVector(id, []float32{0, 1}); err != nil {
		t.Fatalf("update vector: %v", err)
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	vec, err := coll2.GetVector(id)
	if err != nil {
		t.Fatalf("get vector: %v", err)
	}
	if vec[0] != 0 || vec[1] != 1 {
		t.Fatalf("vector = %v, want [0 1]", vec)
	}
	results, err := coll2.Search([]float32{0, 1}, TopK(1))
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Record.ID != id {
		t.Fatalf("results = %+v", results)
	}
}

func TestWALTextDocumentsReplay(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll, err := db.CreateCollection("docs", WithTextIndex("tag"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := coll.InsertTextDocument(fmt.Sprintf("document number %d about veclite", i), map[string]any{"tag": "docs"}); err != nil {
			t.Fatalf("insert text: %v", err)
		}
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("docs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	results, err := coll2.TextSearch("veclite", TopK(10))
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("text results = %d, want 3", len(results))
	}
}

func TestWALNotCreatedWithoutOption(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Collection("c").Insert([]float32{1}, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := os.Stat(storage.WALPath(path)); !os.IsNotExist(err) {
		t.Fatalf("wal file created without WithWAL (err=%v)", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestWALCleanCloseLeavesEmptyLog(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Collection("c").Insert([]float32{1, 2}, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	info, err := os.Stat(storage.WALPath(path))
	if err != nil {
		t.Fatalf("stat wal: %v", err)
	}
	if info.Size() != 16 {
		t.Fatalf("wal size after clean close = %d, want 16", info.Size())
	}

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if coll.Count() != 1 {
		t.Fatalf("count = %d, want 1", coll.Count())
	}
}

func TestWALSharedReaderReloadSeesWriterWrites(t *testing.T) {
	path := walTestPath(t)

	writer, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	defer func() { _ = writer.Close() }()
	coll := writer.Collection("c")
	if _, err := coll.Insert([]float32{1, 2}, map[string]any{"n": 1}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Persist a base snapshot so the reader can open the file at all.
	if err := writer.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	reader, err := Open(path, WithReadOnly(true), WithSharedRead(true))
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Writer keeps writing; only the WAL knows about these.
	if _, err := coll.Insert([]float32{3, 4}, map[string]any{"n": 2}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := reader.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	rColl, err := reader.GetCollection("c")
	if err != nil {
		t.Fatalf("reader get: %v", err)
	}
	if rColl.Count() != 2 {
		t.Fatalf("reader count after reload = %d, want 2 (WAL replay)", rColl.Count())
	}
}

func TestWALKnowledgeGraphReplay(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	kg, err := db.CreateKnowledgeGraph("brain")
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	if err := kg.AddEntity(Entity{ID: "go", Type: "lang", Name: "Go", Vector: []float32{1, 0}}); err != nil {
		t.Fatalf("add entity: %v", err)
	}
	if err := kg.AddEntity(Entity{ID: "hnsw", Type: "algo", Name: "HNSW", Vector: []float32{0, 1}}); err != nil {
		t.Fatalf("add entity: %v", err)
	}
	if err := kg.AddEntity(Entity{ID: "gone", Type: "tmp", Name: "Gone"}); err != nil {
		t.Fatalf("add entity: %v", err)
	}
	if err := kg.AddRelationship(Relationship{ID: "r1", SourceID: "go", TargetID: "hnsw", Type: "implements"}); err != nil {
		t.Fatalf("add relationship: %v", err)
	}
	if err := kg.AddRelationship(Relationship{ID: "r2", SourceID: "hnsw", TargetID: "go", Type: "temp"}); err != nil {
		t.Fatalf("add relationship: %v", err)
	}
	if err := kg.DeleteRelationship("r2"); err != nil {
		t.Fatalf("delete relationship: %v", err)
	}
	if err := kg.DeleteEntity("gone"); err != nil {
		t.Fatalf("delete entity: %v", err)
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()

	kg2, err := db2.GetKnowledgeGraph("brain")
	if err != nil {
		t.Fatalf("graph lost after crash: %v", err)
	}
	if _, err := kg2.GetEntity("go"); err != nil {
		t.Fatalf("entity lost: %v", err)
	}
	if _, err := kg2.GetEntity("gone"); err == nil {
		t.Fatal("deleted entity resurrected")
	}
	rel, err := kg2.GetRelationship("r1")
	if err != nil {
		t.Fatalf("relationship lost: %v", err)
	}
	if rel.SourceID != "go" || rel.TargetID != "hnsw" {
		t.Fatalf("relationship corrupted: %+v", rel)
	}
	if _, err := kg2.GetRelationship("r2"); err == nil {
		t.Fatal("deleted relationship resurrected")
	}
	rels := kg2.GetRelationships("go", "outgoing")
	if len(rels) != 1 || rels[0].ID != "r1" {
		t.Fatalf("adjacency lost: %+v", rels)
	}
}

func TestWALEpisodeStoreReplay(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll := db.Collection("memories")
	var ids []uint64
	for i := 0; i < 3; i++ {
		id, err := coll.Insert([]float32{float32(i), 1}, map[string]any{"i": i})
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		ids = append(ids, id)
	}
	es, err := db.CreateEpisodeStore("memories")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	ep, err := es.CreateEpisode(ids, "first session")
	if err != nil {
		t.Fatalf("create episode: %v", err)
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()

	es2, err := db2.GetEpisodeStore("memories")
	if err != nil {
		t.Fatalf("episode store lost after crash: %v", err)
	}
	got, err := es2.GetEpisode(ep.ID)
	if err != nil {
		t.Fatalf("episode lost: %v", err)
	}
	if got.Title != "first session" || len(got.RecordIDs) != 3 {
		t.Fatalf("episode corrupted: %+v", got)
	}
	records, err := es2.ExpandEpisode(ep.ID)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expanded records = %d, want 3", len(records))
	}
}

// Regression: with WithSyncOnWrite, graph mutations trigger a full DB sync
// that snapshots the graph. Holding kg.mu across the triggering collection
// write used to self-deadlock.
func TestKnowledgeGraphSyncOnWriteNoDeadlock(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithSyncOnWrite(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		kg, err := db.CreateKnowledgeGraph("g")
		if err != nil {
			done <- err
			return
		}
		if err := kg.AddEntity(Entity{ID: "a", Type: "t", Name: "A", Vector: []float32{1, 2}}); err != nil {
			done <- err
			return
		}
		if err := kg.AddEntity(Entity{ID: "b", Type: "t", Name: "B", Vector: []float32{2, 1}}); err != nil {
			done <- err
			return
		}
		if err := kg.AddRelationship(Relationship{ID: "r", SourceID: "a", TargetID: "b", Type: "x"}); err != nil {
			done <- err
			return
		}
		done <- kg.DeleteEntity("b")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graph ops: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock: graph mutation with WithSyncOnWrite did not complete")
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Everything must have been persisted by the per-write syncs.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	kg2, err := db2.GetKnowledgeGraph("g")
	if err != nil {
		t.Fatalf("graph not persisted: %v", err)
	}
	if _, err := kg2.GetEntity("a"); err != nil {
		t.Fatalf("entity not persisted: %v", err)
	}
}

// Regression: UpsertByKey's update-existing branch must be WAL-logged
// (the insert branch was covered; the update branch was initially missed).
func TestWALUpsertByKeyUpdateReplay(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll := db.Collection("c")
	id, _, err := coll.UpsertByKey("key", "k1", []float32{1, 0}, map[string]any{"key": "k1", "v": 1})
	if err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if err := db.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	// Post-snapshot update of the existing record: only the WAL knows.
	id2, created, err := coll.UpsertByKey("key", "k1", []float32{0, 1}, map[string]any{"key": "k1", "v": 2})
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	if created || id2 != id {
		t.Fatalf("expected update of %d, got id=%d created=%v", id, id2, created)
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	rec, err := coll2.Get(id)
	if err != nil {
		t.Fatalf("record lost: %v", err)
	}
	if got, ok := rec.Payload["v"].(int); !ok || got != 2 {
		t.Fatalf("update lost after crash: payload v = %v, want 2", rec.Payload["v"])
	}
	vec, err := coll2.GetVector(id)
	if err != nil {
		t.Fatalf("vector: %v", err)
	}
	if vec[0] != 0 || vec[1] != 1 {
		t.Fatalf("vector update lost: %v", vec)
	}
}

// Regression: InsertTurn's reply path mutates the parent (child list) and the
// child (thread root) after the child's insert is logged; both must be
// re-logged or replay yields a torn thread.
func TestWALConversationThreadReplay(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll := db.Collection("conv")
	rootID, err := coll.InsertTurn(ConversationTurn{
		SessionID: "s1",
		Role:      "user",
		Content:   "hello",
		Vector:    []float32{1, 0},
	})
	if err != nil {
		t.Fatalf("insert root: %v", err)
	}
	childID, err := coll.InsertTurn(ConversationTurn{
		SessionID:     "s1",
		Role:          "assistant",
		Content:       "hi there",
		Vector:        []float32{0, 1},
		ParentChunkID: rootID,
	})
	if err != nil {
		t.Fatalf("insert reply: %v", err)
	}
	crashDB(t, db)

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("conv")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	parent, err := coll2.Get(rootID)
	if err != nil {
		t.Fatalf("parent lost: %v", err)
	}
	children := getChildIDs(parent.Payload)
	if len(children) != 1 || children[0] != childID {
		t.Fatalf("parent child-link lost after crash: %v", children)
	}
	child, err := coll2.Get(childID)
	if err != nil {
		t.Fatalf("child lost: %v", err)
	}
	if got := getThreadRoot(child.Payload); got != rootID {
		t.Fatalf("child thread-root lost after crash: %d, want %d", got, rootID)
	}

	thread, err := coll2.GetThread(childID)
	if err != nil {
		t.Fatalf("thread: %v", err)
	}
	if len(thread) != 2 {
		t.Fatalf("thread length after replay = %d, want 2", len(thread))
	}
}

// Regression: a replayed upsert whose vector does not match the collection
// dimension must be skipped without destroying the prior record state.
func TestWALReplayDimensionMismatchKeepsOldRecord(t *testing.T) {
	path := walTestPath(t)

	db, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	coll, err := db.CreateCollection("c", WithDimension(3))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id, err := coll.Insert([]float32{1, 2, 3}, map[string]any{"v": 1})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Craft a corrupt log: an upsert for the same ID with the wrong dimension.
	wal, _, err := storage.OpenWAL(storage.WALPath(path))
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	bad := storage.WALEntry{
		Op:         storage.WALOpUpsertRecord,
		Collection: "c",
		Record:     &storage.RecordSnapshot{ID: id, Vector: []float32{9, 9}, Payload: map[string]any{"v": 666}},
	}
	if err := wal.Append([]storage.WALEntry{bad}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("close wal: %v", err)
	}

	db2, err := Open(path, WithWAL(true))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	coll2, err := db2.GetCollection("c")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	rec, err := coll2.Get(id)
	if err != nil {
		t.Fatalf("prior record destroyed by bad replay entry: %v", err)
	}
	if got, ok := rec.Payload["v"].(int); !ok || got != 1 {
		t.Fatalf("record corrupted by bad replay entry: %+v", rec.Payload)
	}
}
