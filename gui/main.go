package main

import (
	"embed"

	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	parentconfig "github.com/cache-22/cache-22-client/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg := parentconfig.New()

	servers := NewServerService(cfg.DataDir)
	emu := NewEmulatorService(servers, cfg.DataDir, cfg.PCSX2Version, cfg.Profiling)

	app := application.New(application.Options{
		Name:        "Cache-22",
		Description: "PS2 library streamer",
		Services: []application.Service{
			application.NewService(servers),
			application.NewService(NewLibraryService(servers, cfg.DataDir)),
			application.NewService(NewDownloadService(servers, cfg.DataDir)),
			application.NewService(emu),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	emu.SetPicker(func() ([]string, error) {
		return app.Dialog.OpenFile().
			CanChooseFiles(true).
			AddFilter("BIOS dumps", "*.bin;*.rom;*.bios;*.BIN;*.ROM").
			SetTitle("Select your PS2 BIOS dump").
			PromptForMultipleSelection()
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "Cache-22",
		Width:  1100,
		Height: 700,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(9, 9, 11),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
