// SlabCut — CNC Cut List Optimizer with GCode Export
//
// A cross-platform desktop application for optimizing rectangular
// cut lists from stock sheets and exporting CNC-ready GCode.
//
// Build:
//   go build -o slabcut ./cmd/slabcut
//
// Cross-compile:
//   GOOS=windows GOARCH=amd64 go build -o slabcut.exe ./cmd/slabcut
//   GOOS=darwin  GOARCH=amd64 go build -o slabcut-darwin ./cmd/slabcut
//
// Using fyne-cross (recommended for proper packaging):
//   go install github.com/fyne-io/fyne-cross@latest
//   fyne-cross windows -arch=amd64
//   fyne-cross darwin  -arch=amd64,arm64

package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/piwi3910/SlabCut/internal/assets"
	"github.com/piwi3910/SlabCut/internal/ui"
)

func main() {
	application := app.NewWithID("com.piwi3910.slabcut")
	application.SetIcon(fyne.NewStaticResource("icon.png", assets.IconPNG))

	window := application.NewWindow("SlabCut — CNC Cut List Optimizer")
	window.SetIcon(fyne.NewStaticResource("icon.png", assets.IconPNG))

	appUI := ui.NewApp(application, window)
	appUI.SetupMenus()
	window.SetContent(appUI.Build())
	window.Resize(fyne.NewSize(1400, 800))
	window.CenterOnScreen()

	ui.ShowSplash(application, 2500*time.Millisecond, func() {
		window.Show()
	})

	application.Run()
}
