// Package proc finds running programs by process name and asks them to
// quit, so switching never pulls files out from under a running app.
package proc

import (
	"path/filepath"
	"strings"
	"time"
)

// Process is a running program.
type Process struct {
	PID  int
	Name string
}

// Finder lists and stops processes.
type Finder interface {
	// List returns the running processes.
	List() ([]Process, error)
	// Quit asks a process to exit cleanly (SIGTERM, WM_CLOSE).
	Quit(pid int) error
}

// System returns the Finder for this computer.
func System() Finder { return system{} }

// normalize lowercases a process name and drops a path and ".exe".
func normalize(name string) string {
	name = strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	return strings.TrimSuffix(name, ".exe")
}

// Linux truncates /proc/<pid>/comm to 15 bytes.
const commLen = 15

// Matches reports whether a running process name is one of names.
func Matches(running string, names []string) bool {
	r := normalize(running)
	for _, n := range names {
		n = normalize(n)
		if r == n || (len(r) == commLen && strings.HasPrefix(n, r)) {
			return true
		}
	}
	return false
}

// Running returns the processes matching names.
func Running(f Finder, names []string) ([]Process, error) {
	if len(names) == 0 {
		return nil, nil
	}
	all, err := f.List()
	if err != nil {
		return nil, err
	}
	var out []Process
	for _, p := range all {
		if Matches(p.Name, names) {
			out = append(out, p)
		}
	}
	return out, nil
}

// QuitAndWait asks every process matching names to quit, then waits up
// to timeout for them to exit. It returns the ones still running.
func QuitAndWait(f Finder, names []string, timeout time.Duration) ([]Process, error) {
	procs, err := Running(f, names)
	if err != nil || len(procs) == 0 {
		return nil, err
	}
	for _, p := range procs {
		f.Quit(p.PID) // a process that already exited is fine
	}
	deadline := time.Now().Add(timeout)
	for {
		left, err := Running(f, names)
		if err != nil || len(left) == 0 || time.Now().After(deadline) {
			return left, err
		}
		time.Sleep(250 * time.Millisecond)
	}
}
