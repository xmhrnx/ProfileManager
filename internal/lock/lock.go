// Package lock serializes changes to the vault between processes: the
// window app, its background sync and command-line runs.
package lock

import (
	"os"
	"path/filepath"
	"sync"
)

// Lock is an exclusive lock on a file, held across processes and, with
// its mutex, across goroutines of this process.
type Lock struct {
	path string
	mu   sync.Mutex
	f    *os.File
}

// New returns a lock on path (created on first use).
func New(path string) *Lock { return &Lock{path: path} }

// Acquire blocks until the lock is held.
func (l *Lock) Acquire() error {
	l.mu.Lock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		l.mu.Unlock()
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		l.mu.Unlock()
		return err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		l.mu.Unlock()
		return err
	}
	l.f = f
	return nil
}

// Release lets the next holder in.
func (l *Lock) Release() {
	if l.f != nil {
		unlockFile(l.f)
		l.f.Close()
		l.f = nil
	}
	l.mu.Unlock()
}
