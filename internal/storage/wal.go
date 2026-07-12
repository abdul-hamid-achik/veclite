package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync/atomic"
)

// The write-ahead log (WAL) is an append-only sidecar file (dbpath + ".wal")
// holding mutations applied since the last full-snapshot Save. Each entry is a
// self-contained gob blob framed with a length and CRC32 so that a torn tail
// (crash mid-append) is detected and discarded on open. A successful Save makes
// the log redundant, so it is truncated back to its header.
//
// Entries form a redo log of final record states: replaying them in order onto
// the last saved snapshot is idempotent and reproduces the pre-crash state.
const (
	walMagic      = "VECWAL\x00\x00"
	walVersion    = uint32(1)
	walHeaderSize = 16
	// Header layout:
	//   [0:8]   - magic bytes "VECWAL\0\0"
	//   [8:12]  - version (uint32 LE)
	//   [12:16] - reserved (zeros)

	// walFrameSize is the per-entry frame prefix: payload length + CRC32.
	walFrameSize = 8

	// walMaxEntrySize bounds a single entry so a corrupt length field cannot
	// trigger a huge allocation while scanning.
	walMaxEntrySize = 256 << 20 // 256 MiB
)

// ErrCorruptedWAL is returned when the WAL header is not recognisable. Torn or
// corrupt entries after a valid header are not an error — the valid prefix is
// used and the tail is discarded.
var ErrCorruptedWAL = errors.New("veclite: corrupted WAL file")

// WAL entry op codes.
const (
	// WALOpUpsertRecord stores the full post-mutation state of one record.
	WALOpUpsertRecord uint8 = 1
	// WALOpDeleteRecord removes one record by ID.
	WALOpDeleteRecord uint8 = 2
	// WALOpCollectionConfig creates a collection or updates its configuration
	// (dimension, distance, index config, spaces, profile, metadata). The
	// snapshot carries no records or serialized indexes.
	WALOpCollectionConfig uint8 = 3
	// WALOpDropCollection removes a collection and all its data.
	WALOpDropCollection uint8 = 4
	// WALOpDBMetadata replaces the database-level metadata map.
	WALOpDBMetadata uint8 = 5
	// WALOpGraph replaces a knowledge graph's full state. Graphs are small
	// relative to record data, so full-state entries keep replay trivially
	// idempotent (Collection carries the graph name).
	WALOpGraph uint8 = 6
	// WALOpEpisodeStore replaces an episode store's full state (Collection
	// carries the store's registry key).
	WALOpEpisodeStore uint8 = 7
)

// WALEntry is one logged mutation. Which fields are set depends on Op.
type WALEntry struct {
	Op         uint8
	Collection string
	Record     *RecordSnapshot       // WALOpUpsertRecord
	RecordID   uint64                // WALOpDeleteRecord
	Config     *CollectionSnapshot   // WALOpCollectionConfig (Records/index snapshots nil)
	Metadata   map[string]any        // WALOpDBMetadata
	Graph      *GraphSnapshot        // WALOpGraph
	Episodes   *EpisodeStoreSnapshot // WALOpEpisodeStore
}

// WALPath returns the WAL sidecar path for a database file path.
func WALPath(dbPath string) string {
	return dbPath + ".wal"
}

// WAL is an open write-ahead log positioned for appends.
type WAL struct {
	path string
	f    *os.File
	// size tracks the current file size. Writes happen under the owner's
	// append/reset serialization; reads may come from any goroutine (e.g.
	// checkpoint threshold checks), hence atomic.
	size atomic.Int64
}

// OpenWAL opens (creating if needed) the WAL at path, scans it, and returns
// the valid entries recorded so far. A torn or corrupt tail is truncated away
// so subsequent appends start from the last valid entry. The returned WAL is
// positioned for Append.
func OpenWAL(path string) (*WAL, []WALEntry, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, nil, &Error{Op: "open wal", Err: err}
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, nil, &Error{Op: "secure wal", Err: err}
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, &Error{Op: "stat wal", Err: err}
	}

	// A file shorter than the header can only result from a crash during
	// creation, before any entry was durably appended: reinitialise it.
	if info.Size() < walHeaderSize {
		if err := initWALHeader(f); err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		w := &WAL{path: path, f: f}
		w.size.Store(walHeaderSize)
		return w, nil, nil
	}

	if err := validateWALHeader(f); err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	entries, validEnd, err := scanWALEntries(f)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	// Discard any torn/corrupt tail so appends resume from a clean boundary.
	if validEnd < info.Size() {
		if err := f.Truncate(validEnd); err != nil {
			_ = f.Close()
			return nil, nil, &Error{Op: "truncate wal tail", Err: err}
		}
	}
	if _, err := f.Seek(validEnd, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, nil, &Error{Op: "seek wal", Err: err}
	}

	w := &WAL{path: path, f: f}
	w.size.Store(validEnd)
	return w, entries, nil
}

// ReadWALEntries scans the WAL at path without modifying it and returns the
// valid entries. Returns nil, nil if no WAL file exists. A torn tail (e.g. a
// concurrent writer mid-append) is ignored.
func ReadWALEntries(path string) ([]WALEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &Error{Op: "open wal", Err: err}
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, &Error{Op: "stat wal", Err: err}
	}
	if info.Size() < walHeaderSize {
		return nil, nil
	}
	if err := validateWALHeader(f); err != nil {
		return nil, err
	}

	entries, _, err := scanWALEntries(f)
	return entries, err
}

// initWALHeader truncates the file and writes a fresh header.
func initWALHeader(f *os.File) error {
	if err := f.Truncate(0); err != nil {
		return &Error{Op: "init wal", Err: err}
	}
	header := make([]byte, walHeaderSize)
	copy(header[:8], walMagic)
	binary.LittleEndian.PutUint32(header[8:12], walVersion)
	if _, err := f.WriteAt(header, 0); err != nil {
		return &Error{Op: "write wal header", Err: err}
	}
	if err := f.Sync(); err != nil {
		return &Error{Op: "fsync wal header", Err: err}
	}
	if _, err := f.Seek(walHeaderSize, io.SeekStart); err != nil {
		return &Error{Op: "seek wal", Err: err}
	}
	return nil
}

// validateWALHeader checks magic and version. The caller must have verified
// the file is at least walHeaderSize long.
func validateWALHeader(f *os.File) error {
	header := make([]byte, walHeaderSize)
	if _, err := f.ReadAt(header, 0); err != nil {
		return &Error{Op: "read wal header", Err: err}
	}
	if string(header[:8]) != walMagic {
		return &Error{Op: "validate wal", Err: ErrCorruptedWAL}
	}
	version := binary.LittleEndian.Uint32(header[8:12])
	if version > walVersion {
		return &Error{Op: "validate wal", Err: fmt.Errorf("%w (wal: %d, supported: %d)", ErrInvalidVersion, version, walVersion)}
	}
	return nil
}

// scanWALEntries reads entries starting after the header and returns them
// together with the offset just past the last valid entry. Torn or corrupt
// data stops the scan without error — everything at and after that point is
// considered discarded.
func scanWALEntries(f *os.File) ([]WALEntry, int64, error) {
	var entries []WALEntry
	offset := int64(walHeaderSize)
	frame := make([]byte, walFrameSize)

	for {
		if _, err := f.ReadAt(frame, offset); err != nil {
			// io.EOF / io.ErrUnexpectedEOF: clean end or torn frame header.
			break
		}
		length := binary.LittleEndian.Uint32(frame[0:4])
		crc := binary.LittleEndian.Uint32(frame[4:8])
		if length == 0 || length > walMaxEntrySize {
			break // corrupt length — discard from here
		}

		payload := make([]byte, length)
		if _, err := f.ReadAt(payload, offset+walFrameSize); err != nil {
			break // torn payload
		}
		if crc32.ChecksumIEEE(payload) != crc {
			break // corrupt payload — discard from here on
		}

		var entry WALEntry
		if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&entry); err != nil {
			break // undecodable despite CRC match — treat as corrupt tail
		}

		entries = append(entries, entry)
		offset += walFrameSize + int64(length)
	}

	return entries, offset, nil
}

// Append encodes and appends the entries as one durable batch: a single
// write followed by one fsync (group commit).
func (w *WAL) Append(entries []WALEntry) error {
	if len(entries) == 0 {
		return nil
	}

	var batch bytes.Buffer
	frame := make([]byte, walFrameSize)
	for i := range entries {
		var payload bytes.Buffer
		if err := gob.NewEncoder(&payload).Encode(&entries[i]); err != nil {
			return &Error{Op: "encode wal entry", Err: err}
		}
		binary.LittleEndian.PutUint32(frame[0:4], uint32(payload.Len()))
		binary.LittleEndian.PutUint32(frame[4:8], crc32.ChecksumIEEE(payload.Bytes()))
		batch.Write(frame)
		batch.Write(payload.Bytes())
	}

	n, err := w.f.Write(batch.Bytes())
	// Count bytes that reached the file even on a short write, so the size
	// used for checkpoint decisions tracks the real file. A torn tail from a
	// failed append is discarded (and the counter recomputed) at next open.
	w.size.Add(int64(n))
	if err != nil {
		return &Error{Op: "append wal", Err: err}
	}
	if err := w.f.Sync(); err != nil {
		return &Error{Op: "fsync wal", Err: err}
	}
	return nil
}

// Reset truncates the log back to its header. Called after a successful full
// snapshot Save makes the logged entries redundant.
func (w *WAL) Reset() error {
	if err := w.f.Truncate(walHeaderSize); err != nil {
		return &Error{Op: "reset wal", Err: err}
	}
	// The file is logically header-sized from here on; update the counter
	// before the fallible seek/sync steps so a later error can't leave a
	// stale-high size driving spurious checkpoints.
	w.size.Store(walHeaderSize)
	if _, err := w.f.Seek(walHeaderSize, io.SeekStart); err != nil {
		return &Error{Op: "seek wal", Err: err}
	}
	if err := w.f.Sync(); err != nil {
		return &Error{Op: "fsync wal", Err: err}
	}
	return nil
}

// Size returns the current log file size in bytes (header included).
// Safe to call from any goroutine.
func (w *WAL) Size() int64 {
	return w.size.Load()
}

// Path returns the WAL file path.
func (w *WAL) Path() string {
	return w.path
}

// Close closes the WAL file handle. It does not remove the file: entries not
// yet folded into a snapshot must survive for replay on the next open.
func (w *WAL) Close() error {
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	if err != nil {
		return &Error{Op: "close wal", Err: err}
	}
	return nil
}

// Remove closes the WAL and deletes the file. Used when a database opened
// without WAL support folds a leftover log into a snapshot.
func (w *WAL) Remove() error {
	closeErr := w.Close()
	removeErr := os.Remove(w.path)
	if removeErr != nil && os.IsNotExist(removeErr) {
		removeErr = nil
	}
	if closeErr != nil || removeErr != nil {
		return &Error{Op: "remove wal", Err: errors.Join(closeErr, removeErr)}
	}
	return nil
}
