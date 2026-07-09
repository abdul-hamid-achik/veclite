package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeLock(t *testing.T, dbPath string, pid int, ts time.Time) string {
	t.Helper()
	lockPath := dbPath + ".lock"
	content := fmt.Sprintf("%d\n%d\n", pid, ts.Unix())
	if err := os.WriteFile(lockPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fake lock: %v", err)
	}
	return lockPath
}

func TestUnlockNoLockFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.veclite")
	var out bytes.Buffer
	if err := unlockDB(dbPath, false, &out); err != nil {
		t.Fatalf("unlockDB: %v", err)
	}
	if !strings.Contains(out.String(), "no lock file") {
		t.Fatalf("expected 'no lock file' message, got: %q", out.String())
	}
}

func TestUnlockRemovesDeadPIDLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.veclite")
	// PID beyond any real pid range on macOS/Linux → guaranteed dead.
	lockPath := writeFakeLock(t, dbPath, 99999999, time.Now().Add(-time.Hour))

	var out bytes.Buffer
	if err := unlockDB(dbPath, false, &out); err != nil {
		t.Fatalf("unlockDB: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("stale lock file should have been removed")
	}
	if !strings.Contains(out.String(), "removed stale lock") {
		t.Fatalf("expected stale-lock message, got: %q", out.String())
	}
}

func TestUnlockRefusesLivePIDWithoutForce(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.veclite")
	// Our own PID is definitely alive.
	lockPath := writeFakeLock(t, dbPath, os.Getpid(), time.Now())

	var out bytes.Buffer
	err := unlockDB(dbPath, false, &out)
	if err == nil {
		t.Fatal("unlockDB should refuse to remove a live process's lock")
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatal("lock file should NOT have been removed without --force")
	}
	if !strings.Contains(out.String(), "refusing") {
		t.Fatalf("expected refusal message, got: %q", out.String())
	}
}

func TestUnlockForceRemovesLivePIDLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.veclite")
	lockPath := writeFakeLock(t, dbPath, os.Getpid(), time.Now())

	var out bytes.Buffer
	if err := unlockDB(dbPath, true, &out); err != nil {
		t.Fatalf("unlockDB --force: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should have been force-removed")
	}
	if !strings.Contains(out.String(), "WARNING") {
		t.Fatalf("expected force-removal warning, got: %q", out.String())
	}
}

func TestUnlockRemovesUnparseableLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.veclite")
	lockPath := dbPath + ".lock"
	if err := os.WriteFile(lockPath, []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("writing garbage lock: %v", err)
	}

	var out bytes.Buffer
	if err := unlockDB(dbPath, false, &out); err != nil {
		t.Fatalf("unlockDB: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("unparseable lock file should have been removed")
	}
}
