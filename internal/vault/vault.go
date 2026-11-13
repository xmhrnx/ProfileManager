// Package vault keeps profile snapshots encrypted at rest.
//
// Every snapshot is sealed with AES-256-GCM under a random 256-bit
// master key. The master key lives in the operating system's credential
// store (Keychain, Credential Manager/DPAPI, Secret Service), never next
// to the vault, so copying the vault folder off the computer yields
// nothing readable. Each file is bound to its app and profile through
// the GCM associated data, so files can't be swapped around unnoticed.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/xmhrnx/profilemanager/internal/fsutil"
)

var magic = []byte("PMV1")

const keySize = 32

var (
	// ErrNoKey is returned by a KeySource holding no key yet.
	ErrNoKey = errors.New("vault: no master key stored")
	// ErrKeyLost means the vault has data but its key is gone.
	ErrKeyLost = errors.New("vault: the master key is missing from the credential store, so saved profiles can't be decrypted")
	// ErrWrongKey means the stored key doesn't open the vault.
	ErrWrongKey = errors.New("vault: the stored master key doesn't match this vault")
	// ErrNotFound is returned for a snapshot that doesn't exist.
	ErrNotFound = errors.New("vault: snapshot not found")
	// ErrTampered is returned for a file that fails authentication.
	ErrTampered = errors.New("vault: snapshot failed authentication (corrupted or modified)")
)

// KeySource stores the master key.
type KeySource interface {
	// Load returns the key, or ErrNoKey.
	Load() ([]byte, error)
	Store(key []byte) error
	// Name describes where the key lives, for the UI.
	Name() string
}

// Vault is a folder of sealed snapshots.
type Vault struct {
	dir    string
	aead   cipher.AEAD
	source string
}

const checkFile = "vault.check"

var checkPlain = []byte("profile-manager-vault")

// Open opens the vault in dir with the key from ks, creating the key on
// first use. It refuses to create a new key for a vault that already
// holds data, so a missing key never silently orphans saved profiles.
func Open(dir string, ks KeySource) (*Vault, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	checkPath := filepath.Join(dir, checkFile)
	check, err := os.ReadFile(checkPath)
	hasCheck := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	key, err := ks.Load()
	switch {
	case errors.Is(err, ErrNoKey) && hasCheck:
		return nil, ErrKeyLost
	case errors.Is(err, ErrNoKey):
		key = make([]byte, keySize)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := ks.Store(key); err != nil {
			return nil, fmt.Errorf("vault: saving the master key: %w", err)
		}
		// Read it back: a store that silently drops writes must not
		// leave a vault nobody can open.
		again, err := ks.Load()
		if err != nil || !bytes.Equal(again, key) {
			return nil, fmt.Errorf("vault: the credential store didn't keep the master key: %v", err)
		}
	case err != nil:
		return nil, fmt.Errorf("vault: reading the master key: %w", err)
	}
	if len(key) != keySize {
		return nil, ErrWrongKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	v := &Vault{dir: dir, aead: aead, source: ks.Name()}
	if hasCheck {
		got, err := v.open(check, checkFile)
		if err != nil || !bytes.Equal(got, checkPlain) {
			return nil, ErrWrongKey
		}
	} else if err := fsutil.WriteFileAtomic(checkPath, v.seal(checkPlain, checkFile), 0o600); err != nil {
		return nil, err
	}
	return v, nil
}

// KeyLocation describes where the master key is kept.
func (v *Vault) KeyLocation() string { return v.source }

func (v *Vault) seal(plain []byte, aad string) []byte {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err) // crypto/rand doesn't fail on supported systems
	}
	out := append(append([]byte{}, magic...), nonce...)
	return v.aead.Seal(out, nonce, plain, append(append([]byte{}, magic...), aad...))
}

func (v *Vault) open(data []byte, aad string) ([]byte, error) {
	ns := v.aead.NonceSize()
	if len(data) < len(magic)+ns || !bytes.Equal(data[:len(magic)], magic) {
		return nil, ErrTampered
	}
	nonce := data[len(magic) : len(magic)+ns]
	plain, err := v.aead.Open(nil, nonce, data[len(magic)+ns:], append(append([]byte{}, magic...), aad...))
	if err != nil {
		return nil, ErrTampered
	}
	return plain, nil
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func (v *Vault) path(app, name string) (string, error) {
	if !safeName.MatchString(app) || !safeName.MatchString(name) || app[0] == '.' || name[0] == '.' {
		return "", fmt.Errorf("vault: invalid name %q/%q", app, name)
	}
	return filepath.Join(v.dir, app, name+".pmv"), nil
}

// Put seals data and stores it as app/name, replacing any older copy
// atomically.
func (v *Vault) Put(app, name string, data []byte) error {
	p, err := v.path(app, name)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, v.seal(data, app+"/"+name), 0o600)
}

// Get returns the data stored as app/name.
func (v *Vault) Get(app, name string) ([]byte, error) {
	p, err := v.path(app, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return v.open(data, app+"/"+name)
}

// Has reports whether app/name exists.
func (v *Vault) Has(app, name string) bool {
	p, err := v.path(app, name)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Delete removes app/name; a missing one is not an error.
func (v *Vault) Delete(app, name string) error {
	p, err := v.path(app, name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
