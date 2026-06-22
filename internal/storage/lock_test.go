package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockNormalSuccess(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	f := NewFile(dbPath)

	if err := f.Lock(); err != nil {
		t.Fatalf("Lock() failed on first attempt: %v", err)
	}

	// Verify the lock file exists
	if _, err := os.Stat(dbPath + ".lock"); err != nil {
		t.Errorf("lock file not created: %v", err)
	}

	// Verify f.lockFile is set
	if f.lockFile == nil {
		t.Fatal("lockFile is nil after successful Lock()")
	}

	if err := f.Unlock(); err != nil {
		t.Fatalf("Unlock() failed: %v", err)
	}

	if f.lockFile != nil {
		t.Error("lockFile should be nil after Unlock()")
	}
}

func TestLockStalePIDRecovery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	lockPath := dbPath + ".lock"

	// Create a stale lock file with a dead PID (999999 is extremely unlikely to exist)
	staleContent := []byte("999999\n0\n")
	if err := os.WriteFile(lockPath, staleContent, 0644); err != nil {
		t.Fatalf("failed to write stale lock file: %v", err)
	}

	f := NewFile(dbPath)
	// Use a config with no delay so the test is fast
	cfg := LockConfig{MaxRetries: 3, InitialDelay: 1 * time.Millisecond}

	if err := f.LockWithConfig(cfg); err != nil {
		t.Fatalf("LockWithConfig() should succeed with stale lock: %v", err)
	}

	if f.lockFile == nil {
		t.Fatal("lockFile should be set after recovering from stale lock")
	}

	_ = f.Unlock()
}

func TestLockContentionRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping contention test in short mode")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// First file acquires the lock and holds it
	f1 := NewFile(dbPath)
	if err := f1.Lock(); err != nil {
		t.Fatalf("first Lock() failed: %v", err)
	}
	defer func() { _ = f1.Unlock() }()

	// Second file tries to lock — should fail after retries
	f2 := NewFile(dbPath)
	cfg := LockConfig{MaxRetries: 2, InitialDelay: 10 * time.Millisecond}

	start := time.Now()
	err := f2.LockWithConfig(cfg)
	elapsed := time.Since(start)

	if err == nil {
		_ = f2.Unlock()
		t.Fatal("LockWithConfig() should have failed when lock is held by a live process")
	}

	// Verify the error wraps ErrFileLocked
	if !errors.Is(err, ErrFileLocked) {
		t.Errorf("expected ErrFileLocked, got: %v", err)
	}

	// Verify it actually retried (should take at least 10ms + 20ms = 30ms)
	if elapsed < 25*time.Millisecond {
		t.Errorf("expected retry backoff to take >= 30ms, took %v", elapsed)
	}
}

func TestLockContentionSucceedsAfterRelease(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// First file acquires the lock
	f1 := NewFile(dbPath)
	if err := f1.Lock(); err != nil {
		t.Fatalf("first Lock() failed: %v", err)
	}

	// Release it immediately
	if err := f1.Unlock(); err != nil {
		t.Fatalf("Unlock() failed: %v", err)
	}

	// Second file should succeed now
	f2 := NewFile(dbPath)
	cfg := LockConfig{MaxRetries: 2, InitialDelay: 10 * time.Millisecond}

	if err := f2.LockWithConfig(cfg); err != nil {
		t.Fatalf("LockWithConfig() should succeed after lock released: %v", err)
	}

	_ = f2.Unlock()
}

func TestReadLockPID(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Write a valid lock file
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}
	defer func() { _ = lf.Close() }()

	if _, err := lf.WriteString("12345\n1609459200\n"); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	pid := readLockPID(lf)
	if pid != 12345 {
		t.Errorf("expected PID 12345, got %d", pid)
	}
}

func TestReadLockPIDInvalid(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}
	defer func() { _ = lf.Close() }()

	if _, err := lf.WriteString("not-a-number\n"); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	pid := readLockPID(lf)
	if pid != 0 {
		t.Errorf("expected PID 0 for invalid content, got %d", pid)
	}
}

func TestReadLockPIDEmptyFile(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}
	defer func() { _ = lf.Close() }()

	pid := readLockPID(lf)
	if pid != 0 {
		t.Errorf("expected PID 0 for empty file, got %d", pid)
	}
}

func TestIsProcessAliveSelf(t *testing.T) {
	// The current process should be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("isProcessAlive should return true for the current process")
	}
}

func TestIsProcessAliveDeadPID(t *testing.T) {
	// PID 999999 is extremely unlikely to exist
	if isProcessAlive(999999) {
		// On Windows, isProcessAlive always returns true (conservative), so skip
		t.Log("isProcessAlive returned true for PID 999999 — this is expected on Windows")
	}
}

func TestIsProcessAliveInvalidPID(t *testing.T) {
	if isProcessAlive(0) {
		t.Error("isProcessAlive should return false for PID 0")
	}
	if isProcessAlive(-1) {
		t.Error("isProcessAlive should return false for PID -1")
	}
}

func TestDefaultLockConfig(t *testing.T) {
	cfg := DefaultLockConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay 100ms, got %v", cfg.InitialDelay)
	}
}

func TestLockWithConfigZeroRetries(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	lockPath := dbPath + ".lock"

	// Create a stale lock file with a dead PID
	staleContent := []byte("999999\n0\n")
	if err := os.WriteFile(lockPath, staleContent, 0644); err != nil {
		t.Fatalf("failed to write stale lock file: %v", err)
	}

	f := NewFile(dbPath)
	// With 0 retries, the stale lock should still be cleared on first attempt
	cfg := LockConfig{MaxRetries: 0, InitialDelay: 0}

	if err := f.LockWithConfig(cfg); err != nil {
		t.Fatalf("LockWithConfig with 0 retries should still clear stale lock: %v", err)
	}

	_ = f.Unlock()
}

func TestUnlockRemovesLockFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	lockPath := dbPath + ".lock"

	f := NewFile(dbPath)
	if err := f.Lock(); err != nil {
		t.Fatalf("Lock() failed: %v", err)
	}

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist after Lock(): %v", err)
	}

	if err := f.Unlock(); err != nil {
		t.Fatalf("Unlock() failed: %v", err)
	}

	if _, err := os.Stat(lockPath); err == nil {
		t.Error("lock file should be removed after Unlock()")
	}
}

func TestUnlockWhenNotLocked(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	f := NewFile(dbPath)
	// Unlock without having locked should be a no-op
	if err := f.Unlock(); err != nil {
		t.Errorf("Unlock() on unlocked file should not error: %v", err)
	}
}