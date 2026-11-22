package assets

import "embed"

// FS holds Google's adb for Linux and the phone-side helper for each CPU.
//
//go:embed adb/linux/adb phone/*
var FS embed.FS
