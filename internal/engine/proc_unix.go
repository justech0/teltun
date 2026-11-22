//go:build !windows

package engine

import "os/exec"

const adbName = "adb"

// hidden is a no-op off Windows: child processes have no console window.
func hidden(cmd *exec.Cmd) *exec.Cmd { return cmd }
