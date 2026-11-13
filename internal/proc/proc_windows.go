//go:build windows

package proc

import (
	"encoding/csv"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type system struct{}

const createNoWindow = 0x08000000

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

func (system) List() ([]Process, error) {
	out, err := command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil, err
	}
	rows, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	var procs []Process
	for _, r := range rows {
		if len(r) < 2 {
			continue
		}
		pid, err := strconv.Atoi(r[1])
		if err != nil || pid == self {
			continue
		}
		procs = append(procs, Process{PID: pid, Name: r[0]})
	}
	return procs, nil
}

// Quit sends WM_CLOSE through taskkill without /F, as closing the
// window would, so the program saves its state.
func (system) Quit(pid int) error {
	return command("taskkill", "/PID", strconv.Itoa(pid)).Run()
}
