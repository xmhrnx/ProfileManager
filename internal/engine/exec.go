package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/capture"
)

// Stdio are a command's standard streams.
type Stdio struct {
	In       io.Reader
	Out, Err io.Writer
}

// Exec runs argv signed in to a profile without touching the live
// sign-in: the profile is decrypted into a private temporary folder,
// the program is pointed at it with environment variables, and when it
// exits any change (a refreshed token) is saved back to the vault and
// the folder is wiped. Several profiles can run side by side this way.
func (m *Manager) Exec(ctx context.Context, appID, profileID string, argv []string, stdio Stdio) (int, error) {
	a, err := m.mustApp(appID)
	if err != nil {
		return -1, err
	}
	if !a.SupportsEnv(m.vars.GOOS) {
		return -1, fmt.Errorf("%s can't run under a profile on this system; switch to the profile instead", a.Name)
	}
	if len(argv) == 0 {
		return -1, errors.New("no command given")
	}
	m.mu.Lock()
	as := m.state.Apps[appID]
	var prof Profile
	found := false
	if as != nil {
		if p, ok := as.Profile(profileID); ok {
			prof, found = *p, true
		}
	}
	m.mu.Unlock()
	if !found {
		return -1, errors.New("profile not found")
	}

	runRoot := filepath.Join(m.dir, "run")
	if err := os.MkdirAll(runRoot, 0o700); err != nil {
		return -1, err
	}
	dir, err := os.MkdirTemp(runRoot, appID+"-")
	if err != nil {
		return -1, err
	}
	defer capture.Clear(dir)

	snap, err := m.load(appID, prof.ID)
	if err != nil {
		return -1, err
	}
	target := capture.InDir(&a, m.vars, dir)
	if err := capture.Restore(target, snap); err != nil {
		return -1, err
	}
	before := snap.Hash()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdio.In, stdio.Out, stdio.Err
	cmd.Env = append(os.Environ(), m.profileEnv(&a, prof, dir)...)
	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			return -1, runErr
		}
		code = ee.ExitCode()
	}
	if err := m.saveExec(&a, prof.ID, target, before); err != nil {
		return code, fmt.Errorf("the command finished but its sign-in changes couldn't be saved: %w", err)
	}
	return code, nil
}

func (m *Manager) profileEnv(a *adapter.App, p Profile, dir string) []string {
	v := m.vars.With("profile_dir", dir, "profile", p.Name)
	env := []string{"PROFILE_MANAGER_APP=" + a.ID, "PROFILE_MANAGER_PROFILE=" + p.Name}
	for _, e := range a.EnvFor(m.vars.GOOS) {
		env = append(env, e.Name+"="+v.ExpandPath(e.Value))
	}
	return env
}

// saveExec stores what a command left in its profile folder, and keeps
// the live sign-in in step when that profile is the active one.
func (m *Manager) saveExec(a *adapter.App, profileID string, t capture.Target, before string) error {
	after, err := capture.Capture(t)
	if err != nil {
		return err
	}
	if after.Hash() == before {
		return nil
	}
	return m.update(func(st *State) error {
		as := appState(st, a.ID)
		p, ok := as.Profile(profileID)
		if !ok {
			return nil // deleted meanwhile
		}
		if id := capture.Identity(a, after); id != "" && p.Identity != "" && id != p.Identity {
			// Signed in to someone else inside the command: keep both.
			now := time.Now()
			np := Profile{ID: newID(), Name: uniqueName(as, savedName(id, now)), Color: ProfileColors[len(as.Profiles)%len(ProfileColors)], Created: now}
			if err := m.store(a, &np, after); err != nil {
				return err
			}
			as.Profiles = append(as.Profiles, np)
			return nil
		}
		if err := m.store(a, p, after); err != nil {
			return err
		}
		if as.Active != profileID || (a.CloseBeforeSwitch && m.isRunning(a.ID)) {
			return nil
		}
		// The live copy is this profile: refresh it too, unless it has
		// unsynced changes of its own (the next sync sorts those out).
		live := m.live(a)
		cur, err := capture.Capture(live)
		if err != nil || cur.Hash() != as.LiveHash {
			return nil
		}
		if err := capture.Restore(live, after); err != nil {
			return err
		}
		as.LiveHash = after.Hash()
		as.LiveFingerprint, _ = capture.Fingerprint(live)
		return nil
	})
}

// Shell returns the command for an interactive shell.
func Shell() []string {
	if s := os.Getenv("SHELL"); s != "" {
		return []string{s}
	}
	if c := os.Getenv("COMSPEC"); c != "" {
		return []string{c}
	}
	return []string{"/bin/sh"}
}
