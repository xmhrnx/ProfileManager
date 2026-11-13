//go:build darwin

package proc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type system struct{}

func (system) List() ([]Process, error) {
	out, err := exec.Command("/bin/ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	var procs []Process
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		pidStr, comm, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid == self {
			continue
		}
		comm = strings.TrimSpace(comm)
		procs = append(procs, Process{PID: pid, Name: filepath.Base(comm)})
		// App bundles: /Applications/Slack.app/Contents/MacOS/Slack
		if i := strings.Index(comm, ".app/"); i >= 0 {
			procs = append(procs, Process{PID: pid, Name: filepath.Base(comm[:i])})
		}
	}
	return procs, nil
}

func (system) Quit(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
