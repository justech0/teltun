package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Windows: real acrylic glass behind the page.
func platformOptions(o *options.App) {
	o.BackgroundColour = &options.RGBA{R: 0, G: 0, B: 0, A: 0}
	o.Windows = &windows.Options{
		WebviewIsTransparent: true,
		WindowIsTranslucent:  true,
		BackdropType:         windows.Acrylic,
		Theme:                windows.Dark,
	}
}
