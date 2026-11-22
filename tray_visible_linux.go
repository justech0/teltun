package main

import "github.com/godbus/dbus/v5"

// trayVisible reports whether a tray host is running. Plain GNOME has none
// unless the AppIndicator extension is on, so the icon would be invisible.
func trayVisible() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var has bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0,
		"org.kde.StatusNotifierWatcher").Store(&has)
	return err == nil && has
}
