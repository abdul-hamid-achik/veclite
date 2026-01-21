package veclite

import (
	"errors"
	"fmt"
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

	// ErrCorruptedFile is returned when the database file is corrupted.
	ErrCorruptedFile = errors.New("veclite: corrupted file")

	// ErrInvalidVersion is returned when the file version is not supported.
	ErrInvalidVersion = errors.New("veclite: unsupported file version")

	// ErrBatchSizeMismatch is returned when batch operation input sizes don't match.
	ErrBatchSizeMismatch = errors.New("veclite: batch size mismatch")
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
type StorageError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("veclite: storage %s: %v", e.Op, e.Err)
}

func (e *StorageError) Unwrap() error {
	return e.Err
}
