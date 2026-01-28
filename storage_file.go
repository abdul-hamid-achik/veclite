package veclite

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
)

const (
	// File format constants
	fileMagic   = "VECLITE\x00"
	fileVersion = uint32(1)
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

// FileStorage is a file-based storage implementation.
// Uses gob encoding with atomic writes for durability.
// Acquires an exclusive file lock to prevent concurrent access from multiple processes.
type FileStorage struct {
	path     string
	lockFile *os.File // held open for the duration to maintain the flock
}

// NewFileStorage creates a new file storage for the given path.
func NewFileStorage(path string) *FileStorage {
	return &FileStorage{path: path}
}

// Lock acquires an exclusive file lock on a .lock file adjacent to the database.
// This prevents multiple processes from opening the same database.
func (f *FileStorage) Lock() error {
	lockPath := f.path + ".lock"

	// Ensure parent directory exists
	dir := filepath.Dir(lockPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &StorageError{Op: "mkdir for lock", Err: err}
		}
	}

	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return &StorageError{Op: "open lock file", Err: err}
	}

	if err := lockFile(lf); err != nil {
		_ = lf.Close()
		return &StorageError{Op: "acquire lock", Err: ErrFileLocked}
	}

	f.lockFile = lf
	return nil
}

// Unlock releases the file lock.
func (f *FileStorage) Unlock() error {
	if f.lockFile == nil {
		return nil
	}

	lockPath := f.lockFile.Name()
	err := unlockFile(f.lockFile)
	_ = f.lockFile.Close()
	f.lockFile = nil
	_ = os.Remove(lockPath)

	if err != nil {
		return &StorageError{Op: "release lock", Err: err}
	}
	return nil
}

// Load reads the database from the file.
// Returns nil, nil if the file doesn't exist yet.
func (f *FileStorage) Load() (*DatabaseSnapshot, error) {
	file, err := os.Open(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &StorageError{Op: "open", Err: err}
	}
	defer func() { _ = file.Close() }()

	// Read and validate header
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, &StorageError{Op: "read header", Err: ErrCorruptedFile}
	}

	// Validate magic
	if string(header[:8]) != fileMagic {
		return nil, &StorageError{Op: "validate", Err: ErrCorruptedFile}
	}

	// Validate version
	version := binary.LittleEndian.Uint32(header[8:12])
	if version != fileVersion {
		return nil, &StorageError{Op: "validate", Err: ErrInvalidVersion}
	}

	// Read stored checksum
	storedChecksum := binary.LittleEndian.Uint32(header[12:16])

	// Read payload
	payload, err := io.ReadAll(file)
	if err != nil {
		return nil, &StorageError{Op: "read payload", Err: err}
	}

	// Validate checksum (only if non-zero, for backward compatibility with v1 files without checksums)
	if storedChecksum != 0 {
		computedChecksum := crc32.ChecksumIEEE(payload)
		if computedChecksum != storedChecksum {
			return nil, &StorageError{Op: "validate checksum", Err: ErrChecksumMismatch}
		}
	}

	// Decode payload
	var snapshot DatabaseSnapshot
	decoder := gob.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, &StorageError{Op: "decode", Err: err}
	}

	return &snapshot, nil
}

// Save writes the database to the file using atomic write pattern.
// Writes to .tmp file, fsyncs, then renames old to .bak, then renames .tmp to final.
func (f *FileStorage) Save(snapshot *DatabaseSnapshot) error {
	// Ensure parent directory exists
	dir := filepath.Dir(f.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &StorageError{Op: "mkdir", Err: err}
		}
	}

	// Encode payload to buffer
	var payloadBuf bytes.Buffer
	encoder := gob.NewEncoder(&payloadBuf)
	if err := encoder.Encode(snapshot); err != nil {
		return &StorageError{Op: "encode", Err: err}
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
			return &StorageError{Op: "backup", Err: err}
		}
	}

	// Rename temp to final
	if err := os.Rename(tmpPath, f.path); err != nil {
		// Try to restore backup
		if _, bakErr := os.Stat(bakPath); bakErr == nil {
			_ = os.Rename(bakPath, f.path)
		}
		return &StorageError{Op: "rename", Err: err}
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
func (f *FileStorage) writeFileSync(path string, header, payload []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return &StorageError{Op: "create temp", Err: err}
	}

	if _, err := file.Write(header); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return &StorageError{Op: "write header", Err: err}
	}

	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return &StorageError{Op: "write payload", Err: err}
	}

	// Fsync to ensure data is flushed to disk
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return &StorageError{Op: "fsync", Err: err}
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return &StorageError{Op: "close temp", Err: err}
	}

	return nil
}

// syncDir fsyncs a directory to ensure rename durability.
func (f *FileStorage) syncDir(dir string) error {
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
func (f *FileStorage) Close() error {
	return f.Unlock()
}

// Path returns the file path.
func (f *FileStorage) Path() string {
	return f.path
}

// Exists returns true if the database file exists.
func (f *FileStorage) Exists() bool {
	_, err := os.Stat(f.path)
	return err == nil
}

// Delete removes the database file and any backup/lock files.
func (f *FileStorage) Delete() error {
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
		return &StorageError{Op: "delete", Err: errors.Join(errs...)}
	}
	return nil
}

// Ensure FileStorage implements Storage.
var _ Storage = (*FileStorage)(nil)
