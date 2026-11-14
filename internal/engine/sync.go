package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/capture"
	"github.com/xmhrnx/profilemanager/internal/proc"
)

// SyncStatus is what a sync did.
type SyncStatus int

const (
	SyncUnchanged SyncStatus = iota
	SyncSaved
	// SyncWaiting: the app is running and must close before its files
	// can be copied consistently.
	SyncWaiting
	// SyncConflict: the change was held back for the user to decide.
	SyncConflict
	// SyncSkipped: nothing to sync (no active profile, native mode).
	SyncSkipped
)

// Sync saves changes to the live sign-in (a refreshed token, a renewed
// session) into the active profile, keeping the vault up to date. A
// different account or a sign-out is held back as a Conflict instead.
// force reads keychain entries even when the files are unchanged.
func (m *Manager) Sync(appID string, force bool) (SyncStatus, error) {
	a, err := m.mustApp(appID)
	if err != nil {
		return SyncSkipped, err
	}
	m.mu.Lock()
	as0, ok := m.state.Apps[appID]
	skip := !ok || as0.Active == "" || modeOf(&a, as0) != adapter.ModeSwap
	syncKeychain := m.state.Settings.SyncKeychain
	m.mu.Unlock()
	if skip {
		return SyncSkipped, nil
	}
	if a.CloseBeforeSwitch && len(a.Processes) > 0 {
		running, err := proc.Running(m.procs, a.Processes)
		m.setRunning(appID, names(running))
		if err == nil && len(running) > 0 {
			return SyncWaiting, nil
		}
	}
	status := SyncUnchanged
	err = m.update(func(st *State) error {
		as := appState(st, appID)
		active, ok := as.ActiveProfile()
		if !ok {
			status = SyncSkipped
			return nil
		}
		t := m.live(&a)
		fp, err := capture.Fingerprint(t)
		if err != nil {
			as.SyncError = err.Error()
			return err
		}
		hasSecrets := len(a.KeychainFor(m.vars.GOOS)) > 0
		if fp == as.LiveFingerprint && !force && !(hasSecrets && syncKeychain) {
			return nil
		}
		live, err := capture.Capture(t)
		if err != nil {
			as.SyncError = err.Error()
			return err
		}
		as.SyncError = ""
		as.LiveFingerprint = fp
		as.LastSync = time.Now()
		h := live.Hash()
		if h == as.LiveHash {
			return nil
		}
		liveID := capture.Identity(&a, live)
		switch {
		case live.Empty() && !active.Empty:
			as.Conflict = &Conflict{Kind: ConflictSignedOut, Detected: time.Now()}
			status = SyncConflict
			return nil
		case liveID != "" && active.Identity != "" && liveID != active.Identity:
			as.Conflict = &Conflict{Kind: ConflictIdentity, Identity: liveID, Detected: time.Now()}
			status = SyncConflict
			return nil
		}
		if err := m.store(&a, active, live); err != nil {
			as.SyncError = err.Error()
			return err
		}
		as.LiveHash = h
		as.Conflict = nil
		status = SyncSaved
		return nil
	})
	return status, err
}

// Resolution is the user's answer to a Conflict.
type Resolution int

const (
	// KeepNew saves the live sign-in into the active profile.
	KeepNew Resolution = iota
	// SaveAsNew saves the live sign-in as a new profile, which becomes
	// active; the old profile keeps its sign-in.
	SaveAsNew
	// RestoreProfile puts the active profile's sign-in back.
	RestoreProfile
)

// Resolve settles a conflict. name is the new profile's name for
// SaveAsNew.
func (m *Manager) Resolve(ctx context.Context, appID string, r Resolution, name string) error {
	a, err := m.mustApp(appID)
	if err != nil {
		return err
	}
	switch r {
	case SaveAsNew:
		_, err := m.AddProfile(ctx, appID, name, "", FromCurrent)
		return err
	case RestoreProfile:
		if err := m.ensureClosed(&a, false); err != nil {
			return err
		}
		return m.update(func(st *State) error {
			as := appState(st, appID)
			active, ok := as.ActiveProfile()
			if !ok {
				return errors.New("no active profile")
			}
			snap, err := m.load(appID, active.ID)
			if err != nil {
				return err
			}
			if err := capture.Restore(m.live(&a), snap); err != nil {
				return err
			}
			as.Conflict = nil
			as.LiveHash = snap.Hash()
			as.LiveFingerprint, _ = capture.Fingerprint(m.live(&a))
			return nil
		})
	default:
		return m.update(func(st *State) error {
			as := appState(st, appID)
			active, ok := as.ActiveProfile()
			if !ok {
				return errors.New("no active profile")
			}
			live, err := capture.Capture(m.live(&a))
			if err != nil {
				return err
			}
			if err := m.store(&a, active, live); err != nil {
				return err
			}
			as.Conflict = nil
			as.LiveHash = live.Hash()
			as.LiveFingerprint, _ = capture.Fingerprint(m.live(&a))
			return nil
		})
	}
}

// SyncAll syncs every app with an active profile and returns the
// errors by app ID.
func (m *Manager) SyncAll(force bool) map[string]error {
	errs := map[string]error{}
	m.mu.Lock()
	var ids []string
	for id, as := range m.state.Apps {
		if as.Active != "" {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		if _, err := m.Sync(id, force); err != nil {
			errs[id] = err
		}
	}
	return errs
}

func (m *Manager) setRunning(appID string, procs []string) {
	m.mu.Lock()
	prev := m.running[appID]
	m.running[appID] = procs
	m.mu.Unlock()
	if fmt.Sprint(prev) != fmt.Sprint(procs) {
		m.changed()
	}
}

func (m *Manager) isRunning(appID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.running[appID]) > 0
}

// CheckRunning refreshes which pinned apps that must be closed for a
// switch are running.
func (m *Manager) CheckRunning() {
	all, err := m.procs.List()
	if err != nil {
		return
	}
	for _, v := range m.Apps() {
		if !v.State.Pinned || len(v.App.Processes) == 0 {
			continue
		}
		var hit []proc.Process
		for _, p := range all {
			if proc.Matches(p.Name, v.App.Processes) {
				hit = append(hit, p)
			}
		}
		m.setRunning(v.App.ID, names(hit))
	}
}

// QuitApp asks a running app to close and waits for it.
func (m *Manager) QuitApp(appID string) error {
	a, err := m.mustApp(appID)
	if err != nil {
		return err
	}
	left, err := proc.QuitAndWait(m.procs, a.Processes, 15*time.Second)
	m.setRunning(appID, names(left))
	if err != nil {
		return err
	}
	if len(left) > 0 {
		return &RunningError{App: a.Name, Processes: names(left)}
	}
	return nil
}

// AutoSync keeps the vault up to date until ctx ends: it reloads state
// changed by other processes, saves refreshed sign-ins, notices running
// apps and rereads native profiles now and then. A final sync runs on
// the way out.
func (m *Manager) AutoSync(ctx context.Context, onError func(appID string, err error)) {
	tick := 0
	for {
		s := m.Settings()
		wait := time.Duration(s.IntervalSeconds) * time.Second
		select {
		case <-ctx.Done():
			m.SyncAll(false)
			return
		case <-time.After(wait):
		}
		tick++
		m.Reload()
		m.CheckRunning()
		if s.AutoSync {
			for id, err := range m.SyncAll(false) {
				if onError != nil {
					onError(id, err)
				}
			}
		}
		if tick%4 == 0 {
			m.refreshPinnedNative(ctx)
		}
	}
}

func (m *Manager) refreshPinnedNative(ctx context.Context) {
	for _, v := range m.Apps() {
		if v.State.Pinned && v.Mode == adapter.ModeNative {
			m.RefreshNative(ctx, v.App.ID)
		}
	}
}
