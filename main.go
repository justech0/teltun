// TelTun shares an Android phone's mobile data with a computer over USB.
// Made by Enes.
package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"teltun/internal/engine"
)

//go:embed all:frontend/dist
var assets embed.FS

var version = "1.1.1"

func main() {
	// Linux/macOS: the same binary, started as root, makes the TelTun card.
	if len(os.Args) > 1 && os.Args[1] == "--tun-helper" {
		os.Exit(engine.RunHelper(os.Args[2:]))
	}

	startHidden := false
	for _, a := range os.Args[1:] {
		if a == "--tray" {
			startHidden = true
		}
	}

	app := NewApp()
	app.tray = startTray(app)

	opts := &options.App{
		Title:            "TelTun",
		Width:            980,
		Height:           660,
		MinWidth:         860,
		MinHeight:        600,
		Frameless:        true,
		StartHidden:      startHidden,
		BackgroundColour: &options.RGBA{R: 10, G: 12, B: 34, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "teltun-6c1e0a52-enes",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				runtime.WindowShow(app.ctx)
				runtime.WindowUnminimise(app.ctx)
			},
		},
		Bind: []interface{}{app},
	}
	platformOptions(opts)

	if err := wails.Run(opts); err != nil {
		println("Error:", err.Error())
	}
}
