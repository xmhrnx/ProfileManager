package adapter

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/xmhrnx/profilemanager/internal/fsutil"
)

//go:embed builtin/*.json
var builtinFS embed.FS

// Builtins returns the definitions shipped with Profile Manager.
func Builtins() ([]App, error) {
	entries, err := fs.ReadDir(builtinFS, "builtin")
	if err != nil {
		return nil, err
	}
	var apps []App
	for _, e := range entries {
		data, err := builtinFS.ReadFile("builtin/" + e.Name())
		if err != nil {
			return nil, err
		}
		a, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("built-in %s: %w", e.Name(), err)
		}
		a.Builtin = true
		apps = append(apps, a)
	}
	return apps, nil
}

// Parse reads and validates one definition.
func Parse(data []byte) (App, error) {
	var a App
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return App{}, err
	}
	if err := a.Validate(); err != nil {
		return App{}, err
	}
	return a, nil
}

// Marshal writes a definition the way Parse reads it.
func Marshal(a App) ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

// Registry holds the built-in definitions and the user's own, kept as
// JSON files in a folder. A user file with a built-in's ID replaces it.
type Registry struct {
	dir string

	mu      sync.RWMutex
	builtin map[string]App
	user    map[string]App
	// Problems lists user files that couldn't be read.
	problems []string
}

// NewRegistry loads the built-ins and the user definitions in dir.
func NewRegistry(dir string) (*Registry, error) {
	b, err := Builtins()
	if err != nil {
		return nil, err
	}
	r := &Registry{dir: dir, builtin: map[string]App{}}
	for _, a := range b {
		r.builtin[a.ID] = a
	}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// Reload rereads the user definitions.
func (r *Registry) Reload() error {
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return err
	}
	user := map[string]App{}
	var problems []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		a, err := Parse(data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		user[a.ID] = a
	}
	r.mu.Lock()
	r.user, r.problems = user, problems
	r.mu.Unlock()
	return nil
}

// Problems lists user definition files that couldn't be loaded.
func (r *Registry) Problems() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.problems...)
}

// All returns every definition, sorted by name.
func (r *Registry) All() []App {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]App, 0, len(r.builtin)+len(r.user))
	for id, a := range r.builtin {
		if _, ok := r.user[id]; !ok {
			out = append(out, a.Clone())
		}
	}
	for id, a := range r.user {
		c := a.Clone()
		_, c.Overridden = r.builtin[id]
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// Get returns the definition with id.
func (r *Registry) Get(id string) (App, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if a, ok := r.user[id]; ok {
		c := a.Clone()
		_, c.Overridden = r.builtin[id]
		return c, true
	}
	if a, ok := r.builtin[id]; ok {
		return a.Clone(), true
	}
	return App{}, false
}

// Builtin returns the shipped definition with id, ignoring overrides.
func (r *Registry) Builtin(id string) (App, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.builtin[id]
	return a.Clone(), ok
}

// Save validates a and writes it as a user definition.
func (r *Registry) Save(a App) error {
	if err := a.Validate(); err != nil {
		return err
	}
	data, err := Marshal(a)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(r.dir, a.ID+".json"), data, 0o600); err != nil {
		return err
	}
	a.Builtin = false
	r.mu.Lock()
	r.user[a.ID] = a.Clone()
	r.mu.Unlock()
	return nil
}

// Delete removes the user definition with id. For an overridden
// built-in this restores the shipped definition.
func (r *Registry) Delete(id string) error {
	err := os.Remove(filepath.Join(r.dir, id+".json"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	r.mu.Lock()
	delete(r.user, id)
	r.mu.Unlock()
	return nil
}
