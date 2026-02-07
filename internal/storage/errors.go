package storage

import "fmt"

// Error wraps storage-related errors with context.
type Error struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

func (e *Error) Error() string {
	return fmt.Sprintf("veclite: storage %s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}
