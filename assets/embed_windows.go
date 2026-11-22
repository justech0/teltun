package assets

import "embed"

// FS holds Google's adb for Windows and the phone-side helper for each CPU.
//
//go:embed adb/windows/adb.exe adb/windows/AdbWinApi.dll adb/windows/AdbWinUsbApi.dll phone/*
var FS embed.FS
