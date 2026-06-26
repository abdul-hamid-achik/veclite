// Package session provides a lazy dual-handle database session that resolves
// the multi-process lock contention problem with veclite's file-based storage.
//
// Read paths use a shared flock (LOCK_SH via WithReadOnly + WithSharedRead),
// allowing multiple reader processes to coexist. Write paths use an exclusive
// flock (LOCK_EX) — only one writer at a time. Because LOCK_SH and LOCK_EX are
// mutually exclusive, the session automatically closes any cached read-only
// handle before opening a read-write one, so a long-lived process (e.g. an MCP
// server) can transition from reading to writing without deadlocking.
//
// Handles are lazy: New() does not open anything. The first ReadOnly() or
// ReadWrite() call opens the database. This means an idle process that never
// queries (e.g. an MCP server waiting for tool calls) never holds a lock.
//
// Read-only handles are cached for fast repeated searches. Read-write handles
// are returned uncached — the caller must Close() the *veclite.DB after use so
// the exclusive lock is released, allowing other processes to open the
// database again.
//
// ReloadIfStale re-reads the database from disk so a cached read-only handle
// can pick up writes performed by another process (e.g. the daemon or CLI
// index). It is a no-op if no read-only handle is open or if the reload
// interval has not elapsed.
//
// # Quick start
//
//	sess := session.New(session.Config{
//	    Path:       "data.veclite",
//	    Dimensions:  768,
//	    ReloadInterval: 5 * time.Second,
//	})
//
//	// Read-only (shared lock, cached):
//	db, err := sess.ReadOnly()
//	// ... search ...
//
//	// Pick up external writes:
//	_ = sess.ReloadIfStale(nil)
//
//	// Read-write (closes RO first, returns uncached handle for caller to close):
//	db, err := sess.ReadWrite()
//	defer db.Close()
//	// ... insert/update/delete ...
//
//	// Clean shutdown:
//	_ = sess.Close()
package session

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite"
	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// Config holds session parameters.
type Config struct {
	// Path is the veclite database file path.
	Path string
	// Dimensions is the embedding vector dimension. Passed to veclite.Open
	// for collection creation. Set to 0 to auto-detect on first insert.
	Dimensions int
	// HNSW holds HNSW index tuning. Zero values fall back to veclite defaults.
	HNSW veclite.HNSWConfig
	// ReloadInterval controls auto-reload of the read-only handle: if
	// non-zero, ReloadIfStale reloads when this duration has elapsed since
	// the last reload. Zero means never auto-reload (caller must call Reload
	// manually or use the signal callback).
	ReloadInterval time.Duration
}

// LockError wraps veclite.ErrFileLocked with diagnostic info (PID, lock age)
// parsed from the .lock file. It implements errors.Is(err, ErrFileLocked).
type LockError struct {
	PID int
	Age time.Duration
	err error
}

// Error returns a human-readable description of the lock contention.
func (e *LockError) Error() string {
	if e.PID > 0 {
		return fmt.Sprintf("veclite: database file is locked by PID %d (locked %s ago)", e.PID, e.Age.Truncate(time.Second))
	}
	return e.err.Error()
}

// Unwrap returns the underlying error for errors.Is checks.
func (e *LockError) Unwrap() error {
	return e.err
}

// ErrFileLocked is re-exported for errors.Is checks without importing the
// storage package.
var ErrFileLocked = veclite.ErrFileLocked

// Session manages lazy dual-handle access to a veclite database.
type Session struct {
	cfg Config

	mu  sync.Mutex
	ro  *veclite.DB // cached read-only handle (shared lock), nil when not open
	rw  *veclite.DB // cached read-write handle (exclusive lock), nil when not open

	lastReload time.Time
}

// New creates a new session. No database is opened until ReadOnly() or
// ReadWrite() is called.
func New(cfg Config) *Session {
	return &Session{cfg: cfg}
}

// ReadOnly returns a *veclite.DB opened with WithReadOnly + WithSharedRead
// (shared flock). The handle is cached: the first call opens the database,
// subsequent calls return the same handle.
//
// If a read-write handle is already cached, it is returned directly (it can
// serve reads and is already current). This avoids a same-process flock
// conflict: flock is per-file-description, so opening a second handle to the
// same file in the same process would deadlock.
func (s *Session) ReadOnly() (*veclite.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rw != nil {
		return s.rw, nil
	}
	if s.ro != nil {
		return s.ro, nil
	}

	db, err := veclite.Open(s.cfg.Path,
		veclite.WithReadOnly(true),
		veclite.WithSharedRead(true),
	)
	if err != nil {
		return nil, err
	}
	s.ro = db
	s.lastReload = time.Now()
	return db, nil
}

// ReadWrite returns a *veclite.DB opened with an exclusive flock for writing.
//
// The handle is NOT cached — the caller must call db.Close() after use so the
// exclusive lock is released, allowing other processes to access the database.
//
// If a read-only handle is cached, it is closed first (releasing the shared
// lock) so the exclusive lock can be acquired. If a read-write handle is
// already cached (e.g. from a previous call that didn't close), it is returned
// directly.
//
// On lock contention (another process holds the lock), returns *LockError
// with PID and lock-age diagnostics parsed from the .lock file.
func (s *Session) ReadWrite() (*veclite.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rw != nil {
		return s.rw, nil
	}

	// Close cached RO so LOCK_EX can be acquired (LOCK_SH and LOCK_EX are
	// mutually exclusive).
	if s.ro != nil {
		_ = s.ro.Close()
		s.ro = nil
	}

	db, err := veclite.Open(s.cfg.Path)
	if err != nil {
		if errors.Is(err, veclite.ErrFileLocked) {
			return nil, &LockError{
				PID: storage.ReadLockPID(s.cfg.Path),
				Age: lockAge(s.cfg.Path),
				err: err,
			}
		}
		return nil, err
	}
	s.rw = db
	return db, nil
}

// ReloadIfStale reloads the cached read-only handle from disk if the reload
// interval has elapsed since the last reload, or if the signal callback
// returns true.
//
// The optional signal callback lets callers provide a cheaper check (e.g.
// stat a daemon.json mtime) — if it returns true, the reload happens
// immediately regardless of the time threshold.
//
// No-op if no read-only handle is open, if a read-write handle is cached
// (already current), or if the reload interval is zero and no signal is
// provided.
func (s *Session) ReloadIfStale(signal func() bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// RW handle is always current — no reload needed.
	if s.rw != nil {
		return nil
	}
	// No RO handle to reload.
	if s.ro == nil {
		return nil
	}

	stale := false
	if s.cfg.ReloadInterval > 0 && time.Since(s.lastReload) > s.cfg.ReloadInterval {
		stale = true
	}
	if signal != nil && signal() {
		stale = true
	}
	if !stale {
		return nil
	}

	if err := s.ro.Reload(); err != nil {
		return err
	}
	s.lastReload = time.Now()
	return nil
}

// ReleaseReadOnly closes the cached read-only handle if one is open, releasing
// the shared lock. This is useful before an external operation that needs the
// exclusive lock (e.g. a CLI sub-process that opens its own writer).
//
// The next ReadOnly() call will re-open the handle.
func (s *Session) ReleaseReadOnly() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ro != nil {
		err := s.ro.Close()
		s.ro = nil
		return err
	}
	return nil
}

// Close closes any cached handles and releases their locks.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	if s.ro != nil {
		if err := s.ro.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close read-only: %w", err))
		}
		s.ro = nil
	}
	if s.rw != nil {
		if err := s.rw.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close read-write: %w", err))
		}
		s.rw = nil
	}
	return errors.Join(errs...)
}

// lockAge reads the lock file at dbPath + ".lock" and returns how long ago it
// was acquired. Returns 0 if the lock file can't be read or parsed.
func lockAge(dbPath string) time.Duration {
	lockPath := dbPath + ".lock"
	f, err := os.Open(lockPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	buf := make([]byte, 128)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return 0
	}
	// Parse "PID\nTIMESTAMP\n" format written by writeLockInfo.
	lines := splitLines(string(buf[:n]), 3)
	if len(lines) < 2 {
		return 0
	}
	ts, err := parseInt64(lines[1])
	if err != nil {
		return 0
	}
	return time.Since(time.Unix(ts, 0))
}

func splitLines(s string, n int) []string {
	s = trimSpace(s)
	if s == "" {
		return nil
	}
	var lines []string
	for i := 0; i < n; i++ {
		idx := indexByte(s, '\n')
		if idx < 0 {
			lines = append(lines, s)
			break
		}
		lines = append(lines, s[:idx])
		s = s[idx+1:]
	}
	return lines
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func parseInt64(s string) (int64, error) {
	var n int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a digit")
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}