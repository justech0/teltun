//go:build !linux

package main

import "runtime"

// trayVisible: the Windows notification area is always there; macOS has no
// TelTun tray icon yet.
func trayVisible() bool { return runtime.GOOS == "windows" }
