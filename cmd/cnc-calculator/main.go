// CNCCalculator — CNC Cut List Optimizer with GCode Export
//
// A cross-platform desktop application for optimizing rectangular
// cut lists from stock sheets and exporting CNC-ready GCode.
//
// Build:
//   go build -o cnc-calculator ./cmd/cutoptimizer
//
// Cross-compile:
//   GOOS=windows GOARCH=amd64 go build -o cnc-calculator.exe ./cmd/cutoptimizer
//   GOOS=darwin  GOARCH=amd64 go build -o cnc-calculator-darwin ./cmd/cutoptimizer
//
// Using fyne-cross (recommended for proper packaging):
//   go install github.com/fyne-io/fyne-cross@latest
//   fyne-cross windows -arch=amd64
//   fyne-cross darwin  -arch=amd64,arm64

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/piwi3910/cnc-calculator/internal/ui"
)

func main() {
	application := app.NewWithID("com.piwi3910.cnc-calculator")
	window := application.NewWindow("CNCCalculator — CNC Cut List Optimizer")

	appUI := ui.NewApp(window)
	appUI.SetupMenus() // Setup the native menu bar
	window.SetContent(appUI.Build())
	window.Resize(fyne.NewSize(1000, 700))
	window.CenterOnScreen()
	window.ShowAndRun()
}
