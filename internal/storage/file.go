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
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// File format constants
	fileMagic   = "VECLITE\x00"
	fileVersion = uint32(2) // v2: added KnowledgeGraphs, EpisodeStores, TF entries in BM25
	headerSize  = 32
	// Header layout:
	//   [0:8]   - magic bytes "VECLITE\0"
	//   [8:12]  - version (uint32 LE)
	//   [12:16] - CRC32 checksum of payload (uint32 LE)
	//   [16:32] - reserved (zeros)
)

// ErrFileLocked is returned when the database file is already locked by another process.
var ErrFileLocked = errors.New("veclite: database file is locked by another process")

// ErrChecksumMismatch is returned when the file checksum does not match.
var ErrChecksumMismatch = errors.New("veclite: checksum mismatch")

// ErrCorruptedFile is returned when the database file is corrupted.
var ErrCorruptedFile = errors.New("veclite: corrupted file")

// ErrInvalidVersion is returned when the file version is not supported.
var ErrInvalidVersion = errors.New("veclite: unsupported file version")

// File is a file-based storage implementation.
// Uses gob encoding with atomic writes for durability.
// Acquires an exclusive file lock to prevent concurrent access from multiple processes.
type File struct {
	path     string
	lockFile *os.File // held open for the duration to maintain the flock
}

// NewFile creates a new file storage for the given path.
func NewFile(path string) *File {
	return &File{path: path}
}

// writeLockInfo writes diagnostic info (PID and timestamp) to the lock file.
func writeLockInfo(f *os.File) {
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().Unix())
	_ = f.Sync()
}

// readLockInfo reads diagnostic info from a lock file and returns a human-readable string.
// Returns empty string if the lock file cannot be read or parsed.
func readLockInfo(f *os.File) string {
	_, _ = f.Seek(0, 0)
	buf := make([]byte, 128)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return ""
	}

	lines := strings.SplitN(strings.TrimSpace(string(buf[:n])), "\n", 3)
	if len(lines) < 2 {
		return ""
	}

	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return ""
	}

	ts, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		return fmt.Sprintf("PID %d", pid)
	}

	age := time.Since(time.Unix(ts, 0)).Truncate(time.Second)
	return fmt.Sprintf("PID %d, locked %s ago", pid, age)
}

// Lock acquires an exclusive file lock on a .lock file adjacent to the database.
// This prevents multiple processes from opening the same database.
func (f *File) Lock() error {
	lockPath := f.path + ".lock"

	// Ensure parent directory exists
	dir := filepath.Dir(lockPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &Error{Op: "mkdir for lock", Err: err}
		}
	}

	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return &Error{Op: "open lock file", Err: err}
	}

	if err := lockFile(lf); err != nil {
		info := readLockInfo(lf)
		_ = lf.Close()
		if info != "" {
			return &Error{Op: "acquire lock", Err: fmt.Errorf("%w (%s)", ErrFileLocked, info)}
		}
		return &Error{Op: "acquire lock", Err: ErrFileLocked}
	}

	// Write diagnostic info for other processes to read
	writeLockInfo(lf)

	f.lockFile = lf
	return nil
}

// Unlock releases the file lock.
func (f *File) Unlock() error {
	if f.lockFile == nil {
		return nil
	}

	lockPath := f.lockFile.Name()

	// Clear diagnostic info
	_ = f.lockFile.Truncate(0)

	err := unlockFile(f.lockFile)
	_ = f.lockFile.Close()
	f.lockFile = nil
	_ = os.Remove(lockPath)

	if err != nil {
		return &Error{Op: "release lock", Err: err}
	}
	return nil
}

// Load reads the database from the file.
// Returns nil, nil if the file doesn't exist yet.
func (f *File) Load() (*DatabaseSnapshot, error) {
	file, err := os.Open(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &Error{Op: "open", Err: err}
	}
	defer func() { _ = file.Close() }()

	// Read and validate header
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, &Error{Op: "read header", Err: ErrCorruptedFile}
	}

	// Validate magic
	if string(header[:8]) != fileMagic {
		return nil, &Error{Op: "validate", Err: ErrCorruptedFile}
	}

	// Validate version
	version := binary.LittleEndian.Uint32(header[8:12])
	if version > fileVersion {
		return nil, &Error{Op: "validate", Err: fmt.Errorf("%w (file: %d, supported: %d)", ErrInvalidVersion, version, fileVersion)}
	}

	// Read stored checksum
	storedChecksum := binary.LittleEndian.Uint32(header[12:16])

	// Read payload
	payload, err := io.ReadAll(file)
	if err != nil {
		return nil, &Error{Op: "read payload", Err: err}
	}

	// Validate checksum (only if non-zero, for backward compatibility with v1 files without checksums)
	if storedChecksum != 0 {
		computedChecksum := crc32.ChecksumIEEE(payload)
		if computedChecksum != storedChecksum {
			return nil, &Error{Op: "validate checksum", Err: ErrChecksumMismatch}
		}
	}

	// Decode payload
	var snapshot DatabaseSnapshot
	decoder := gob.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&snapshot); err != nil {
		wrappedErr := fmt.Errorf("gob decode failed: %w (ensure payload types are gob-encodable)", err)
		return nil, &Error{Op: "decode", Err: wrappedErr}
	}

	// Migrate from older versions
	if version < fileVersion {
		migrateSnapshot(&snapshot, version)
	}

	return &snapshot, nil
}

// Save writes the database to the file using atomic write pattern.
// Writes to .tmp file, fsyncs, then renames old to .bak, then renames .tmp to final.
func (f *File) Save(snapshot *DatabaseSnapshot) error {
	// Ensure parent directory exists
	dir := filepath.Dir(f.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &Error{Op: "mkdir", Err: err}
		}
	}

	// Encode payload to buffer
	var payloadBuf bytes.Buffer
	encoder := gob.NewEncoder(&payloadBuf)
	if err := encoder.Encode(snapshot); err != nil {
		return &Error{Op: "encode", Err: err}
	}

	payloadBytes := payloadBuf.Bytes()

	// Compute CRC32 checksum of the payload
	checksum := crc32.ChecksumIEEE(payloadBytes)

	// Build header
	header := make([]byte, headerSize)
	copy(header[:8], fileMagic)
	binary.LittleEndian.PutUint32(header[8:12], fileVersion)
	binary.LittleEndian.PutUint32(header[12:16], checksum)
	// Bytes 16-31 remain reserved (zeros)

	// Write to temp file with fsync
	tmpPath := f.path + ".tmp"
	if err := f.writeFileSync(tmpPath, header, payloadBytes); err != nil {
		return err
	}

	// Check if original file exists
	bakPath := f.path + ".bak"
	if _, err := os.Stat(f.path); err == nil {
		// Rename original to backup
		if err := os.Rename(f.path, bakPath); err != nil {
			// Clean up temp file
			_ = os.Remove(tmpPath)
			return &Error{Op: "backup", Err: err}
		}
	}

	// Rename temp to final
	if err := os.Rename(tmpPath, f.path); err != nil {
		// Try to restore backup
		if _, bakErr := os.Stat(bakPath); bakErr == nil {
			_ = os.Rename(bakPath, f.path)
		}
		return &Error{Op: "rename", Err: err}
	}

	// Fsync the directory to ensure the rename is durable
	if err := f.syncDir(dir); err != nil {
		// Non-fatal: the file is written, just might not survive power loss
		_ = err
	}

	// Clean up backup (optional, ignore errors)
	_ = os.Remove(bakPath)

	return nil
}

// writeFileSync writes data to a file and fsyncs before closing.
func (f *File) writeFileSync(path string, header, payload []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return &Error{Op: "create temp", Err: err}
	}

	if _, err := file.Write(header); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return &Error{Op: "write header", Err: err}
	}

	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return &Error{Op: "write payload", Err: err}
	}

	// Fsync to ensure data is flushed to disk
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return &Error{Op: "fsync", Err: err}
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return &Error{Op: "close temp", Err: err}
	}

	return nil
}

// syncDir fsyncs a directory to ensure rename durability.
func (f *File) syncDir(dir string) error {
	if dir == "" || dir == "." {
		dir = "."
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	_ = d.Close()
	return err
}

// Close releases the file lock.
func (f *File) Close() error {
	return f.Unlock()
}

// Path returns the file path.
func (f *File) Path() string {
	return f.path
}

// Exists returns true if the database file exists.
func (f *File) Exists() bool {
	_, err := os.Stat(f.path)
	return err == nil
}

// Delete removes the database file and any backup/lock files.
func (f *File) Delete() error {
	var errs []error

	if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if err := os.Remove(f.path + ".tmp"); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if err := os.Remove(f.path + ".bak"); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if err := os.Remove(f.path + ".lock"); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return &Error{Op: "delete", Err: errors.Join(errs...)}
	}
	return nil
}

// Ensure File implements Backend.
var _ Backend = (*File)(nil)

// migrateSnapshot applies migrations to bring an older snapshot format
// up to the current version. This handles backward compatibility when
// the schema changes between releases.
func migrateSnapshot(snapshot *DatabaseSnapshot, fromVersion uint32) {
	if fromVersion < 2 {
		// v1 -> v2: Added KnowledgeGraphs, EpisodeStores, and TF entries in BM25.
		// These are nil/empty in v1 files, so no data transformation needed —
		// just ensure the maps are initialized.
		if snapshot.KnowledgeGraphs == nil {
			snapshot.KnowledgeGraphs = make(map[string]*GraphSnapshot)
		}
		if snapshot.EpisodeStores == nil {
			snapshot.EpisodeStores = make(map[string]*EpisodeStoreSnapshot)
		}
		// Convert v1 InvertedIndexSnapshot.Postings (map[string][]uint64)
		// to v2 format (map[string][]TFEntry with count=1) if needed.
		// This is handled by gob's backward compatibility: v1 files that
		// decoded into map[string][]uint64 are compatible since v2 uses
		// map[string][]TFEntry and gob respects field presence.
	}
	snapshot.Version = fileVersion
}
