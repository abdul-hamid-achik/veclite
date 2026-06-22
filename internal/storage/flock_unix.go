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

// unlockFile releases the lock on the given file.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
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