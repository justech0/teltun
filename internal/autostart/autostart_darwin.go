package autostart

// macOS: a per-user LaunchAgent.

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
)

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.enes.teltun.plist")
}

func Enabled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.enes.teltun</string>
  <key>ProgramArguments</key><array><string>%s</string><string>--tray</string></array>
  <key>RunAtLoad</key><true/>
</dict>
</plist>
`, html.EscapeString(exe))
	p := plistPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(plist), 0o644)
}

func Disable() error {
	err := os.Remove(plistPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
