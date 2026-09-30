// Package desktop runs cents as a native window whose UI is plain
// HTML/CSS/JS from the frontend directory, rendered by the OS webview.
//
// Wails needs build tags to produce a working window: build with
// `-tags desktop,production` (see the Makefile).
package desktop

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func Run(opts Options) error {
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		return fmt.Errorf("failed to load frontend assets: %w", err)
	}

	api := newAPI(opts)

	err = wails.Run(&options.App{
		Title:            "cents",
		Width:            720,
		Height:           640,
		MinWidth:         420,
		MinHeight:        360,
		BackgroundColour: &options.RGBA{R: 0x1F, G: 0x1A, B: 0x17, A: 0xFF},
		AssetServer:      &assetserver.Options{Assets: frontend},
		OnStartup:        api.startup,
		Bind:             []any{api},
	})
	if err != nil {
		return fmt.Errorf("desktop app failed: %w", err)
	}

	return nil
}
