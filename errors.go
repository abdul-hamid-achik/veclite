package veclite

import (
	"errors"
	"fmt"

	"github.com/abdul-hamid-achik/veclite/internal/storage"
)

// Sentinel errors for common conditions.
var (
	// ErrNotFound is returned when a record or collection is not found.
	ErrNotFound = errors.New("veclite: not found")

	// ErrDimensionMismatch is returned when vector dimensions don't match.
	ErrDimensionMismatch = errors.New("veclite: dimension mismatch")

	// ErrEmptyVector is returned when an empty vector is provided.
	ErrEmptyVector = errors.New("veclite: empty vector")

	// ErrCollectionExists is returned when trying to create a collection that already exists.
	ErrCollectionExists = errors.New("veclite: collection already exists")

	// ErrDatabaseClosed is returned when operations are attempted on a closed database.
	ErrDatabaseClosed = errors.New("veclite: database closed")

	// ErrInvalidPath is returned when an invalid file path is provided.
	ErrInvalidPath = errors.New("veclite: invalid path")

	// ErrBatchSizeMismatch is returned when batch operation input sizes don't match.
	ErrBatchSizeMismatch = errors.New("veclite: batch size mismatch")

	// ErrReadOnly is returned when a write operation is attempted on a read-only database.
	ErrReadOnly = errors.New("veclite: database is read-only")
)

// DimensionError provides details about dimension mismatches.
type DimensionError struct {
	Expected int
	Got      int
}

func (e *DimensionError) Error() string {
	return fmt.Sprintf("veclite: dimension mismatch: expected %d, got %d", e.Expected, e.Got)
}

func (e *DimensionError) Unwrap() error {
	return ErrDimensionMismatch
}

// NotFoundError provides details about what was not found.
type NotFoundError struct {
	Type string // "record", "collection", etc.
	ID   string // Identifier that was not found
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("veclite: %s not found: %s", e.Type, e.ID)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// StorageError wraps storage-related errors with context.
type StorageError = storage.Error

// Storage-level sentinel errors re-exported for consumer use.
var (
	// ErrFileLocked is returned when the database file is locked by another process.
	ErrFileLocked = storage.ErrFileLocked

	// ErrChecksumMismatch is returned when the file checksum does not match.
	ErrChecksumMismatch = storage.ErrChecksumMismatch

	// ErrCorruptedFile is returned when the database file is corrupted.
	ErrCorruptedFile = storage.ErrCorruptedFile

	// ErrInvalidVersion is returned when the file version is not supported.
	ErrInvalidVersion = storage.ErrInvalidVersion
)
