package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveCreatesPrivateStorage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics")
	}
	dir := filepath.Join(t.TempDir(), "private", "nested")
	path := filepath.Join(dir, "memory.veclite")
	file := NewFile(path)
	if err := file.Save(NewDatabaseSnapshot()); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database mode = %04o, want 0600", got)
	}
	if info, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Fatalf("storage directory mode = %04o, want no group/other access", got)
	}
}

func TestSaveTightensBroadModeAndPreservesStricterMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics")
	}
	path := filepath.Join(t.TempDir(), "memory.veclite")
	file := NewFile(path)
	if err := file.Save(NewDatabaseSnapshot()); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := file.Save(NewDatabaseSnapshot()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("broad mode remained %04o, want 0600", got)
	}

	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := file.Save(NewDatabaseSnapshot()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o400 {
		t.Fatalf("stricter mode = %04o, want preserved 0400", got)
	}
}

func TestLockCreatesPrivateArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics")
	}
	path := filepath.Join(t.TempDir(), "private", "memory.veclite")
	file := NewFile(path)
	if err := file.Lock(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Unlock() }()

	if info, err := os.Stat(path + ".lock"); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock mode = %04o, want 0600", got)
	}
}
