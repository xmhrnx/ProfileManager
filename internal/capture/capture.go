package capture

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/keyring"
)

// MaxSize caps a snapshot, so a misconfigured folder (a whole home
// directory) fails loudly instead of filling the disk.
const MaxSize = 1 << 30

// ErrTooLarge is returned for a capture over MaxSize.
var ErrTooLarge = errors.New("capture: the sign-in data is over 1 GB; add exclude patterns for caches")

// Target is where an app's sign-in lives: its live locations, or a
// per-profile folder when a command runs under a profile.
type Target struct {
	App  *adapter.App
	Vars adapter.Vars
	// Keyring holds the keychain entries; nil skips them.
	Keyring keyring.Keyring
	// Locate maps a file to its path; ok is false when the file has no
	// place on this system.
	Locate func(f *adapter.FileSpec) (path string, ok bool)
}

// Live targets the app's real files and keychain entries.
func Live(app *adapter.App, vars adapter.Vars, kr keyring.Keyring) Target {
	return Target{App: app, Vars: vars, Keyring: kr, Locate: func(f *adapter.FileSpec) (string, bool) {
		return f.Resolve(vars)
	}}
}

// InDir targets a per-profile folder, laid out by each file's EnvPath.
func InDir(app *adapter.App, vars adapter.Vars, dir string) Target {
	return Target{App: app, Vars: vars, Locate: func(f *adapter.FileSpec) (string, bool) {
		if f.EnvPath == "" {
			return "", false
		}
		return filepath.Join(dir, filepath.FromSlash(f.EnvPath)), true
	}}
}

func (t Target) secrets() []adapter.KeychainSpec {
	if t.Keyring == nil {
		return nil
	}
	return t.App.KeychainFor(t.Vars.GOOS)
}

// Capture reads the sign-in into a snapshot.
func Capture(t Target) (*Snapshot, error) {
	s := &Snapshot{Version: snapshotVersion, App: t.App.ID, Captured: time.Now()}
	var total int64
	for i := range t.App.Files {
		f := &t.App.Files[i]
		path, ok := t.Locate(f)
		if !ok {
			continue
		}
		it, err := captureItem(f, path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		s.Items = append(s.Items, it)
		total += int64(len(it.Data))
		for _, e := range it.Entries {
			total += int64(len(e.Data))
		}
		if total > MaxSize {
			return nil, ErrTooLarge
		}
	}
	for _, k := range t.secrets() {
		sec := Secret{ID: k.ID}
		v, err := t.Keyring.Get(item(k, t.Vars))
		switch {
		case err == nil:
			sec.Present, sec.Value = true, v
		case errors.Is(err, keyring.ErrNotFound):
		case errors.Is(err, keyring.ErrUnsupported):
			sec.Unavailable = true
		default:
			return nil, fmt.Errorf("keychain %q: %w", k.Service, err)
		}
		s.Secrets = append(s.Secrets, sec)
	}
	return s, nil
}

func item(k adapter.KeychainSpec, v adapter.Vars) keyring.Item {
	k = k.Expanded(v)
	return keyring.Item{Service: k.Service, Account: k.Account, Attributes: k.Attributes, Label: k.Label}
}

func captureItem(f *adapter.FileSpec, path string) (Item, error) {
	it := Item{ID: f.ID}
	info, err := os.Stat(path) // follows symlinks: dotfile managers link these
	if errors.Is(err, fs.ErrNotExist) {
		it.Kind = KindAbsent
		if len(f.JSONKeys) > 0 {
			it.Kind = KindJSON
		}
		return it, nil
	}
	if err != nil {
		return it, err
	}
	it.Mode = uint32(info.Mode().Perm())
	if info.IsDir() {
		it.Kind = KindDir
		it.Entries, err = captureDir(path, adapter.NewExcluder(f.Exclude))
		return it, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return it, err
	}
	if len(f.JSONKeys) == 0 {
		it.Kind, it.Data = KindFile, data
		return it, nil
	}
	obj, err := parseObject(data)
	if err != nil {
		// A half-written file (the program is saving it right now)
		// must not be captured as "signed out".
		return it, fmt.Errorf("reading JSON: %w", err)
	}
	it.Kind = KindJSON
	for _, k := range f.JSONKeys {
		if v, ok := obj.vals[k]; ok {
			if it.Keys == nil {
				it.Keys = map[string]json.RawMessage{}
			}
			it.Keys[k] = compact(v)
		}
	}
	return it, nil
}

func captureDir(root string, ex adapter.Excluder) ([]Entry, error) {
	var out []Entry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Files come and go while a program runs (lock files, temp
			// files); a vanished one is simply not part of the copy.
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if ex.Excluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		e := Entry{Path: rel, Mode: uint32(info.Mode().Perm())}
		switch {
		case d.IsDir():
			e.Dir = true
		case info.Mode()&fs.ModeSymlink != 0:
			if e.Link, err = os.Readlink(p); err != nil {
				return nil
			}
		case info.Mode().IsRegular():
			data, err := os.ReadFile(p)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			e.Data = data
		default:
			return nil // sockets, pipes and devices aren't copied
		}
		out = append(out, e)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}
