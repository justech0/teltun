package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

func platformOptions(o *options.App) {
	o.Mac = &mac.Options{
		TitleBar:   mac.TitleBarHiddenInset(),
		Appearance: mac.NSAppearanceNameDarkAqua,
	}
}
