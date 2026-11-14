package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/keyring"
	"github.com/xmhrnx/profilemanager/internal/proc"
	"github.com/xmhrnx/profilemanager/internal/vault"
)

type fakeProcs struct{ running []proc.Process }

func (f *fakeProcs) List() ([]proc.Process, error) { return f.running, nil }
func (f *fakeProcs) Quit(pid int) error {
	var keep []proc.Process
	for _, p := range f.running {
		if p.PID != pid {
			keep = append(keep, p)
		}
	}
	f.running = keep
	return nil
}

type env struct {
	m     *Manager
	home  string
	procs *fakeProcs
	kr    *keyring.Memory
}

func setup(t *testing.T) *env {
	t.Helper()
	home := t.TempDir()
	v := adapter.Vars{GOOS: runtime.GOOS, Values: map[string]string{"home": home, "user": "me"}}
	e := &env{home: home, procs: &fakeProcs{}, kr: &keyring.Memory{}}
	m, err := New(Options{DataDir: filepath.Join(home, "data"), Keys: &vault.MemoryKeySource{}, Keyring: e.kr, Procs: e.procs, Vars: &v})
	if err != nil {
		t.Fatal(err)
	}
	e.m = m
	app := adapter.App{
		ID: "cli", Name: "CLI",
		Files: []adapter.FileSpec{
			{ID: "token", Paths: map[string][]string{"default": {"{home}/.cli/token"}}, EnvPath: "token"},
			{ID: "acct", Paths: map[string][]string{"default": {"{home}/.cli.json"}}, JSONKeys: []string{"user"}, EnvPath: "cli.json"},
		},
		Env:       []adapter.EnvSpec{{Name: "CLI_HOME", Value: "{profile_dir}"}},
		Identity:  &adapter.IdentitySpec{File: "acct", JSON: "user"},
		Processes: []string{"cli-gui"}, CloseBeforeSwitch: true,
	}
	if err := m.SaveApp(app); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) login(t *testing.T, user, token string) {
	t.Helper()
	os.MkdirAll(filepath.Join(e.home, ".cli"), 0o700)
	os.WriteFile(filepath.Join(e.home, ".cli/token"), []byte(token), 0o600)
	os.WriteFile(filepath.Join(e.home, ".cli.json"), []byte(`{"opts":1,"user":"`+user+`"}`), 0o600)
}

func (e *env) token(t *testing.T) string {
	b, err := os.ReadFile(filepath.Join(e.home, ".cli/token"))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSwitchSyncAndSaveBack(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.login(t, "work@x", "W1")
	work, err := e.m.AddProfile(ctx, "cli", "Work", "", FromCurrent)
	if err != nil {
		t.Fatal(err)
	}
	if work.Identity != "work@x" {
		t.Fatalf("identity %q", work.Identity)
	}
	personal, err := e.m.AddProfile(ctx, "cli", "Personal", "", Blank)
	if err != nil {
		t.Fatal(err)
	}

	// Switching to the blank profile signs out.
	if _, err := e.m.Switch(ctx, "cli", personal.ID, SwitchOptions{}); err != nil {
		t.Fatal(err)
	}
	if e.token(t) != "" {
		t.Fatal("blank profile left a token")
	}
	// Signing in there is saved into Personal by the next sync.
	e.login(t, "me@x", "P1")
	if s, err := e.m.Sync("cli", false); err != nil || s != SyncSaved {
		t.Fatalf("sync: %v %v", s, err)
	}
	v, _ := e.m.App("cli")
	if p, _ := v.State.Profile(personal.ID); p.Identity != "me@x" || p.Empty {
		t.Fatalf("personal not updated: %+v", p)
	}

	// The token refreshes; switching away saves it first.
	e.login(t, "me@x", "P2")
	if _, err := e.m.Switch(ctx, "cli", work.ID, SwitchOptions{}); err != nil {
		t.Fatal(err)
	}
	if e.token(t) != "W1" {
		t.Fatalf("work token %q", e.token(t))
	}
	if _, err := e.m.Switch(ctx, "cli", personal.ID, SwitchOptions{}); err != nil {
		t.Fatal(err)
	}
	if e.token(t) != "P2" {
		t.Fatalf("refreshed personal token lost: %q", e.token(t))
	}
	cfg, _ := os.ReadFile(filepath.Join(e.home, ".cli.json"))
	if !strings.Contains(string(cfg), `"opts": 1`) {
		t.Fatalf("other settings lost: %s", cfg)
	}
}

func TestIdentityConflictIsHeldBack(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.login(t, "work@x", "W1")
	work, _ := e.m.AddProfile(ctx, "cli", "Work", "", FromCurrent)

	// Someone signs in to another account outside the app.
	e.login(t, "other@x", "O1")
	if s, _ := e.m.Sync("cli", false); s != SyncConflict {
		t.Fatalf("status %v", s)
	}
	v, _ := e.m.App("cli")
	if v.State.Conflict == nil || v.State.Conflict.Identity != "other@x" {
		t.Fatalf("conflict %+v", v.State.Conflict)
	}
	p, _ := v.State.Profile(work.ID)
	if p.Identity != "work@x" {
		t.Fatal("work profile overwritten")
	}
	// Switching away keeps the stray sign-in as its own profile.
	blank, _ := e.m.AddProfile(ctx, "cli", "Fresh", "", Blank)
	res, err := e.m.Switch(ctx, "cli", blank.ID, SwitchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.SavedAs != "other@x" {
		t.Fatalf("saved as %q", res.SavedAs)
	}
	if _, err := e.m.Switch(ctx, "cli", work.ID, SwitchOptions{}); err != nil {
		t.Fatal(err)
	}
	if e.token(t) != "W1" {
		t.Fatal("work sign-in damaged")
	}
}

func TestUntrackedSignInIsNeverLost(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	blank, _ := e.m.AddProfile(ctx, "cli", "New", "", Blank)
	e.login(t, "", "LOOSE")
	res, err := e.m.Switch(ctx, "cli", blank.ID, SwitchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.SavedAs == "" {
		t.Fatal("untracked sign-in not saved")
	}
	v, _ := e.m.App("cli")
	if len(v.State.Profiles) != 2 {
		t.Fatalf("%d profiles", len(v.State.Profiles))
	}
}

func TestRunningAppBlocksSwitch(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.login(t, "a", "A")
	e.m.AddProfile(ctx, "cli", "A", "", FromCurrent)
	b, _ := e.m.AddProfile(ctx, "cli", "B", "", Blank)
	e.procs.running = []proc.Process{{PID: 42, Name: "cli-gui"}}

	_, err := e.m.Switch(ctx, "cli", b.ID, SwitchOptions{})
	var re *RunningError
	if !errors.As(err, &re) {
		t.Fatalf("expected RunningError, got %v", err)
	}
	if e.token(t) != "A" {
		t.Fatal("files touched while running")
	}
	if s, _ := e.m.Sync("cli", false); s != SyncWaiting {
		t.Fatalf("sync while running: %v", s)
	}
	if _, err := e.m.Switch(ctx, "cli", b.ID, SwitchOptions{QuitRunning: true}); err != nil {
		t.Fatal(err)
	}
	if e.token(t) != "" {
		t.Fatal("switch after quitting didn't happen")
	}
}

func TestExecRunsUnderProfileAndSavesChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	e := setup(t)
	ctx := context.Background()
	e.login(t, "a@x", "A1")
	a, _ := e.m.AddProfile(ctx, "cli", "A", "", FromCurrent)
	e.login(t, "b@x", "B1")
	b, _ := e.m.AddProfile(ctx, "cli", "B", "", FromCurrent)

	var out bytes.Buffer
	code, err := e.m.Exec(ctx, "cli", a.ID, []string{"sh", "-c", `cat "$CLI_HOME/token"; printf A2 > "$CLI_HOME/token"`}, Stdio{Out: &out, Err: &out})
	if err != nil || code != 0 {
		t.Fatalf("exec: %d %v %s", code, err, out.String())
	}
	if out.String() != "A1" {
		t.Fatalf("command saw %q", out.String())
	}
	if e.token(t) != "B1" {
		t.Fatal("live sign-in touched by exec")
	}
	if _, err := e.m.Switch(ctx, "cli", a.ID, SwitchOptions{}); err != nil {
		t.Fatal(err)
	}
	if e.token(t) != "A2" {
		t.Fatalf("token refreshed during exec not kept: %q", e.token(t))
	}
	_ = b
	entries, _ := os.ReadDir(filepath.Join(e.home, "data", "run"))
	if len(entries) != 0 {
		t.Fatal("decrypted run folder left behind")
	}
}

func TestNativeMode(t *testing.T) {
	e := setup(t)
	active := "dev"
	var switched string
	e.m.run = func(ctx context.Context, args []string) (string, error) {
		switch strings.Join(args, " ") {
		case "tool list":
			return "dev\nprod\n", nil
		case "tool current":
			return active + "\n", nil
		}
		if args[1] == "use" {
			switched, active = args[2], args[2]
			return "", nil
		}
		return "", errors.New("unexpected")
	}
	app := adapter.App{ID: "tool", Name: "Tool", Native: &adapter.NativeSpec{
		List: []string{"tool", "list"}, Current: []string{"tool", "current"}, Switch: []string{"tool", "use", "{profile}"}}}
	if err := e.m.SaveApp(app); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := e.m.RefreshNative(ctx, "tool"); err != nil {
		t.Fatal(err)
	}
	v, _ := e.m.App("tool")
	if len(v.State.Profiles) != 2 || v.State.NativeActive != "dev" {
		t.Fatalf("%+v", v.State)
	}
	if _, err := e.m.Switch(ctx, "tool", "prod", SwitchOptions{}); err != nil {
		t.Fatal(err)
	}
	v, _ = e.m.App("tool")
	if switched != "prod" || v.State.NativeActive != "prod" {
		t.Fatalf("switched %q, active %q", switched, v.State.NativeActive)
	}
}

func TestVaultRefusesMissingKey(t *testing.T) {
	e := setup(t)
	e.login(t, "a", "A")
	p, _ := e.m.AddProfile(context.Background(), "cli", "Keep", "", FromCurrent)
	v := e.m.vars
	m2, err := New(Options{DataDir: e.m.dir, Keys: &vault.MemoryKeySource{}, Keyring: e.kr, Procs: e.procs, Vars: &v})
	if err == nil || m2 != nil {
		t.Fatal("a vault opened without its key")
	}
	_ = p
}
