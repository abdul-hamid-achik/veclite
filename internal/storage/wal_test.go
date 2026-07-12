package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

func testWALPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.veclite.wal")
}

func TestWALOpenCreatesHeader(t *testing.T) {
	path := testWALPath(t)

	wal, entries, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	defer func() { _ = wal.Close() }()

	if len(entries) != 0 {
		t.Fatalf("new WAL returned %d entries, want 0", len(entries))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != walHeaderSize {
		t.Fatalf("new WAL size = %d, want %d", info.Size(), walHeaderSize)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("WAL permissions = %v, want 0600", info.Mode().Perm())
	}
}

func TestWALAppendAndReplay(t *testing.T) {
	path := testWALPath(t)

	wal, _, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}

	batch1 := []WALEntry{
		{Op: WALOpCollectionConfig, Collection: "docs", Config: NewCollectionSnapshot("docs", 3, floats.DistanceCosine)},
		{Op: WALOpUpsertRecord, Collection: "docs", Record: &RecordSnapshot{
			ID:      1,
			Vector:  []float32{1, 2, 3},
			Payload: map[string]any{"title": "first", "rank": 7},
			Content: "hello world",
		}},
	}
	if err := wal.Append(batch1); err != nil {
		t.Fatalf("Append batch1: %v", err)
	}
	batch2 := []WALEntry{
		{Op: WALOpDeleteRecord, Collection: "docs", RecordID: 1},
		{Op: WALOpDBMetadata, Metadata: map[string]any{"owner": "test"}},
		{Op: WALOpDropCollection, Collection: "old"},
	}
	if err := wal.Append(batch2); err != nil {
		t.Fatalf("Append batch2: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and verify all entries round-trip in order.
	wal2, entries, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = wal2.Close() }()

	if len(entries) != 5 {
		t.Fatalf("replayed %d entries, want 5", len(entries))
	}
	ops := []uint8{WALOpCollectionConfig, WALOpUpsertRecord, WALOpDeleteRecord, WALOpDBMetadata, WALOpDropCollection}
	for i, want := range ops {
		if entries[i].Op != want {
			t.Errorf("entry %d op = %d, want %d", i, entries[i].Op, want)
		}
	}
	rec := entries[1].Record
	if rec == nil || rec.ID != 1 || rec.Content != "hello world" {
		t.Fatalf("record entry mismatch: %+v", rec)
	}
	if got := rec.Payload["title"]; got != "first" {
		t.Errorf("payload title = %v, want first", got)
	}
	if entries[2].RecordID != 1 {
		t.Errorf("delete RecordID = %d, want 1", entries[2].RecordID)
	}
	if entries[3].Metadata["owner"] != "test" {
		t.Errorf("metadata = %v", entries[3].Metadata)
	}
	if entries[4].Collection != "old" {
		t.Errorf("drop collection = %q, want old", entries[4].Collection)
	}
}

func TestWALReset(t *testing.T) {
	path := testWALPath(t)

	wal, _, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	if err := wal.Append([]WALEntry{{Op: WALOpDeleteRecord, Collection: "c", RecordID: 9}}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != walHeaderSize {
		t.Fatalf("size after Reset = %d, want %d", info.Size(), walHeaderSize)
	}

	// Appends continue to work after a reset.
	if err := wal.Append([]WALEntry{{Op: WALOpDropCollection, Collection: "x"}}); err != nil {
		t.Fatalf("Append after Reset: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := ReadWALEntries(path)
	if err != nil {
		t.Fatalf("ReadWALEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].Op != WALOpDropCollection {
		t.Fatalf("entries after reset+append = %+v, want single drop", entries)
	}
}

func TestWALTornTailTruncated(t *testing.T) {
	path := testWALPath(t)

	wal, _, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	if err := wal.Append([]WALEntry{{Op: WALOpDeleteRecord, Collection: "c", RecordID: 1}}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate a crash mid-append: garbage where the next frame would start.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open for corruption: %v", err)
	}
	if _, err := f.Write([]byte{0xde, 0xad, 0xbe}); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	_ = f.Close()

	wal2, entries, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("reopen with torn tail: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 (valid prefix)", len(entries))
	}

	// The torn bytes must be gone so new appends land on a clean boundary.
	info, _ := os.Stat(path)
	sizeBefore := info.Size()
	if err := wal2.Append([]WALEntry{{Op: WALOpDeleteRecord, Collection: "c", RecordID: 2}}); err != nil {
		t.Fatalf("Append after truncation: %v", err)
	}
	_ = wal2.Close()

	entries, err = ReadWALEntries(path)
	if err != nil {
		t.Fatalf("ReadWALEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries after re-append = %d, want 2 (sizeBefore=%d)", len(entries), sizeBefore)
	}
}

func TestWALCorruptPayloadStopsScan(t *testing.T) {
	path := testWALPath(t)

	wal, _, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	if err := wal.Append([]WALEntry{
		{Op: WALOpDeleteRecord, Collection: "c", RecordID: 1},
		{Op: WALOpDeleteRecord, Collection: "c", RecordID: 2},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	_ = wal.Close()

	// Flip a byte inside the second entry's payload.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	data[len(data)-3] ^= 0xff
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := ReadWALEntries(path)
	if err != nil {
		t.Fatalf("ReadWALEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].RecordID != 1 {
		t.Fatalf("entries = %+v, want only record 1", entries)
	}
}

func TestWALReadMissingFile(t *testing.T) {
	entries, err := ReadWALEntries(filepath.Join(t.TempDir(), "absent.wal"))
	if err != nil {
		t.Fatalf("ReadWALEntries on missing file: %v", err)
	}
	if entries != nil {
		t.Fatalf("entries = %+v, want nil", entries)
	}
}

func TestWALBadMagicRejected(t *testing.T) {
	path := testWALPath(t)
	if err := os.WriteFile(path, []byte("NOTAWAL\x00________________"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := OpenWAL(path); err == nil {
		t.Fatal("OpenWAL accepted a file with bad magic")
	}
	if _, err := ReadWALEntries(path); err == nil {
		t.Fatal("ReadWALEntries accepted a file with bad magic")
	}
}

func TestWALShortFileReinitialised(t *testing.T) {
	path := testWALPath(t)
	// A crash during creation can leave a partial header.
	if err := os.WriteFile(path, []byte("VECW"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	wal, entries, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL on short file: %v", err)
	}
	defer func() { _ = wal.Close() }()
	if len(entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(entries))
	}
	info, _ := os.Stat(path)
	if info.Size() != walHeaderSize {
		t.Fatalf("size = %d, want %d", info.Size(), walHeaderSize)
	}
}

func TestFileDeleteRemovesWAL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.veclite")
	walPath := WALPath(dbPath)

	if err := os.WriteFile(dbPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	f := NewFile(dbPath)
	if err := f.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(walPath); !os.IsNotExist(err) {
		t.Fatalf("WAL file still present after Delete (err=%v)", err)
	}
}
