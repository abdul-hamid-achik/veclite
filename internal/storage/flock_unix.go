//go:build !windows

package storage

import (
	"os"
	"syscall"
)

// lockFile acquires an exclusive lock on the given file.
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// lockFileShared acquires a shared (read) lock on the given file.
// Multiple processes can hold a shared lock simultaneously, but no process
// can hold an exclusive lock while any shared lock is held.
func lockFileShared(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
}

// unlockFile releases the lock on the given file.
// It works for both shared and exclusive locks.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// IsProcessAlive returns true if a process with the given PID is currently running.
// Uses kill(pid, 0) which sends no signal but checks process existence.
func IsProcessAlive(pid int) bool {
	return isProcessAlive(pid)
}

// isProcessAlive returns true if a process with the given PID is currently running.
// Uses kill(pid, 0) which sends no signal but checks process existence.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
