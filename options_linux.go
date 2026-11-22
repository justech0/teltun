package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// Linux: no system blur, so the page paints its own solid background.
func platformOptions(o *options.App) {
	o.Linux = &linux.Options{
		ProgramName:      "teltun",
		WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
	}
}
