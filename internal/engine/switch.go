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
	"github.com/xmhrnx/profilemanager/internal/proc"
)

// RunningError is returned when an app that must be closed for a
// switch is still running.
type RunningError struct {
	App       string
	Processes []string
}

func (e *RunningError) Error() string {
	return fmt.Sprintf("%s is running (%s); close it before switching", e.App, strings.Join(e.Processes, ", "))
}

// SwitchOptions tune a switch.
type SwitchOptions struct {
	// QuitRunning asks the app to quit, and waits for it, instead of
	// failing with a RunningError.
	QuitRunning bool
}

// SwitchResult reports what a switch did besides switching.
type SwitchResult struct {
	// SavedAs names a profile created to keep a live sign-in that
	// belonged to no profile, so nothing is ever lost by switching.
	SavedAs string
}

// rollbackName holds the live sign-in from just before the last switch.
const rollbackName = "_rollback"

// Switch signs the app in to a profile.
func (m *Manager) Switch(ctx context.Context, appID, profileID string, opt SwitchOptions) (SwitchResult, error) {
	a, err := m.mustApp(appID)
	if err != nil {
		return SwitchResult{}, err
	}
	m.mu.Lock()
	mode := modeOf(&a, m.state.Apps[appID])
	m.mu.Unlock()
	if mode == adapter.ModeNative {
		return SwitchResult{}, m.switchNative(ctx, &a, profileID)
	}
	if err := m.ensureClosed(&a, opt.QuitRunning); err != nil {
		return SwitchResult{}, err
	}
	var res SwitchResult
	err = m.update(func(st *State) error {
		as := appState(st, appID)
		target, ok := as.Profile(profileID)
		if !ok {
			return errors.New("profile not found")
		}
		if as.Active == profileID && as.Conflict == nil {
			return nil
		}
		live, err := capture.Capture(m.live(&a))
		if err != nil {
			return fmt.Errorf("reading the current sign-in: %w", err)
		}
		if res.SavedAs, err = m.saveBack(&a, as, live); err != nil {
			return err
		}
		if data, err := live.Encode(); err == nil {
			// Best effort: the switch itself undoes failures below.
			m.vault.Put(appID, rollbackName, data)
		}
		snap, err := m.load(appID, target.ID)
		if err != nil {
			return fmt.Errorf("opening %q: %w", target.Name, err)
		}
		if err := capture.Restore(m.live(&a), snap); err != nil {
			if rerr := capture.Restore(m.live(&a), live); rerr != nil {
				return fmt.Errorf("switching failed (%v) and so did restoring the previous sign-in (%v); it is kept in the vault as a rollback copy", err, rerr)
			}
			return fmt.Errorf("switching failed, the previous sign-in was put back: %w", err)
		}
		as.Active = target.ID
		as.Conflict = nil
		as.SyncError = ""
		as.LiveHash = snap.Hash()
		as.LiveFingerprint, _ = capture.Fingerprint(m.live(&a))
		as.LastSync = time.Now()
		return nil
	})
	return res, err
}

// saveBack keeps the live sign-in before it is replaced: into the
// active profile when it changed there, or into a new profile when it
// belongs to none (or to someone else), so a switch never loses one.
func (m *Manager) saveBack(a *adapter.App, as *AppState, live *capture.Snapshot) (string, error) {
	h := live.Hash()
	if live.Empty() {
		return "", nil
	}
	for _, p := range as.Profiles {
		if p.Hash == h {
			return "", nil
		}
	}
	active, hasActive := as.ActiveProfile()
	if hasActive && h == as.LiveHash {
		// Unchanged since it was last written or synced: the vault copy
		// is as new or newer (a command run under the profile may have
		// refreshed it).
		return "", nil
	}
	liveID := capture.Identity(a, live)
	if hasActive && (active.Identity == "" || liveID == "" || liveID == active.Identity) && as.Conflict == nil {
		return "", m.store(a, active, live)
	}
	now := time.Now()
	p := Profile{ID: newID(), Name: uniqueName(as, savedName(liveID, now)), Color: ProfileColors[len(as.Profiles)%len(ProfileColors)], Created: now}
	if err := m.store(a, &p, live); err != nil {
		return "", err
	}
	as.Profiles = append(as.Profiles, p)
	return p.Name, nil
}

func savedName(identity string, t time.Time) string {
	if identity != "" {
		return identity
	}
	return "Saved sign-in " + t.Format("Jan 2 15:04")
}

func uniqueName(as *AppState, name string) string {
	taken := func(n string) bool {
		for _, p := range as.Profiles {
			if strings.EqualFold(p.Name, n) {
				return true
			}
		}
		return false
	}
	if !taken(name) {
		return name
	}
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s (%d)", name, i)
		if !taken(n) {
			return n
		}
	}
}

// ensureClosed checks, and with quit stops, an app that must not run
// during a switch.
func (m *Manager) ensureClosed(a *adapter.App, quit bool) error {
	if !a.CloseBeforeSwitch || len(a.Processes) == 0 {
		return nil
	}
	running, err := proc.Running(m.procs, a.Processes)
	if err != nil {
		return fmt.Errorf("checking whether %s is running: %w", a.Name, err)
	}
	if len(running) == 0 {
		return nil
	}
	if quit {
		left, err := proc.QuitAndWait(m.procs, a.Processes, 15*time.Second)
		if err != nil {
			return err
		}
		if len(left) == 0 {
			// Give the program's last writes a moment to land.
			time.Sleep(500 * time.Millisecond)
			return nil
		}
		running = left
	}
	return &RunningError{App: a.Name, Processes: names(running)}
}

func names(ps []proc.Process) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range ps {
		if !seen[p.Name] {
			seen[p.Name] = true
			out = append(out, p.Name)
		}
	}
	return out
}

func (m *Manager) switchNative(ctx context.Context, a *adapter.App, name string) error {
	d := native.Driver{Spec: a.Native, Run: m.run}
	err := d.Switch(ctx, name)
	m.RefreshNative(ctx, a.ID)
	return err
}

// RefreshNative rereads the profiles of an app in native mode.
func (m *Manager) RefreshNative(ctx context.Context, appID string) error {
	a, err := m.mustApp(appID)
	if err != nil || a.Native == nil {
		return err
	}
	d := native.Driver{Spec: a.Native, Run: m.run}
	list, lerr := d.List(ctx)
	current, cerr := d.Current(ctx)
	return m.update(func(st *State) error {
		as := appState(st, appID)
		if lerr != nil {
			as.NativeError = lerr.Error()
			return lerr
		}
		as.NativeError = ""
		if cerr != nil {
			as.NativeError = cerr.Error()
		}
		var keep []Profile
		for _, p := range as.Profiles {
			if !p.Native {
				keep = append(keep, p)
			}
		}
		for _, n := range list {
			keep = append(keep, Profile{ID: n, Name: n, Native: true})
		}
		as.NativeActive = current
		as.Profiles = keep
		return nil
	})
}
