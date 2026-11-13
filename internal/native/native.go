// Package native drives programs that keep profiles of their own (gh,
// gcloud, kubectl, az) through their command-line tools.
package native

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/xmhrnx/profilemanager/internal/adapter"
)

// Runner runs a command and returns its combined output.
type Runner func(ctx context.Context, args []string) (string, error)

// ErrMissing is returned when the program isn't installed.
var ErrMissing = errors.New("not found on PATH")

// Exec runs commands for real.
func Exec(ctx context.Context, args []string) (string, error) {
	if _, err := exec.LookPath(args[0]); err != nil {
		return "", fmt.Errorf("%s: %w", args[0], ErrMissing)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout, cmd.Stderr = &out, &out
	hideWindow(cmd)
	err := cmd.Run()
	return out.String(), err
}

// Driver runs one app's profile commands.
type Driver struct {
	Spec *adapter.NativeSpec
	Run  Runner
}

// Installed reports whether the app's tool is on PATH.
func (d Driver) Installed() bool {
	_, err := exec.LookPath(d.Spec.List[0])
	return err == nil
}

// List returns the profiles the program knows.
func (d Driver) List(ctx context.Context) ([]string, error) {
	out, err := d.Run(ctx, d.Spec.List)
	// Some tools (gh auth status) exit non-zero when any account needs
	// attention but still list them all.
	if err != nil && (errors.Is(err, ErrMissing) || strings.TrimSpace(out) == "") {
		return nil, commandError(d.Spec.List, out, err)
	}
	return parse(out, d.Spec.ListRegex, true), nil
}

// Current returns the active profile, or "" when unknown.
func (d Driver) Current(ctx context.Context) (string, error) {
	if len(d.Spec.Current) == 0 {
		return "", nil
	}
	out, err := d.Run(ctx, d.Spec.Current)
	if err != nil && strings.TrimSpace(out) == "" {
		return "", commandError(d.Spec.Current, out, err)
	}
	names := parse(out, d.Spec.CurrentRegex, false)
	if len(names) == 0 {
		return "", nil
	}
	return names[0], nil
}

// Switch activates profile.
func (d Driver) Switch(ctx context.Context, profile string) error {
	args := expand(d.Spec.Switch, profile)
	if out, err := d.Run(ctx, args); err != nil {
		return commandError(args, out, err)
	}
	return nil
}

// CanCreate reports whether profiles can be created by command.
func (d Driver) CanCreate() bool { return len(d.Spec.Create) > 0 }

// Create makes a new profile.
func (d Driver) Create(ctx context.Context, profile string) error {
	args := expand(d.Spec.Create, profile)
	if out, err := d.Run(ctx, args); err != nil {
		return commandError(args, out, err)
	}
	return nil
}

func expand(args []string, profile string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.ReplaceAll(a, "{profile}", profile)
	}
	return out
}

func parse(out, pattern string, all bool) []string {
	var names []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			names = append(names, s)
		}
	}
	if pattern == "" {
		for _, line := range strings.Split(out, "\n") {
			add(line)
		}
		return names
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	n := 1
	if all {
		n = -1
	}
	for _, m := range re.FindAllStringSubmatch(out, n) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	return names
}

func commandError(args []string, out string, err error) error {
	msg := strings.TrimSpace(out)
	if len(msg) > 400 {
		msg = msg[:400] + "…"
	}
	if msg == "" {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, msg)
}
