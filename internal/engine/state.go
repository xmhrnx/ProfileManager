package engine

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"time"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/fsutil"
)

// State is what Profile Manager remembers between runs. It holds no
// secrets: sign-ins live only in the encrypted vault.
type State struct {
	Version  int                  `json:"version"`
	Settings Settings             `json:"settings"`
	Apps     map[string]*AppState `json:"apps"`
}

// Settings are the user's preferences.
type Settings struct {
	// AutoSync saves sign-in changes (refreshed tokens) back into the
	// active profile in the background.
	AutoSync bool `json:"auto_sync"`
	// IntervalSeconds is how often auto-sync checks.
	IntervalSeconds int `json:"interval_seconds"`
	// SyncKeychain includes keychain entries in background checks. On
	// macOS a check may ask for Keychain access the first time.
	SyncKeychain bool `json:"sync_keychain"`
	// Theme is "dark", "light" or "system".
	Theme string `json:"theme"`
	// CloseToTray keeps the app in the tray when the window closes.
	CloseToTray bool `json:"close_to_tray"`
	// MinimizeToTray hides the window to the tray when it's minimized.
	MinimizeToTray bool `json:"minimize_to_tray"`
}

// DefaultSettings are used on first run.
func DefaultSettings() Settings {
	return Settings{AutoSync: true, IntervalSeconds: 30, SyncKeychain: true, Theme: "dark", CloseToTray: true}
}

// AppState is what's known about one app.
type AppState struct {
	// Pinned apps are listed under "Your apps".
	Pinned bool         `json:"pinned"`
	Mode   adapter.Mode `json:"mode,omitempty"`
	// Active is the ID of the profile currently signed in.
	Active   string    `json:"active,omitempty"`
	Profiles []Profile `json:"profiles,omitempty"`

	// LiveFingerprint and LiveHash describe the live sign-in as of the
	// last switch or sync, to notice changes cheaply.
	LiveFingerprint string    `json:"live_fingerprint,omitempty"`
	LiveHash        string    `json:"live_hash,omitempty"`
	LastSync        time.Time `json:"last_sync,omitzero"`
	SyncError       string    `json:"sync_error,omitempty"`
	// Conflict is set when the live sign-in changed in a way that
	// shouldn't overwrite the active profile silently.
	Conflict *Conflict `json:"conflict,omitempty"`

	// NativeActive is the program's own active profile, in native mode.
	NativeActive string `json:"native_active,omitempty"`
	// NativeError is the last error from the program's own commands.
	NativeError string `json:"native_error,omitempty"`
}

// Profile is one saved account.
type Profile struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
	// Identity is the account name read from the sign-in (an email).
	Identity string `json:"identity,omitempty"`
	// Hash fingerprints the stored snapshot.
	Hash string `json:"hash,omitempty"`
	// Empty is set while the profile holds no sign-in yet.
	Empty   bool      `json:"empty,omitempty"`
	Size    int64     `json:"size,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	// Native profiles belong to the program; Name is its own name.
	Native bool `json:"native,omitempty"`
}

// Conflict kinds.
const (
	// ConflictIdentity: someone else is signed in now.
	ConflictIdentity = "identity"
	// ConflictSignedOut: the sign-in was removed outside the app.
	ConflictSignedOut = "signed_out"
)

// Conflict describes a live sign-in change held back from the vault.
type Conflict struct {
	Kind     string    `json:"kind"`
	Identity string    `json:"identity,omitempty"`
	Detected time.Time `json:"detected"`
}

// Profile returns the saved (not native) profile with id.
func (s *AppState) Profile(id string) (*Profile, bool) {
	for i := range s.Profiles {
		if s.Profiles[i].ID == id && !s.Profiles[i].Native {
			return &s.Profiles[i], true
		}
	}
	return nil, false
}

// ActiveProfile returns the signed-in profile, if any.
func (s *AppState) ActiveProfile() (*Profile, bool) {
	if s.Active == "" {
		return nil, false
	}
	return s.Profile(s.Active)
}

func (s *AppState) clone() AppState {
	c := *s
	c.Profiles = append([]Profile(nil), s.Profiles...)
	if s.Conflict != nil {
		k := *s.Conflict
		c.Conflict = &k
	}
	return c
}

const stateVersion = 1

func loadState(path string) (State, error) {
	st := State{Version: stateVersion, Settings: DefaultSettings(), Apps: map[string]*AppState{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, err
	}
	if st.Apps == nil {
		st.Apps = map[string]*AppState{}
	}
	if st.Settings.IntervalSeconds < 5 {
		st.Settings.IntervalSeconds = DefaultSettings().IntervalSeconds
	}
	return st, nil
}

func saveState(path string, st State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o600)
}
