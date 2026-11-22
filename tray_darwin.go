package main

import "teltun/internal/engine"

// macOS: the tray needs the main thread, which the Wails window owns, so
// there is no tray icon yet. The window stays the way in.
type tray struct{}

func startTray(*App) *tray { return nil }

func (t *tray) update(engine.State, string) {}
