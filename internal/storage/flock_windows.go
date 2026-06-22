//go:build windows

package storage

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock   = 0x00000002
	lockfileFailImmediately = 0x00000001
)

// lockFile acquires an exclusive, non-blocking lock on the given file using LockFileEx.
func lockFile(f *os.File) error {
	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		uintptr(f.Fd()),
		uintptr(lockfileExclusiveLock|lockfileFailImmediately),
		0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// unlockFile releases the lock on the given file using UnlockFileEx.
func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	r1, _, err := procUnlockFileEx.Call(
		uintptr(f.Fd()), 0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// isProcessAlive returns true if a process with the given PID is currently running.
// On Windows, stale lock detection is not supported via kill(pid, 0); we conservatively
// return true so the retry path is used instead of the stale-lock-clear path.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return true
}