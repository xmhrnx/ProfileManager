package vault

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/xmhrnx/profilemanager/internal/fsutil"
)

// FuncKeySource adapts a pair of functions (an OS credential store) to
// a KeySource. Get must return an error matching notFound for a key
// that was never stored.
type FuncKeySource struct {
	Label    string
	Get      func() ([]byte, error)
	Set      func([]byte) error
	NotFound error
}

func (f FuncKeySource) Load() ([]byte, error) {
	v, err := f.Get()
	if f.NotFound != nil && errors.Is(err, f.NotFound) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, err
	}
	return decodeKey(v)
}

func (f FuncKeySource) Store(key []byte) error { return f.Set(encodeKey(key)) }
func (f FuncKeySource) Name() string           { return f.Label }

// FileKeySource keeps the key in a file readable only by this user.
// It is the fallback for systems without a credential store (a Linux
// server with no Secret Service); the UI warns while it's in use.
type FileKeySource struct{ Path string }

func (f FileKeySource) Load() ([]byte, error) {
	v, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, err
	}
	return decodeKey(v)
}

func (f FileKeySource) Store(key []byte) error {
	return fsutil.WriteFileAtomic(f.Path, encodeKey(key), 0o600)
}

func (f FileKeySource) Name() string { return "a key file (no OS credential store found)" }

// MemoryKeySource holds the key in memory, for tests.
type MemoryKeySource struct{ Key []byte }

func (m *MemoryKeySource) Load() ([]byte, error) {
	if m.Key == nil {
		return nil, ErrNoKey
	}
	return m.Key, nil
}
func (m *MemoryKeySource) Store(key []byte) error { m.Key = append([]byte(nil), key...); return nil }
func (m *MemoryKeySource) Name() string           { return "memory" }

// Keys are stored base64-encoded: some stores only take text.
func encodeKey(k []byte) []byte { return []byte(base64.StdEncoding.EncodeToString(k)) }

func decodeKey(v []byte) ([]byte, error) {
	k, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(v)))
	if err != nil || len(k) != keySize {
		return nil, ErrWrongKey
	}
	return k, nil
}
