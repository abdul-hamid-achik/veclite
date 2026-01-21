package hnsw

import (
	"errors"
	"fmt"
)

var (
	// ErrEmptyVector is returned when an empty vector is provided.
	ErrEmptyVector = errors.New("vector cannot be empty")

	// ErrEmptyIndex is returned when searching an empty index.
	ErrEmptyIndex = errors.New("index is empty")

	// ErrNotFound is returned when a node is not found.
	ErrNotFound = errors.New("node not found")
)

// DimensionError is returned when vector dimensions don't match.
type DimensionError struct {
	Expected int
	Got      int
}

func (e *DimensionError) Error() string {
	return fmt.Sprintf("dimension mismatch: expected %d, got %d", e.Expected, e.Got)
}

// DuplicateError is returned when inserting a duplicate ID.
type DuplicateError struct {
	ID uint64
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("node with ID %d already exists", e.ID)
}
