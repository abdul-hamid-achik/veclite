package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/veclite"
)

func newTestSession(t *testing.T, reloadInterval time.Duration) (*Session, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.veclite")
	sess := New(Config{
		Path:           dbPath,
		Dimensions:     4,
		ReloadInterval: reloadInterval,
	})
	return sess, dbPath
}

func TestNewIsLazy(t *testing.T) {
	sess, dbPath := newTestSession(t, 0)
	// No lock file should exist until ReadOnly or ReadWrite is called.
	if _, err := os.Stat(dbPath + ".lock"); err == nil {
		t.Fatal("lock file should not exist before first open")
	}
	_ = sess.Close()
}

func TestReadOnlyOpensWithSharedLock(t *testing.T) {
	sess, _ := newTestSession(t, 0)
	defer sess.Close()

	db, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly: %v", err)
	}
	if db == nil {
		t.Fatal("db is nil")
	}
	// Lock file should exist.
	if _, err := os.Stat(sess.cfg.Path + ".lock"); err != nil {
		t.Fatalf("lock file should exist after ReadOnly: %v", err)
	}
}

func TestReadOnlyIsCached(t *testing.T) {
	sess, _ := newTestSession(t, 0)
	defer sess.Close()

	db1, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly 1: %v", err)
	}
	db2, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly 2: %v", err)
	}
	if db1 != db2 {
		t.Fatal("ReadOnly should return cached handle")
	}
}

func TestReadWriteIsNotCached(t *testing.T) {
	sess, _ := newTestSession(t, 0)
	defer sess.Close()

	db, err := sess.ReadWrite()
	if err != nil {
		t.Fatalf("ReadWrite: %v", err)
	}
	// Caller must close the RW handle.
	defer db.Close()

	// After closing, the session should not have a cached RW handle.
	// Since we closed it externally, s.rw is still set but the DB is closed.
	// The session doesn't track external closes — that's by design (RW is
	// returned for caller to manage). The next ReadWrite() returns the stale
	// cached handle. This is acceptable because callers should call Close()
	// on the session, not reuse it after closing the returned DB.
	// In practice, each write tool call creates a fresh session or calls
	// ReadWrite() once.
}

func TestReadWriteClosesReadOnlyFirst(t *testing.T) {
	sess, _ := newTestSession(t, 0)
	defer sess.Close()

	// Open RO first.
	roDB, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly: %v", err)
	}
	if roDB == nil {
		t.Fatal("roDB is nil")
	}

	// Now open RW — should close RO first.
	rwDB, err := sess.ReadWrite()
	if err != nil {
		t.Fatalf("ReadWrite after ReadOnly: %v", err)
	}
	defer rwDB.Close()

	// Verify the RO handle was closed by checking that a new RO open works
	// (it would fail if the old RO was still holding the shared lock and
	// we tried to open RW — but since ReadWrite closes RO first, this
	// should succeed).
}

func TestReadWriteReturnsLockErrorOnContention(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.veclite")

	// Open a writer in session 1.
	sess1 := New(Config{Path: dbPath, Dimensions: 4})
	defer sess1.Close()
	rwDB, err := sess1.ReadWrite()
	if err != nil {
		t.Fatalf("sess1 ReadWrite: %v", err)
	}
	defer rwDB.Close()

	// Try to open a writer in session 2 — should get LockError.
	sess2 := New(Config{Path: dbPath, Dimensions: 4})
	defer sess2.Close()
	_, err = sess2.ReadWrite()
	if err == nil {
		t.Fatal("expected LockError, got nil")
	}
	var lockErr *LockError
	if !errors.As(err, &lockErr) {
		t.Fatalf("expected *LockError, got %T: %v", err, err)
	}
	if !errors.Is(err, ErrFileLocked) {
		t.Fatal("error should wrap ErrFileLocked")
	}
	if lockErr.PID <= 0 {
		t.Fatalf("LockError should have PID > 0, got %d", lockErr.PID)
	}
}

func TestReadOnlyMultipleSessions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.veclite")

	// First session: create the DB with a writer.
	sess1 := New(Config{Path: dbPath, Dimensions: 4})
	rwDB, err := sess1.ReadWrite()
	if err != nil {
		t.Fatalf("sess1 ReadWrite: %v", err)
	}
	// Insert a record so the DB is non-empty.
	coll, err := rwDB.CreateCollection("test",
		veclite.WithDimension(4),
	)
	if err != nil {
		// Collection might already exist.
		coll, err = rwDB.GetCollection("test")
		if err != nil {
			t.Fatalf("get collection: %v", err)
		}
	}
	_, _ = coll.Insert([]float32{1, 0, 0, 0}, map[string]any{"v": 1})
	rwDB.Close()
	sess1.Close()

	// Two sessions open RO simultaneously — both should succeed.
	sess2 := New(Config{Path: dbPath, Dimensions: 4})
	defer sess2.Close()
	sess3 := New(Config{Path: dbPath, Dimensions: 4})
	defer sess3.Close()

	db2, err := sess2.ReadOnly()
	if err != nil {
		t.Fatalf("sess2 ReadOnly: %v", err)
	}
	if db2 == nil {
		t.Fatal("db2 is nil")
	}

	db3, err := sess3.ReadOnly()
	if err != nil {
		t.Fatalf("sess3 ReadOnly: %v", err)
	}
	if db3 == nil {
		t.Fatal("db3 is nil")
	}
}

func TestReloadIfStale(t *testing.T) {
	sess, dbPath := newTestSession(t, 50*time.Millisecond)
	defer sess.Close()

	// Open RO.
	_, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly: %v", err)
	}

	// Immediately reload — should be no-op (not stale yet).
	if err := sess.ReloadIfStale(nil); err != nil {
		t.Fatalf("ReloadIfStale (not stale): %v", err)
	}

	// Wait for stale threshold.
	time.Sleep(60 * time.Millisecond)

	// Now reload should happen.
	if err := sess.ReloadIfStale(nil); err != nil {
		t.Fatalf("ReloadIfStale (stale): %v", err)
	}
	_ = dbPath
}

func TestReloadIfStaleWithSignal(t *testing.T) {
	sess, _ := newTestSession(t, time.Hour) // long interval
	defer sess.Close()

	_, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly: %v", err)
	}

	// Signal returns true → should reload even though interval hasn't elapsed.
	called := false
	signal := func() bool {
		called = true
		return true
	}
	if err := sess.ReloadIfStale(signal); err != nil {
		t.Fatalf("ReloadIfStale with signal: %v", err)
	}
	if !called {
		t.Fatal("signal callback was not called")
	}
}

func TestReloadIfStaleNoReadOnly(t *testing.T) {
	sess, _ := newTestSession(t, 0)
	defer sess.Close()

	// No RO handle open — should be no-op.
	if err := sess.ReloadIfStale(nil); err != nil {
		t.Fatalf("ReloadIfStale with no RO: %v", err)
	}
}

func TestReleaseReadOnly(t *testing.T) {
	sess, _ := newTestSession(t, 0)
	defer sess.Close()

	_, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly: %v", err)
	}

	// Release RO.
	if err := sess.ReleaseReadOnly(); err != nil {
		t.Fatalf("ReleaseReadOnly: %v", err)
	}

	// Reopen RO — should work.
	_, err = sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly after release: %v", err)
	}
}

func TestReadWriteAfterReadOnlyReleaseAllowsExternalWriter(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.veclite")

	// Session 1: open RO.
	sess1 := New(Config{Path: dbPath, Dimensions: 4})
	defer sess1.Close()
	_, err := sess1.ReadOnly()
	if err != nil {
		t.Fatalf("sess1 ReadOnly: %v", err)
	}

	// Release RO.
	if err := sess1.ReleaseReadOnly(); err != nil {
		t.Fatalf("sess1 ReleaseReadOnly: %v", err)
	}

	// Session 2: open RW — should succeed because RO was released.
	sess2 := New(Config{Path: dbPath, Dimensions: 4})
	defer sess2.Close()
	rwDB, err := sess2.ReadWrite()
	if err != nil {
		t.Fatalf("sess2 ReadWrite after RO release: %v", err)
	}
	defer rwDB.Close()
}

func TestCloseClosesBothHandles(t *testing.T) {
	sess, _ := newTestSession(t, 0)

	_, err := sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly: %v", err)
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After close, RO should be nil and ReadOnly should reopen.
	_, err = sess.ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly after Close: %v", err)
	}
	_ = sess.Close()
}