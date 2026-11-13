// Package engine switches programs between saved accounts: it keeps
// profiles in the encrypted vault, swaps them in and out of the live
// locations, saves refreshed sign-ins back as they change, and runs
// single commands under a profile without touching the live sign-in.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/capture"
	"github.com/xmhrnx/profilemanager/internal/keyring"
	"github.com/xmhrnx/profilemanager/internal/lock"
	"github.com/xmhrnx/profilemanager/internal/native"
	"github.com/xmhrnx/profilemanager/internal/proc"
	"github.com/xmhrnx/profilemanager/internal/vault"
)

// Options configure a Manager. Zero fields use the real system.
type Options struct {
	// DataDir holds the state, the vault and user app definitions.
	DataDir string
	// Keys stores the vault's master key.
	Keys    vault.KeySource
	Keyring keyring.Keyring
	Procs   proc.Finder
	Vars    *adapter.Vars
	Runner  native.Runner
}

// Manager is the engine. Its methods are safe for concurrent use, and
// for use alongside other processes (the command line) on the same
// data folder: changes are made under a file lock, on freshly read
// state.
type Manager struct {
	dir     string
	reg     *adapter.Registry
	vault   *vault.Vault
	kr      keyring.Keyring
	procs   proc.Finder
	vars    adapter.Vars
	run     native.Runner
	fileLck *lock.Lock

	mu       sync.Mutex
	state    State
	running  map[string][]string
	onChange func()
}

// New opens the data folder.
func New(o Options) (*Manager, error) {
	if o.DataDir == "" {
		return nil, errors.New("engine: no data folder")
	}
	if o.Keys == nil {
		return nil, errors.New("engine: no key source")
	}
	m := &Manager{
		dir:     o.DataDir,
		kr:      o.Keyring,
		procs:   o.Procs,
		run:     o.Runner,
		fileLck: lock.New(filepath.Join(o.DataDir, "lock")),
		running: map[string][]string{},
	}
	if m.kr == nil {
		m.kr = keyring.System()
	}
	if m.procs == nil {
		m.procs = proc.System()
	}
	if m.run == nil {
		m.run = native.Exec
	}
	if o.Vars != nil {
		m.vars = *o.Vars
	} else {
		m.vars = adapter.SystemVars()
	}
	var err error
	if m.reg, err = adapter.NewRegistry(filepath.Join(o.DataDir, "apps")); err != nil {
		return nil, err
	}
	if m.vault, err = vault.Open(filepath.Join(o.DataDir, "vault"), o.Keys); err != nil {
		return nil, err
	}
	if m.state, err = loadState(m.statePath()); err != nil {
		return nil, fmt.Errorf("reading %s: %w", m.statePath(), err)
	}
	return m, nil
}

func (m *Manager) statePath() string { return filepath.Join(m.dir, "state.json") }

// DataDir returns the data folder.
func (m *Manager) DataDir() string { return m.dir }

// KeyLocation describes where the vault key is kept.
func (m *Manager) KeyLocation() string { return m.vault.KeyLocation() }

// Registry returns the app definitions.
func (m *Manager) Registry() *adapter.Registry { return m.reg }

// GOOS is the system paths are resolved for.
func (m *Manager) GOOS() string { return m.vars.GOOS }

// SetOnChange registers fn, called (from any goroutine) after the
// state changes.
func (m *Manager) SetOnChange(fn func()) {
	m.mu.Lock()
	m.onChange = fn
	m.mu.Unlock()
}

func (m *Manager) changed() {
	m.mu.Lock()
	fn := m.onChange
	m.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// update runs fn on freshly loaded state under the file lock and saves
// the result. fn returning an error still saves what it changed, so
// partial progress (a profile saved before a failed switch) is kept.
func (m *Manager) update(fn func(st *State) error) error {
	if err := m.fileLck.Acquire(); err != nil {
		return err
	}
	defer m.fileLck.Release()
	st, err := loadState(m.statePath())
	if err != nil {
		return err
	}
	ferr := fn(&st)
	if err := saveState(m.statePath(), st); err != nil {
		return errors.Join(ferr, err)
	}
	m.mu.Lock()
	m.state = st
	m.mu.Unlock()
	m.changed()
	return ferr
}

// Reload rereads the state and the user's app definitions, picking up
// changes made by the command line.
func (m *Manager) Reload() error {
	if err := m.reg.Reload(); err != nil {
		return err
	}
	if err := m.fileLck.Acquire(); err != nil {
		return err
	}
	st, err := loadState(m.statePath())
	m.fileLck.Release()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.state = st
	m.mu.Unlock()
	m.changed()
	return nil
}

// AppView is an app's definition with what's known about it.
type AppView struct {
	App   adapter.App
	State AppState
	Mode  adapter.Mode
	// Running lists the app's processes seen by the last check.
	Running []string
}

// Apps lists every app, pinned ones first, then by name.
func (m *Manager) Apps() []AppView {
	apps := m.reg.All()
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]AppView, 0, len(apps))
	for _, a := range apps {
		out = append(out, m.viewLocked(a))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].State.Pinned && !out[j].State.Pinned })
	return out
}

// App returns one app's view.
func (m *Manager) App(id string) (AppView, bool) {
	a, ok := m.reg.Get(id)
	if !ok {
		return AppView{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.viewLocked(a), true
}

func (m *Manager) viewLocked(a adapter.App) AppView {
	v := AppView{App: a, Mode: a.DefaultMode()}
	if st, ok := m.state.Apps[a.ID]; ok {
		v.State = st.clone()
		if st.Mode != "" && a.Supports(st.Mode) {
			v.Mode = st.Mode
		}
	}
	v.Running = append([]string(nil), m.running[a.ID]...)
	return v
}

// Settings returns the preferences.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.Settings
}

// UpdateSettings changes the preferences.
func (m *Manager) UpdateSettings(fn func(*Settings)) error {
	return m.update(func(st *State) error {
		fn(&st.Settings)
		if st.Settings.IntervalSeconds < 5 {
			st.Settings.IntervalSeconds = 5
		}
		return nil
	})
}

func appState(st *State, id string) *AppState {
	a, ok := st.Apps[id]
	if !ok {
		a = &AppState{}
		st.Apps[id] = a
	}
	return a
}

func (m *Manager) mustApp(id string) (adapter.App, error) {
	a, ok := m.reg.Get(id)
	if !ok {
		return adapter.App{}, fmt.Errorf("unknown app %q", id)
	}
	return a, nil
}

func modeOf(a *adapter.App, s *AppState) adapter.Mode {
	if s != nil && s.Mode != "" && a.Supports(s.Mode) {
		return s.Mode
	}
	return a.DefaultMode()
}

// Pin adds or removes an app from "Your apps".
func (m *Manager) Pin(id string, on bool) error {
	if _, err := m.mustApp(id); err != nil {
		return err
	}
	return m.update(func(st *State) error {
		appState(st, id).Pinned = on
		return nil
	})
}

// SetMode picks how an app is switched.
func (m *Manager) SetMode(id string, mode adapter.Mode) error {
	a, err := m.mustApp(id)
	if err != nil {
		return err
	}
	if !a.Supports(mode) {
		return fmt.Errorf("%s can't be switched in %s mode", a.Name, mode)
	}
	err = m.update(func(st *State) error {
		appState(st, id).Mode = mode
		return nil
	})
	if err == nil && mode == adapter.ModeNative {
		go m.RefreshNative(context.Background(), id)
	}
	return err
}

// ErrNeedsKeychain is returned for an app whose sign-in is (partly) in
// the OS keychain when the keychain can't be reached: swapping only its
// files would leave a mismatched sign-in behind.
var ErrNeedsKeychain = errors.New("this app keeps its sign-in in the system keychain on this computer, which Profile Manager can't reach yet")

// NeedsKeychain reports whether a can't be swapped here for want of
// keychain access.
func (m *Manager) NeedsKeychain(a *adapter.App) bool {
	return len(a.KeychainFor(m.vars.GOOS)) > 0 && !keyring.Available(m.kr)
}

func (m *Manager) checkKeychain(a *adapter.App) error {
	if m.NeedsKeychain(a) {
		return fmt.Errorf("%s: %w", a.Name, ErrNeedsKeychain)
	}
	return nil
}

func newID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (m *Manager) live(a *adapter.App) capture.Target {
	return capture.Live(a, m.vars, m.kr)
}

// load returns a profile's snapshot from the vault.
func (m *Manager) load(appID, profileID string) (*capture.Snapshot, error) {
	data, err := m.vault.Get(appID, profileID)
	if err != nil {
		return nil, err
	}
	return capture.Decode(data)
}

// store saves a snapshot into a profile and refreshes its metadata.
func (m *Manager) store(a *adapter.App, p *Profile, s *capture.Snapshot) error {
	data, err := s.Encode()
	if err != nil {
		return err
	}
	if err := m.vault.Put(a.ID, p.ID, data); err != nil {
		return err
	}
	p.Hash = s.Hash()
	p.Empty = s.Empty()
	p.Size = s.Size()
	p.Updated = s.Captured
	if id := capture.Identity(a, s); id != "" {
		p.Identity = id
	} else if p.Empty {
		p.Identity = ""
	}
	return nil
}
