package assets

import "embed"

// FS holds Google's adb for macOS and the phone-side helper for each CPU.
//
//go:embed adb/darwin/adb phone/*
var FS embed.FS
