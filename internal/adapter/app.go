// Package adapter describes the software Profile Manager can switch
// accounts for: where each program keeps its sign-in (files, folders,
// keys inside a JSON file, OS keychain entries), how to redirect it with
// environment variables, which processes must be closed first, and the
// commands of programs that keep profiles of their own.
package adapter

import (
	"fmt"
	"regexp"
	"strings"
)

// Mode is how an app's profiles are switched.
type Mode string

const (
	// ModeSwap swaps the files and keychain entries holding the sign-in.
	ModeSwap Mode = "swap"
	// ModeNative calls the program's own profile commands.
	ModeNative Mode = "native"
)

// App is one program Profile Manager can switch accounts for.
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	// Color is the accent of the app's tile, a CSS hex color.
	Color string `json:"color,omitempty"`
	// Glyph is the one or two letters shown on the app's tile.
	Glyph string `json:"glyph,omitempty"`
	// Website points at the program's documentation.
	Website string `json:"website,omitempty"`

	// Files are the files and folders that hold the sign-in.
	Files []FileSpec `json:"files,omitempty"`
	// Keychain are OS credential store entries that hold the sign-in.
	Keychain []KeychainSpec `json:"keychain,omitempty"`
	// Env redirects the program to a per-profile folder, for running
	// a single command (or shell) under a profile without swapping.
	Env []EnvSpec `json:"env,omitempty"`
	// Identity reads the signed-in account's name out of the captured
	// files, to label profiles and catch sign-ins made outside the app.
	Identity *IdentitySpec `json:"identity,omitempty"`

	// Processes are the program's process names (without .exe).
	Processes []string `json:"processes,omitempty"`
	// CloseBeforeSwitch requires the program to be closed before a
	// switch, and holds automatic syncing while it runs: programs
	// keeping databases open (Electron apps) would otherwise be copied
	// half-written, or overwrite the switched files on exit.
	CloseBeforeSwitch bool `json:"close_before_switch,omitempty"`

	// Native describes the program's own profile commands.
	Native *NativeSpec `json:"native,omitempty"`

	// LoginHint tells the user how to sign in to a fresh profile.
	LoginHint string `json:"login_hint,omitempty"`
	// Notes are shown on the app's page.
	Notes string `json:"notes,omitempty"`

	// Builtin is set for definitions shipped with Profile Manager.
	Builtin bool `json:"-"`
	// Overridden is set when a user definition replaces a built-in one.
	Overridden bool `json:"-"`
}

// FileSpec is one file or folder holding (part of) a sign-in.
type FileSpec struct {
	// ID names the item inside snapshots, so a snapshot taken on one
	// system restores on another where the path differs.
	ID string `json:"id"`
	// Paths are candidate locations per OS ("linux", "darwin",
	// "windows", or "default" for any). The first existing candidate is
	// used; when none exists, the first one is created.
	Paths map[string][]string `json:"paths"`
	// JSONKeys, for a JSON file holding other settings too, limits the
	// swap to these top-level keys; everything else in the file stays.
	JSONKeys []string `json:"json_keys,omitempty"`
	// Exclude are glob patterns, relative to a folder, left out of
	// snapshots: caches and logs ("Cache/**", "*.log").
	Exclude []string `json:"exclude,omitempty"`
	// EnvPath is where the item lives inside the per-profile folder
	// when the program is redirected with Env.
	EnvPath string `json:"env_path,omitempty"`
}

// KeychainSpec is one entry in the OS credential store: the macOS
// Keychain, Windows Credential Manager or the Secret Service on Linux.
type KeychainSpec struct {
	ID string `json:"id"`
	// OS limits the entry to these systems; empty means all.
	OS []string `json:"os,omitempty"`
	// Service is the Keychain service, the Credential Manager target
	// name, or the Secret Service "service" attribute.
	Service string `json:"service"`
	// Account is the Keychain account, the Credential Manager user
	// name, or the Secret Service "username" attribute.
	Account string `json:"account,omitempty"`
	// Attributes replace the Secret Service lookup attributes on Linux,
	// for programs that don't use service/username.
	Attributes map[string]string `json:"attributes,omitempty"`
	// Label is the entry's display name when it is created.
	Label string `json:"label,omitempty"`
}

// EnvSpec is an environment variable set when running a command under
// a profile. Value may use {profile_dir} and the path variables.
type EnvSpec struct {
	Name  string   `json:"name"`
	Value string   `json:"value"`
	OS    []string `json:"os,omitempty"`
}

// IdentitySpec extracts the account name from a captured file.
type IdentitySpec struct {
	// File is the FileSpec ID to read; for a folder, Path inside it.
	File string `json:"file"`
	Path string `json:"path,omitempty"`
	// JSON is a dotted path into a JSON document ("oauthAccount.emailAddress").
	JSON string `json:"json,omitempty"`
	// Regex's first group is the identity, for any other format.
	Regex string `json:"regex,omitempty"`
}

// NativeSpec runs the program's own profile commands. Arguments may
// use {profile}, replaced by the profile's name.
type NativeSpec struct {
	// List prints the profiles, one per line unless ListRegex is set.
	List []string `json:"list"`
	// ListRegex's first group, matched repeatedly, is a profile name.
	ListRegex string `json:"list_regex,omitempty"`
	// Current prints the active profile.
	Current []string `json:"current,omitempty"`
	// CurrentRegex's first group is the active profile's name.
	CurrentRegex string `json:"current_regex,omitempty"`
	// Switch activates {profile}.
	Switch []string `json:"switch"`
	// Create makes a new profile named {profile}; empty when profiles
	// can only be added by signing in (see LoginHint).
	Create []string `json:"create,omitempty"`
}

// Modes lists the modes the app supports, preferred first.
func (a *App) Modes() []Mode {
	var m []Mode
	if a.Native != nil {
		m = append(m, ModeNative)
	}
	if len(a.Files) > 0 || len(a.Keychain) > 0 {
		m = append(m, ModeSwap)
	}
	return m
}

// Supports reports whether the app can be switched in mode m.
func (a *App) Supports(m Mode) bool {
	for _, x := range a.Modes() {
		if x == m {
			return true
		}
	}
	return false
}

// DefaultMode is the mode used until the user picks one.
func (a *App) DefaultMode() Mode {
	if m := a.Modes(); len(m) > 0 {
		return m[0]
	}
	return ModeSwap
}

// KeychainFor returns the keychain entries that apply on goos.
func (a *App) KeychainFor(goos string) []KeychainSpec {
	var out []KeychainSpec
	for _, k := range a.Keychain {
		if appliesTo(k.OS, goos) {
			out = append(out, k)
		}
	}
	return out
}

// EnvFor returns the environment variables that apply on goos.
func (a *App) EnvFor(goos string) []EnvSpec {
	var out []EnvSpec
	for _, e := range a.Env {
		if appliesTo(e.OS, goos) {
			out = append(out, e)
		}
	}
	return out
}

// SupportsEnv reports whether a command can run under a profile on
// goos without swapping: every file has a place in the profile folder,
// and nothing lives in the keychain.
func (a *App) SupportsEnv(goos string) bool {
	if len(a.EnvFor(goos)) == 0 || len(a.KeychainFor(goos)) > 0 {
		return false
	}
	for _, f := range a.Files {
		if f.EnvPath == "" {
			return false
		}
	}
	return true
}

func appliesTo(list []string, goos string) bool {
	if len(list) == 0 {
		return true
	}
	for _, s := range list {
		if s == goos {
			return true
		}
	}
	return false
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// Validate reports the first problem with the definition.
func (a *App) Validate() error {
	if !idPattern.MatchString(a.ID) {
		return fmt.Errorf("id %q: use 1-64 lowercase letters, digits, dots, dashes or underscores", a.ID)
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("%s: a name is required", a.ID)
	}
	if len(a.Modes()) == 0 {
		return fmt.Errorf("%s: add at least one file, keychain entry or native command set", a.ID)
	}
	seen := map[string]bool{}
	for _, f := range a.Files {
		if err := validateItemID(seen, f.ID); err != nil {
			return fmt.Errorf("%s: file %w", a.ID, err)
		}
		if len(f.Paths) == 0 {
			return fmt.Errorf("%s: file %q has no paths", a.ID, f.ID)
		}
		for _, list := range f.Paths {
			for _, p := range list {
				if strings.TrimSpace(p) == "" {
					return fmt.Errorf("%s: file %q has an empty path", a.ID, f.ID)
				}
			}
		}
		for _, g := range f.Exclude {
			if _, err := compileGlob(g); err != nil {
				return fmt.Errorf("%s: file %q: %w", a.ID, f.ID, err)
			}
		}
	}
	for _, k := range a.Keychain {
		if err := validateItemID(seen, k.ID); err != nil {
			return fmt.Errorf("%s: keychain %w", a.ID, err)
		}
		if k.Service == "" && len(k.Attributes) == 0 {
			return fmt.Errorf("%s: keychain entry %q needs a service", a.ID, k.ID)
		}
	}
	for _, e := range a.Env {
		if e.Name == "" {
			return fmt.Errorf("%s: an environment variable has no name", a.ID)
		}
	}
	if id := a.Identity; id != nil {
		if id.JSON == "" && id.Regex == "" {
			return fmt.Errorf("%s: identity needs a json path or a regex", a.ID)
		}
		if id.Regex != "" {
			if _, err := regexp.Compile(id.Regex); err != nil {
				return fmt.Errorf("%s: identity regex: %w", a.ID, err)
			}
		}
	}
	if n := a.Native; n != nil {
		if len(n.List) == 0 || len(n.Switch) == 0 {
			return fmt.Errorf("%s: native commands need list and switch", a.ID)
		}
		for _, re := range []string{n.ListRegex, n.CurrentRegex} {
			if re == "" {
				continue
			}
			if _, err := regexp.Compile(re); err != nil {
				return fmt.Errorf("%s: native regex: %w", a.ID, err)
			}
		}
	}
	return nil
}

func validateItemID(seen map[string]bool, id string) error {
	if id == "" {
		return fmt.Errorf("entry without an id")
	}
	if seen[id] {
		return fmt.Errorf("id %q is used twice", id)
	}
	seen[id] = true
	return nil
}

// Clone returns a deep copy, so callers can edit it freely.
func (a App) Clone() App {
	c := a
	c.Files = make([]FileSpec, len(a.Files))
	for i, f := range a.Files {
		f.Paths = cloneMap(f.Paths)
		f.JSONKeys = append([]string(nil), f.JSONKeys...)
		f.Exclude = append([]string(nil), f.Exclude...)
		c.Files[i] = f
	}
	c.Keychain = make([]KeychainSpec, len(a.Keychain))
	for i, k := range a.Keychain {
		k.OS = append([]string(nil), k.OS...)
		if k.Attributes != nil {
			m := make(map[string]string, len(k.Attributes))
			for key, v := range k.Attributes {
				m[key] = v
			}
			k.Attributes = m
		}
		c.Keychain[i] = k
	}
	c.Env = make([]EnvSpec, len(a.Env))
	for i, e := range a.Env {
		e.OS = append([]string(nil), e.OS...)
		c.Env[i] = e
	}
	c.Processes = append([]string(nil), a.Processes...)
	if a.Identity != nil {
		id := *a.Identity
		c.Identity = &id
	}
	if a.Native != nil {
		n := *a.Native
		n.List = append([]string(nil), n.List...)
		n.Current = append([]string(nil), n.Current...)
		n.Switch = append([]string(nil), n.Switch...)
		n.Create = append([]string(nil), n.Create...)
		c.Native = &n
	}
	return c
}

func cloneMap(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}
