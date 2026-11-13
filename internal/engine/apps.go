package engine

import (
	"fmt"

	"github.com/xmhrnx/profilemanager/internal/adapter"
)

// SaveApp adds or updates a user app definition. Editing a built-in
// saves a customized copy that replaces it.
func (m *Manager) SaveApp(a adapter.App) error {
	if err := m.reg.Save(a); err != nil {
		return err
	}
	return m.update(func(st *State) error {
		appState(st, a.ID).Pinned = true
		return nil
	})
}

// DeleteApp removes a user definition. A customized built-in goes back
// to the shipped definition and keeps its profiles; a user app's saved
// profiles are deleted with it.
func (m *Manager) DeleteApp(id string) error {
	_, builtin := m.reg.Builtin(id)
	if err := m.reg.Delete(id); err != nil {
		return err
	}
	if builtin {
		m.changed()
		return nil
	}
	return m.update(func(st *State) error {
		if as, ok := st.Apps[id]; ok {
			for _, p := range as.Profiles {
				if !p.Native {
					if err := m.vault.Delete(id, p.ID); err != nil {
						return fmt.Errorf("deleting profile %q: %w", p.Name, err)
					}
				}
			}
			m.vault.Delete(id, rollbackName)
		}
		delete(st.Apps, id)
		return nil
	})
}

// ExportApp returns an app definition as JSON, for sharing.
func (m *Manager) ExportApp(id string) ([]byte, error) {
	a, err := m.mustApp(id)
	if err != nil {
		return nil, err
	}
	return adapter.Marshal(a)
}

// ImportApp reads a JSON definition and saves it.
func (m *Manager) ImportApp(data []byte) (adapter.App, error) {
	a, err := adapter.Parse(data)
	if err != nil {
		return adapter.App{}, err
	}
	return a, m.SaveApp(a)
}

// LivePaths lists where an app's sign-in lives on this system, for
// display: the resolved file paths and keychain entries.
func (m *Manager) LivePaths(a *adapter.App) (files []string, keychain []string) {
	for i := range a.Files {
		if p, ok := a.Files[i].Resolve(m.vars); ok {
			files = append(files, p)
		}
	}
	for _, k := range a.KeychainFor(m.vars.GOOS) {
		k = k.Expanded(m.vars)
		s := k.Service
		if k.Account != "" {
			s += " · " + k.Account
		}
		keychain = append(keychain, s)
	}
	return files, keychain
}
