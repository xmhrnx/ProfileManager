package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/capture"
	"github.com/xmhrnx/profilemanager/internal/native"
)

// Source says what a new profile starts with.
type Source int

const (
	// FromCurrent saves the sign-in that's live right now.
	FromCurrent Source = iota
	// Blank starts signed out: switching to it clears the live sign-in
	// so the user can sign in to another account, which is then saved
	// into it automatically.
	Blank
)

// ProfileColors are the accents offered for profiles.
var ProfileColors = []string{"#7C5CFF", "#3B82F6", "#14B8A6", "#22C55E", "#F59E0B", "#F43F5E", "#EC4899", "#A855F7"}

// AddProfile creates a profile.
func (m *Manager) AddProfile(ctx context.Context, appID, name, color string, src Source) (Profile, error) {
	a, err := m.mustApp(appID)
	if err != nil {
		return Profile{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, errors.New("give the profile a name")
	}
	m.mu.Lock()
	mode := modeOf(&a, m.state.Apps[appID])
	m.mu.Unlock()
	if mode == adapter.ModeNative {
		return m.addNative(ctx, &a, name)
	}
	var created Profile
	err = m.update(func(st *State) error {
		as := appState(st, appID)
		for _, p := range as.Profiles {
			if strings.EqualFold(p.Name, name) {
				return fmt.Errorf("there's already a profile named %q", p.Name)
			}
		}
		now := time.Now()
		p := Profile{ID: newID(), Name: name, Color: color, Created: now, Updated: now}
		if p.Color == "" {
			p.Color = ProfileColors[len(as.Profiles)%len(ProfileColors)]
		}
		var snap *capture.Snapshot
		if src == FromCurrent {
			if snap, err = capture.Capture(m.live(&a)); err != nil {
				return err
			}
		} else {
			snap = blank(&a, m.vars.GOOS)
		}
		if err := m.store(&a, &p, snap); err != nil {
			return err
		}
		if src == FromCurrent {
			// What's live is this profile now: later changes to it are
			// saved here. A live sign-in that belonged to the previous
			// active profile was already saved there by the last sync.
			as.Active = p.ID
			as.LiveHash = p.Hash
			as.LiveFingerprint, _ = capture.Fingerprint(m.live(&a))
			as.Conflict = nil
		}
		as.Pinned = true
		as.Profiles = append(as.Profiles, p)
		created = p
		return nil
	})
	return created, err
}

func blank(a *adapter.App, goos string) *capture.Snapshot {
	var items, secrets []string
	jsonIDs := map[string]bool{}
	for _, f := range a.Files {
		items = append(items, f.ID)
		if len(f.JSONKeys) > 0 {
			jsonIDs[f.ID] = true
		}
	}
	for _, k := range a.KeychainFor(goos) {
		secrets = append(secrets, k.ID)
	}
	return capture.Blank(a.ID, items, jsonIDs, secrets)
}

func (m *Manager) addNative(ctx context.Context, a *adapter.App, name string) (Profile, error) {
	d := native.Driver{Spec: a.Native, Run: m.run}
	if !d.CanCreate() {
		hint := a.LoginHint
		if hint == "" {
			hint = "Sign in with the program itself, then refresh."
		}
		return Profile{}, fmt.Errorf("%s adds accounts by signing in. %s", a.Name, hint)
	}
	if err := d.Create(ctx, name); err != nil {
		return Profile{}, err
	}
	m.RefreshNative(ctx, a.ID)
	return Profile{ID: name, Name: name, Native: true}, nil
}

// UpdateProfile renames or recolors a profile.
func (m *Manager) UpdateProfile(appID, profileID, name, color string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("give the profile a name")
	}
	return m.update(func(st *State) error {
		as := appState(st, appID)
		p, ok := as.Profile(profileID)
		if !ok || p.Native {
			return errors.New("profile not found")
		}
		for _, o := range as.Profiles {
			if o.ID != profileID && strings.EqualFold(o.Name, name) {
				return fmt.Errorf("there's already a profile named %q", o.Name)
			}
		}
		p.Name = name
		if color != "" {
			p.Color = color
		}
		return nil
	})
}

// DeleteProfile removes a saved profile. Deleting the active one only
// forgets it: the live sign-in stays as it is.
func (m *Manager) DeleteProfile(appID, profileID string) error {
	return m.update(func(st *State) error {
		as := appState(st, appID)
		for i, p := range as.Profiles {
			if p.ID != profileID || p.Native {
				continue
			}
			if err := m.vault.Delete(appID, profileID); err != nil {
				return err
			}
			as.Profiles = append(as.Profiles[:i], as.Profiles[i+1:]...)
			if as.Active == profileID {
				as.Active = ""
				as.Conflict = nil
			}
			return nil
		}
		return errors.New("profile not found")
	})
}

// MoveProfile reorders a profile by delta places.
func (m *Manager) MoveProfile(appID, profileID string, delta int) error {
	return m.update(func(st *State) error {
		as := appState(st, appID)
		for i, p := range as.Profiles {
			if p.ID == profileID {
				j := min(max(i+delta, 0), len(as.Profiles)-1)
				as.Profiles[i], as.Profiles[j] = as.Profiles[j], as.Profiles[i]
				return nil
			}
		}
		return errors.New("profile not found")
	})
}
