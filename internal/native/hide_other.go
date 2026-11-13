//go:build !windows

package native

import "os/exec"

func hideWindow(*exec.Cmd) {}
