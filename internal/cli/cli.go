// Package cli is Profile Manager's command line: list, switch, sync,
// and run commands or shells signed in to a profile.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// Commands are the first arguments the command line handles.
var Commands = map[string]bool{
	"list": true, "ls": true, "switch": true, "use": true, "sync": true,
	"exec": true, "run": true, "shell": true, "status": true, "export": true,
	"import": true, "add": true, "remove": true, "rm": true, "help": true, "-h": true, "--help": true, "version": true,
}

// IsCommand reports whether args start with a command-line command.
func IsCommand(args []string) bool { return len(args) > 0 && Commands[args[0]] }

// Run executes a command and returns the process exit code. open is
// called lazily, so "help" works without touching the vault.
func Run(args []string, open func() (*engine.Manager, error), stdout, stderr io.Writer) int {
	c := &cmd{out: stdout, err: stderr, open: open}
	if err := c.run(args); err != nil {
		var ec exitCode
		if errors.As(err, &ec) {
			return int(ec)
		}
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

type cmd struct {
	out, err io.Writer
	open     func() (*engine.Manager, error)
	m        *engine.Manager
}

const usage = `Profile Manager - switch sign-ins between accounts

Usage:
  profilemanager                          open the app
  profilemanager list [app]               list apps with profiles, or one app's profiles
  profilemanager switch <app> <profile>   sign the app in to a profile
        --quit                            close the app first if it must not be running
  profilemanager add <app> <name> [--blank]
                                          save the current sign-in as a profile, or
                                          with --blank create an empty one to sign in to
  profilemanager remove <app> <profile>   delete a saved profile
  profilemanager sync [app]               save refreshed sign-ins into the active profiles
  profilemanager exec <app> <profile> -- <command> [args...]
                                          run one command signed in to a profile,
                                          leaving the current sign-in alone
  profilemanager shell <app> <profile>    open a shell signed in to a profile
  profilemanager status                   where data and the vault key are kept
  profilemanager export <app>             print an app definition as JSON
  profilemanager import <file.json>       add an app definition

Profiles can be named by name or ID. Apps by ID (see "list").
`

func (c *cmd) manager() (*engine.Manager, error) {
	if c.m == nil {
		m, err := c.open()
		if err != nil {
			return nil, err
		}
		c.m = m
	}
	return c.m, nil
}

func (c *cmd) run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(c.out, usage)
		return nil
	}
	rest := args[1:]
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(c.out, usage)
		return nil
	case "list", "ls":
		return c.list(rest)
	case "switch", "use":
		return c.switchCmd(rest)
	case "add":
		return c.add(rest)
	case "remove", "rm":
		return c.remove(rest)
	case "sync":
		return c.sync(rest)
	case "exec", "run":
		return c.exec(rest)
	case "shell":
		return c.shell(rest)
	case "status":
		return c.status()
	case "export":
		return c.export(rest)
	case "import":
		return c.importCmd(rest)
	case "version":
		fmt.Fprintln(c.out, "profilemanager", Version)
		return nil
	}
	return fmt.Errorf("unknown command %q; see profilemanager help", args[0])
}

// Version is set at build time.
var Version = "dev"

func (c *cmd) list(args []string) error {
	m, err := c.manager()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(c.out, 0, 4, 2, ' ', 0)
	defer tw.Flush()
	if len(args) == 0 {
		fmt.Fprintln(tw, "APP\tNAME\tMODE\tACTIVE\tPROFILES")
		for _, v := range m.Apps() {
			if !v.State.Pinned && len(v.State.Profiles) == 0 {
				continue
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n", v.App.ID, v.App.Name, v.Mode, activeName(v), countProfiles(v))
		}
		fmt.Fprintln(tw, "\nOther supported apps:")
		var other []string
		for _, v := range m.Apps() {
			if !v.State.Pinned && len(v.State.Profiles) == 0 {
				other = append(other, v.App.ID)
			}
		}
		fmt.Fprintln(tw, "  "+strings.Join(other, ", "))
		return nil
	}
	v, err := c.app(m, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(tw, " \tNAME\tACCOUNT\tUPDATED\tID")
	for _, p := range profilesFor(v) {
		mark := " "
		if isActive(v, p) {
			mark = "*"
		}
		updated := "-"
		if !p.Updated.IsZero() {
			updated = p.Updated.Format(time.DateTime)
		}
		acct := p.Identity
		if p.Empty {
			acct = "(not signed in yet)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", mark, p.Name, acct, updated, p.ID)
	}
	return nil
}

func countProfiles(v engine.AppView) int { return len(profilesFor(v)) }

// profilesFor returns the profiles of the app's current mode.
func profilesFor(v engine.AppView) []engine.Profile {
	var out []engine.Profile
	for _, p := range v.State.Profiles {
		if p.Native == (v.Mode == adapter.ModeNative) {
			out = append(out, p)
		}
	}
	return out
}

func isActive(v engine.AppView, p engine.Profile) bool {
	if v.Mode == adapter.ModeNative {
		return p.ID == v.State.NativeActive
	}
	return p.ID == v.State.Active
}

func activeName(v engine.AppView) string {
	for _, p := range profilesFor(v) {
		if isActive(v, p) {
			return p.Name
		}
	}
	return "-"
}

func (c *cmd) app(m *engine.Manager, id string) (engine.AppView, error) {
	v, ok := m.App(id)
	if ok {
		return v, nil
	}
	for _, a := range m.Apps() {
		if strings.EqualFold(a.App.Name, id) {
			return a, nil
		}
	}
	return v, fmt.Errorf("unknown app %q; see profilemanager list", id)
}

func findProfile(v engine.AppView, name string) (engine.Profile, error) {
	for _, p := range profilesFor(v) {
		if p.ID == name {
			return p, nil
		}
	}
	for _, p := range profilesFor(v) {
		if strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	return engine.Profile{}, fmt.Errorf("%s has no profile %q", v.App.Name, name)
}

func (c *cmd) switchCmd(args []string) error {
	fs := flag.NewFlagSet("switch", flag.ContinueOnError)
	fs.SetOutput(c.err)
	quit := fs.Bool("quit", false, "close the app first")
	if err := fs.Parse(reorder(args)); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: profilemanager switch <app> <profile> [--quit]")
	}
	m, err := c.manager()
	if err != nil {
		return err
	}
	v, err := c.app(m, fs.Arg(0))
	if err != nil {
		return err
	}
	p, err := findProfile(v, fs.Arg(1))
	if err != nil {
		return err
	}
	res, err := m.Switch(context.Background(), v.App.ID, p.ID, engine.SwitchOptions{QuitRunning: *quit})
	if err != nil {
		var re *engine.RunningError
		if errors.As(err, &re) {
			return fmt.Errorf("%w (or pass --quit)", err)
		}
		return err
	}
	if res.SavedAs != "" {
		fmt.Fprintf(c.out, "The sign-in that was active belonged to no profile; saved it as %q.\n", res.SavedAs)
	}
	fmt.Fprintf(c.out, "%s is now signed in as %s.\n", v.App.Name, p.Name)
	return nil
}

func (c *cmd) add(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(c.err)
	blank := fs.Bool("blank", false, "create an empty profile")
	if err := fs.Parse(reorder(args)); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: profilemanager add <app> <name> [--blank]")
	}
	m, err := c.manager()
	if err != nil {
		return err
	}
	v, err := c.app(m, fs.Arg(0))
	if err != nil {
		return err
	}
	src := engine.FromCurrent
	if *blank {
		src = engine.Blank
	}
	p, err := m.AddProfile(context.Background(), v.App.ID, fs.Arg(1), "", src)
	if err != nil {
		return err
	}
	switch {
	case *blank:
		fmt.Fprintf(c.out, "Created %q. Switch to it, then sign in to %s; the sign-in is saved into it.\n", p.Name, v.App.Name)
	case p.Empty:
		fmt.Fprintf(c.out, "Saved %q, but %s isn't signed in right now, so it's empty.\n", p.Name, v.App.Name)
	default:
		fmt.Fprintf(c.out, "Saved the current %s sign-in as %q.\n", v.App.Name, p.Name)
	}
	return nil
}

func (c *cmd) remove(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: profilemanager remove <app> <profile>")
	}
	m, err := c.manager()
	if err != nil {
		return err
	}
	v, err := c.app(m, args[0])
	if err != nil {
		return err
	}
	p, err := findProfile(v, args[1])
	if err != nil {
		return err
	}
	if err := m.DeleteProfile(v.App.ID, p.ID); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Deleted %q.\n", p.Name)
	return nil
}

// reorder moves flags before positional arguments, so "switch a b
// --quit" works like "switch --quit a b".
func reorder(args []string) []string {
	var flags, pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

func (c *cmd) sync(args []string) error {
	m, err := c.manager()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		errs := m.SyncAll(true)
		for id, err := range errs {
			fmt.Fprintf(c.err, "%s: %v\n", id, err)
		}
		if len(errs) > 0 {
			return exitCode(1)
		}
		fmt.Fprintln(c.out, "All active profiles are up to date.")
		return nil
	}
	v, err := c.app(m, args[0])
	if err != nil {
		return err
	}
	s, err := m.Sync(v.App.ID, true)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.out, map[engine.SyncStatus]string{
		engine.SyncUnchanged: "Up to date.",
		engine.SyncSaved:     "Saved the latest sign-in.",
		engine.SyncWaiting:   v.App.Name + " is running; it will be synced once it's closed.",
		engine.SyncConflict:  "The live sign-in belongs to another account (or was signed out); open the app to decide what to keep.",
		engine.SyncSkipped:   "No active profile to sync.",
	}[s])
	return nil
}

func (c *cmd) exec(args []string) error {
	dash := -1
	for i, a := range args {
		if a == "--" {
			dash = i
			break
		}
	}
	if dash != 2 || len(args) == 3 {
		return errors.New("usage: profilemanager exec <app> <profile> -- <command> [args...]")
	}
	return c.runIn(args[0], args[1], args[3:])
}

func (c *cmd) shell(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: profilemanager shell <app> <profile>")
	}
	fmt.Fprintf(c.err, "Starting a shell signed in to %s / %s. Exit it to save any changes and wipe the temporary copy.\n", args[0], args[1])
	return c.runIn(args[0], args[1], engine.Shell())
}

func (c *cmd) runIn(appID, profile string, argv []string) error {
	m, err := c.manager()
	if err != nil {
		return err
	}
	v, err := c.app(m, appID)
	if err != nil {
		return err
	}
	p, err := findProfile(v, profile)
	if err != nil {
		return err
	}
	// Ctrl+C reaches the child through the terminal; this process only
	// catches it (catching, unlike ignoring, isn't inherited) and waits,
	// so it can save changes and wipe the temporary copy afterwards.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	code, err := m.Exec(context.Background(), v.App.ID, p.ID, argv, engine.Stdio{In: os.Stdin, Out: c.out, Err: c.err})
	if err != nil {
		return err
	}
	if code != 0 {
		return exitCode(code)
	}
	return nil
}

func (c *cmd) status() error {
	m, err := c.manager()
	if err != nil {
		return err
	}
	s := m.Settings()
	fmt.Fprintf(c.out, "Data folder:     %s\n", m.DataDir())
	fmt.Fprintf(c.out, "Vault key:       %s\n", m.KeyLocation())
	fmt.Fprintf(c.out, "Auto-sync:       %v (every %ds)\n", s.AutoSync, s.IntervalSeconds)
	for _, p := range m.Registry().Problems() {
		fmt.Fprintf(c.out, "Definition problem: %s\n", p)
	}
	return nil
}

func (c *cmd) export(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: profilemanager export <app>")
	}
	m, err := c.manager()
	if err != nil {
		return err
	}
	data, err := m.ExportApp(args[0])
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.out, string(data))
	return err
}

func (c *cmd) importCmd(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: profilemanager import <file.json>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	m, err := c.manager()
	if err != nil {
		return err
	}
	a, err := m.ImportApp(data)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Added %s (%s).\n", a.Name, a.ID)
	return nil
}
