package engine

import (
	"os/exec"
	"syscall"
)

const adbName = "adb.exe"

// hidden makes a child process run with no console window.
func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}
