//go:build linux

package proc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type system struct{}

func (system) List() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	var out []Process
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		dir := filepath.Join("/proc", e.Name())
		// The executable's name isn't truncated like comm; it's only
		// readable for this user's processes, which are the ones that
		// matter here.
		if exe, err := os.Readlink(filepath.Join(dir, "exe")); err == nil {
			out = append(out, Process{PID: pid, Name: filepath.Base(strings.TrimSuffix(exe, " (deleted)"))})
		}
		if comm, err := os.ReadFile(filepath.Join(dir, "comm")); err == nil {
			out = append(out, Process{PID: pid, Name: strings.TrimSpace(string(comm))})
		}
	}
	return out, nil
}

func (system) Quit(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
