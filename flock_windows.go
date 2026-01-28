//go:build windows

package veclite

import "os"

// lockFile acquires an exclusive lock on the given file.
// On Windows, this is a no-op (file locking is not yet supported).
func lockFile(f *os.File) error {
	return nil
}

// unlockFile releases the lock on the given file.
// On Windows, this is a no-op.
func unlockFile(f *os.File) error {
	return nil
}
