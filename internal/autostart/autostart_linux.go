package autostart

// Linux: an XDG autostart entry, which every common desktop reads.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func entryPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "autostart", "teltun.desktop")
}

func Enabled() bool {
	_, err := os.Stat(entryPath())
	return err == nil
}

func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	q := `"` + strings.ReplaceAll(exe, `"`, `\"`) + `"`
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=TelTun
Comment=Start TelTun at sign-in
Exec=%s --tray
Icon=teltun
Terminal=false
X-GNOME-Autostart-enabled=true
`, q)
	p := entryPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(entry), 0o644)
}

func Disable() error {
	err := os.Remove(entryPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
