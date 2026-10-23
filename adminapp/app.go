// Package adminapp runs the separate administration desktop application.
package adminapp

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/lazybark/cents/config"
	"github.com/lazybark/cents/desktop"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

type frontendFS struct{ admin, shared fs.FS }

func (f frontendFS) Open(name string) (fs.File, error) {
	file, err := f.admin.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return f.shared.Open(name)
	}
	return file, err
}

func Run() error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	api, err := newAPI(filepath.Join(dir, "admin.yml"))
	if err != nil {
		return err
	}
	shared, err := desktop.FrontendAssets()
	if err != nil {
		return err
	}
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		return err
	}
	return wails.Run(&options.App{
		Title: "cents admin", Width: 1100, Height: 780, MinWidth: 760, MinHeight: 540,
		BackgroundColour: &options.RGBA{R: 0x1F, G: 0x1A, B: 0x17, A: 0xFF},
		AssetServer:      &assetserver.Options{Assets: frontendFS{frontend, shared}},
		OnStartup:        func(ctx context.Context) { api.startup(ctx) },
		OnShutdown:       func(context.Context) { api.cancel() }, Bind: []any{api},
	})
}
