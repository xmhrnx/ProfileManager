//go:build windows

package native

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps console tools from flashing a window from the GUI.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
