package veclite

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	// File format constants
	fileMagic   = "VECLITE\x00"
	fileVersion = uint32(1)
	headerSize  = 32
)

// FileStorage is a file-based storage implementation.
// Uses gob encoding with atomic writes for durability.
type FileStorage struct {
	path string
}

// NewFileStorage creates a new file storage for the given path.
func NewFileStorage(path string) *FileStorage {
	return &FileStorage{path: path}
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
	defer file.Close()

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
	version := uint32(header[8]) | uint32(header[9])<<8 | uint32(header[10])<<16 | uint32(header[11])<<24
	if version != fileVersion {
		return nil, &StorageError{Op: "validate", Err: ErrInvalidVersion}
	}

	// Decode payload
	var snapshot DatabaseSnapshot
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, &StorageError{Op: "decode", Err: err}
	}

	return &snapshot, nil
}

// Save writes the database to the file using atomic write pattern.
// Writes to .tmp file, then renames old to .bak, then renames .tmp to final.
func (f *FileStorage) Save(snapshot *DatabaseSnapshot) error {
	// Ensure parent directory exists
	dir := filepath.Dir(f.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &StorageError{Op: "mkdir", Err: err}
		}
	}

	// Encode to buffer first
	var buf bytes.Buffer

	// Write header
	header := make([]byte, headerSize)
	copy(header[:8], fileMagic)
	header[8] = byte(fileVersion)
	header[9] = byte(fileVersion >> 8)
	header[10] = byte(fileVersion >> 16)
	header[11] = byte(fileVersion >> 24)
	// Flags and reserved are left as zeros
	buf.Write(header)

	// Encode snapshot
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(snapshot); err != nil {
		return &StorageError{Op: "encode", Err: err}
	}

	// Write to temp file
	tmpPath := f.path + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
		return &StorageError{Op: "write temp", Err: err}
	}

	// Check if original file exists
	bakPath := f.path + ".bak"
	if _, err := os.Stat(f.path); err == nil {
		// Rename original to backup
		if err := os.Rename(f.path, bakPath); err != nil {
			// Clean up temp file
			os.Remove(tmpPath)
			return &StorageError{Op: "backup", Err: err}
		}
	}

	// Rename temp to final
	if err := os.Rename(tmpPath, f.path); err != nil {
		// Try to restore backup
		if _, bakErr := os.Stat(bakPath); bakErr == nil {
			os.Rename(bakPath, f.path)
		}
		return &StorageError{Op: "rename", Err: err}
	}

	// Clean up backup (optional, ignore errors)
	os.Remove(bakPath)

	return nil
}

// Close is a no-op for file storage.
func (f *FileStorage) Close() error {
	return nil
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

// Delete removes the database file and any backup files.
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

	if len(errs) > 0 {
		return &StorageError{Op: "delete", Err: errors.Join(errs...)}
	}
	return nil
}

// Ensure FileStorage implements Storage.
var _ Storage = (*FileStorage)(nil)
