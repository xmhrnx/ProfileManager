// Package keyring is how the engine reads and writes other programs'
// entries in the OS credential store (the macOS Keychain, Windows
// Credential Manager, the Secret Service on Linux).
//
// The access itself is Giode's to provide: giode.SecureStore only
// reaches this app's own entries so far. Until it can read and write
// entries by service, account and attributes, System is unavailable and
// apps that keep their sign-in in the keychain can't be swapped.
package keyring

import "errors"

// ErrNotFound is returned when an entry doesn't exist.
var ErrNotFound = errors.New("keyring: entry not found")

// ErrUnsupported is returned where no credential store is available.
var ErrUnsupported = errors.New("keyring: no credential store on this system")

// Item identifies an entry.
type Item struct {
	// Service is the Keychain service, the Credential Manager target
	// name, or the Secret Service "service" attribute.
	Service string
	// Account is the Keychain account, the Credential Manager user
	// name, or the Secret Service "username" attribute.
	Account string
	// Attributes, when set, replace service/username as the Secret
	// Service lookup attributes.
	Attributes map[string]string
	// Label names a newly created entry.
	Label string
}

// Keyring is a credential store.
type Keyring interface {
	Get(it Item) ([]byte, error)
	Set(it Item, value []byte) error
	// Delete removes the entry; a missing entry is not an error.
	Delete(it Item) error
}

// System returns the credential store of this computer.
func System() Keyring { return unavailable{} }

// Available reports whether k can reach the credential store.
func Available(k Keyring) bool {
	_, none := k.(unavailable)
	return !none
}

type unavailable struct{}

func (unavailable) Get(Item) ([]byte, error) { return nil, ErrUnsupported }
func (unavailable) Set(Item, []byte) error   { return ErrUnsupported }
func (unavailable) Delete(Item) error        { return ErrUnsupported }

// Memory is an in-memory Keyring for tests.
type Memory struct{ M map[string][]byte }

func memKey(it Item) string { return it.Service + "\x00" + it.Account }

func (m *Memory) Get(it Item) ([]byte, error) {
	v, ok := m.M[memKey(it)]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

func (m *Memory) Set(it Item, value []byte) error {
	if m.M == nil {
		m.M = map[string][]byte{}
	}
	m.M[memKey(it)] = append([]byte(nil), value...)
	return nil
}

func (m *Memory) Delete(it Item) error {
	delete(m.M, memKey(it))
	return nil
}
