// Package lock provides a cross-process advisory file lock guarding concurrent
// agentsmith runs against a single hub (plan 4: ~/.agentsmith/.lock).
package lock

import (
	"fmt"
	"os"
	"syscall"
)

// Lock is a held file lock. Release it with Unlock.
type Lock struct {
	f *os.File
}

// Acquire takes an exclusive advisory lock on path, blocking until available.
// The lock file is created if missing with 0600 perms.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquire lock %s: %w", path, err)
	}
	return &Lock{f: f}, nil
}

// Unlock releases the lock and closes the underlying file.
func (l *Lock) Unlock() error {
	if l == nil || l.f == nil {
		return nil
	}
	ferr := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	cerr := l.f.Close()
	l.f = nil
	if ferr != nil {
		return ferr
	}
	return cerr
}
