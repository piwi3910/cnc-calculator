package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/piwi3910/SlabCut/internal/engine"
	"github.com/piwi3910/SlabCut/internal/export"
	"github.com/piwi3910/SlabCut/internal/gcode"
	partimporter "github.com/piwi3910/SlabCut/internal/importer"
	"github.com/piwi3910/SlabCut/internal/model"
	"github.com/piwi3910/SlabCut/internal/project"
	"github.com/piwi3910/SlabCut/internal/ui/widgets"
)

// App holds all application state and UI references.
type App struct {
	window  fyne.Window
	project model.Project
	tabs    *container.AppTabs
	history *History

	// UI references for dynamic updates
	partsContainer  *fyne.Container
	stockContainer  *fyne.Container
	resultContainer *fyne.Container
}

func NewApp(window fyne.Window) *App {
	return &App{
		window:  window,
		project: model.NewProject(),
		history: NewHistory(),
	}
}

// SetupMenus creates the native menu bar for the application.
func (a *App) SetupMenus() {
	// File Menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Project", func() {
			a.saveState("New Project")
			a.project = model.NewProject()
			a.refreshPartsList()
			a.refreshStockList()
			a.refreshResults()
		}),
		fyne.NewMenuItem("Open Project...", func() {
			a.loadProject()
		}),
		fyne.NewMenuItem("Save Project...", func() {
			a.saveProject()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Import Parts from CSV...", func() {
			a.importCSV()
		}),
		fyne.NewMenuItem("Import Parts from Excel...", func() {
			a.importExcel()
		}),
		fyne.NewMenuItem("Import Parts from DXF...", func() {
			a.importDXF()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Export GCode...", func() {
			a.exportGCode()
		}),
		fyne.NewMenuItem("Export PDF...", func() {
			a.exportPDF()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			a.window.Close()
		}),
	)

	// Edit Menu
	editMenu := fyne.NewMenu("Edit",
		fyne.NewMenuItem("Undo", func() {
			a.undo()
		}),
		fyne.NewMenuItem("Redo", func() {
			a.redo()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Clear All Parts", func() {
			a.saveState("Clear All Parts")
			a.project.Parts = nil
			a.refreshPartsList()
		}),
		fyne.NewMenuItem("Clear All Stock Sheets", func() {
			a.saveState("Clear All Stock Sheets")
			a.project.Stocks = nil
			a.refreshStockList()
		}),
	)

	// Tools Menu
	toolsMenu := fyne.NewMenu("Tools",
		fyne.NewMenuItem("Optimize", func() {
			a.runOptimize()
			a.tabs.SelectIndex(3) // Switch to Results tab
		}),
	)

	// Help Menu
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			a.showAboutDialog()
		}),
	)

	// Set the main menu
	mainMenu := fyne.NewMainMenu(
		fileMenu,
		editMenu,
		toolsMenu,
		helpMenu,
	)
	a.window.SetMainMenu(mainMenu)
}

func (a *App) showAboutDialog() {
	dialog.ShowInformation(
		"About SlabCut",
		"SlabCut — CNC Cut List Optimizer\n\n"+
			"A cross-platform desktop application for optimizing\n"+
			"rectangular cut lists and generating CNC-ready GCode.\n\n"+
			"Version 1.0.0",
		a.window,
	)
}

// Build constructs the full UI and returns the root container.
func (a *App) Build() fyne.CanvasObject {
	// Main tabs
	partsTab := container.NewTabItem("Parts", a.buildPartsPanel())
	stockTab := container.NewTabItem("Stock Sheets", a.buildStockPanel())
	settingsTab := container.NewTabItem("Settings", a.buildSettingsPanel())
	resultsTab := container.NewTabItem("Results", a.buildResultsPanel())

	a.tabs = container.NewAppTabs(partsTab, stockTab, settingsTab, resultsTab)
	a.tabs.SetTabLocation(container.TabLocationTop)

	a.registerShortcuts()

	return a.tabs
}

// saveState captures the current project state before a modification.
func (a *App) saveState(label string) {
	a.history.Push(MakeSnapshot(a.project.Parts, a.project.Stocks, label))
}

// undo restores the previous state from the undo stack.
func (a *App) undo() {
	current := MakeSnapshot(a.project.Parts, a.project.Stocks, "current")
	snap, ok := a.history.Undo(current)
	if !ok {
		return
	}
	a.project.Parts = snap.Parts
	a.project.Stocks = snap.Stocks
	a.refreshPartsList()
	a.refreshStockList()
}

// redo restores the next state from the redo stack.
func (a *App) redo() {
	current := MakeSnapshot(a.project.Parts, a.project.Stocks, "current")
	snap, ok := a.history.Redo(current)
	if !ok {
		return
	}
	a.project.Parts = snap.Parts
	a.project.Stocks = snap.Stocks
	a.refreshPartsList()
	a.refreshStockList()
}

// registerShortcuts adds keyboard shortcuts for undo and redo.
func (a *App) registerShortcuts() {
	canvas := a.window.Canvas()

	// Ctrl+Z / Cmd+Z -> Undo
	canvas.AddShortcut(&fyne.ShortcutUndo{}, func(_ fyne.Shortcut) {
		a.undo()
	})

	// Ctrl+Y -> Redo (CustomShortcut for non-Mac)
	canvas.AddShortcut(&fyne.ShortcutRedo{}, func(_ fyne.Shortcut) {
		a.redo()
	})
}

// ─── Parts Panel ───────────────────────────────────────────

func (a *App) buildPartsPanel() fyne.CanvasObject {
	a.partsContainer = container.NewVBox()
	a.refreshPartsList()

	addBtn := widget.NewButtonWithIcon("Add Part", theme.ContentAddIcon(), func() {
		a.showAddPartDialog()
	})

	return container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("Required Parts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			addBtn,
		),
		nil, nil, nil,
		container.NewVScroll(a.partsContainer),
	)
}

func (a *App) refreshPartsList() {
	a.partsContainer.RemoveAll()

	if len(a.project.Parts) == 0 {
		a.partsContainer.Add(widget.NewLabel("No parts added yet. Click 'Add Part' to begin."))
		return
	}

	// Header
	header := container.NewGridWithColumns(7,
		widget.NewLabelWithStyle("Label", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Width (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Height (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Qty", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Grain", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
	)
	a.partsContainer.Add(header)
	a.partsContainer.Add(widget.NewSeparator())

	for i := range a.project.Parts {
		idx := i // capture
		p := a.project.Parts[idx]
		row := container.NewGridWithColumns(7,
			widget.NewLabel(p.Label),
			widget.NewLabel(fmt.Sprintf("%.1f", p.Width)),
			widget.NewLabel(fmt.Sprintf("%.1f", p.Height)),
			widget.NewLabel(fmt.Sprintf("%d", p.Quantity)),
			widget.NewLabel(p.Grain.String()),
			widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
				a.showEditPartDialog(idx)
			}),
			widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				a.saveState("Delete Part")
				a.project.Parts = append(a.project.Parts[:idx], a.project.Parts[idx+1:]...)
				a.refreshPartsList()
			}),
		)
		a.partsContainer.Add(row)
	}
}

func (a *App) showAddPartDialog() {
	labelEntry := widget.NewEntry()
	labelEntry.SetPlaceHolder("Part name")
	labelEntry.SetText(fmt.Sprintf("Part %d", len(a.project.Parts)+1))

	widthEntry := widget.NewEntry()
	widthEntry.SetPlaceHolder("Width in mm")

	heightEntry := widget.NewEntry()
	heightEntry.SetPlaceHolder("Height in mm")

	qtyEntry := widget.NewEntry()
	qtyEntry.SetText("1")

	grainSelect := widget.NewSelect([]string{"None", "Horizontal", "Vertical"}, nil)
	grainSelect.SetSelected("None")

	form := dialog.NewForm("Add Part", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
			widget.NewFormItem("Grain", grainSelect),
		},
		func(ok bool) {
			if !ok {
				return
			}
			w, _ := strconv.ParseFloat(widthEntry.Text, 64)
			h, _ := strconv.ParseFloat(heightEntry.Text, 64)
			q, _ := strconv.Atoi(qtyEntry.Text)
			if w <= 0 || h <= 0 || q <= 0 {
				dialog.ShowError(fmt.Errorf("width, height, and quantity must be > 0"), a.window)
				return
			}

			part := model.NewPart(labelEntry.Text, w, h, q)
			switch grainSelect.Selected {
			case "Horizontal":
				part.Grain = model.GrainHorizontal
			case "Vertical":
				part.Grain = model.GrainVertical
			}

			a.saveState("Add Part")
			a.project.Parts = append(a.project.Parts, part)
			a.refreshPartsList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 350))
	form.Show()
}

func (a *App) showEditPartDialog(idx int) {
	p := a.project.Parts[idx]

	labelEntry := widget.NewEntry()
	labelEntry.SetPlaceHolder("Part name")
	labelEntry.SetText(p.Label)

	widthEntry := widget.NewEntry()
	widthEntry.SetText(fmt.Sprintf("%.1f", p.Width))

	heightEntry := widget.NewEntry()
	heightEntry.SetText(fmt.Sprintf("%.1f", p.Height))

	qtyEntry := widget.NewEntry()
	qtyEntry.SetText(fmt.Sprintf("%d", p.Quantity))

	grainSelect := widget.NewSelect([]string{"None", "Horizontal", "Vertical"}, nil)
	grainSelect.SetSelected(p.Grain.String())

	form := dialog.NewForm("Edit Part", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
			widget.NewFormItem("Grain", grainSelect),
		},
		func(ok bool) {
			if !ok {
				return
			}
			w, _ := strconv.ParseFloat(widthEntry.Text, 64)
			h, _ := strconv.ParseFloat(heightEntry.Text, 64)
			q, _ := strconv.Atoi(qtyEntry.Text)
			if w <= 0 || h <= 0 || q <= 0 {
				dialog.ShowError(fmt.Errorf("width, height, and quantity must be > 0"), a.window)
				return
			}

			// Update the existing part
			a.saveState("Edit Part")
			a.project.Parts[idx].Label = labelEntry.Text
			a.project.Parts[idx].Width = w
			a.project.Parts[idx].Height = h
			a.project.Parts[idx].Quantity = q
			switch grainSelect.Selected {
			case "Horizontal":
				a.project.Parts[idx].Grain = model.GrainHorizontal
			case "Vertical":
				a.project.Parts[idx].Grain = model.GrainVertical
			default:
				a.project.Parts[idx].Grain = model.GrainNone
			}
			a.refreshPartsList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 350))
	form.Show()
}

// ─── Stock Sheets Panel ────────────────────────────────────

func (a *App) buildStockPanel() fyne.CanvasObject {
	a.stockContainer = container.NewVBox()
	a.refreshStockList()

	addBtn := widget.NewButtonWithIcon("Add Stock Sheet", theme.ContentAddIcon(), func() {
		a.showAddStockDialog()
	})

	return container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("Available Stock Sheets", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			addBtn,
		),
		nil, nil, nil,
		container.NewVScroll(a.stockContainer),
	)
}

func (a *App) refreshStockList() {
	a.stockContainer.RemoveAll()

	if len(a.project.Stocks) == 0 {
		a.stockContainer.Add(widget.NewLabel("No stock sheets defined. Click 'Add Stock Sheet' to begin."))
		return
	}

	header := container.NewGridWithColumns(6,
		widget.NewLabelWithStyle("Label", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Width (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Height (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Qty", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
	)
	a.stockContainer.Add(header)
	a.stockContainer.Add(widget.NewSeparator())

	for i := range a.project.Stocks {
		idx := i
		s := a.project.Stocks[idx]
		row := container.NewGridWithColumns(6,
			widget.NewLabel(s.Label),
			widget.NewLabel(fmt.Sprintf("%.1f", s.Width)),
			widget.NewLabel(fmt.Sprintf("%.1f", s.Height)),
			widget.NewLabel(fmt.Sprintf("%d", s.Quantity)),
			widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
				a.showEditStockDialog(idx)
			}),
			widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				a.saveState("Delete Stock Sheet")
				a.project.Stocks = append(a.project.Stocks[:idx], a.project.Stocks[idx+1:]...)
				a.refreshStockList()
			}),
		)
		a.stockContainer.Add(row)
	}
}

// stockPreset defines a common stock sheet size for quick selection.
type stockPreset struct {
	Label  string
	Width  float64
	Height float64
}

// Common stock sheet presets covering standard panel sizes worldwide.
var stockPresets = []stockPreset{
	{Label: "Custom", Width: 0, Height: 0},
	{Label: "Full Sheet (2440 x 1220)", Width: 2440, Height: 1220},
	{Label: "Half Sheet (1220 x 1220)", Width: 1220, Height: 1220},
	{Label: "Quarter Sheet (1220 x 610)", Width: 1220, Height: 610},
	{Label: "Large Sheet (3050 x 1525)", Width: 3050, Height: 1525},
	{Label: "Euro Full (2500 x 1250)", Width: 2500, Height: 1250},
	{Label: "Euro Half (1250 x 1250)", Width: 1250, Height: 1250},
	{Label: "Small Panel (600 x 300)", Width: 600, Height: 300},
}

func (a *App) showAddStockDialog() {
	labelEntry := widget.NewEntry()
	labelEntry.SetText("Plywood 2440x1220")

	widthEntry := widget.NewEntry()
	widthEntry.SetText("2440")

	heightEntry := widget.NewEntry()
	heightEntry.SetText("1220")

	qtyEntry := widget.NewEntry()
	qtyEntry.SetText("1")

	// Build preset names for the dropdown
	presetNames := make([]string, len(stockPresets))
	for i, p := range stockPresets {
		presetNames[i] = p.Label
	}

	presetSelect := widget.NewSelect(presetNames, func(selected string) {
		for _, p := range stockPresets {
			if p.Label == selected && p.Width > 0 {
				widthEntry.SetText(fmt.Sprintf("%.0f", p.Width))
				heightEntry.SetText(fmt.Sprintf("%.0f", p.Height))
				labelEntry.SetText(fmt.Sprintf("Plywood %.0fx%.0f", p.Width, p.Height))
				break
			}
		}
	})
	presetSelect.PlaceHolder = "Select a preset size..."

	form := dialog.NewForm("Add Stock Sheet", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Preset Size", presetSelect),
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			w, _ := strconv.ParseFloat(widthEntry.Text, 64)
			h, _ := strconv.ParseFloat(heightEntry.Text, 64)
			q, _ := strconv.Atoi(qtyEntry.Text)
			if w <= 0 || h <= 0 || q <= 0 {
				dialog.ShowError(fmt.Errorf("width, height, and quantity must be > 0"), a.window)
				return
			}
			a.saveState("Add Stock Sheet")
			a.project.Stocks = append(a.project.Stocks, model.NewStockSheet(labelEntry.Text, w, h, q))
			a.refreshStockList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 400))
	form.Show()
}

func (a *App) showEditStockDialog(idx int) {
	s := a.project.Stocks[idx]

	labelEntry := widget.NewEntry()
	labelEntry.SetText(s.Label)

	widthEntry := widget.NewEntry()
	widthEntry.SetText(fmt.Sprintf("%.1f", s.Width))

	heightEntry := widget.NewEntry()
	heightEntry.SetText(fmt.Sprintf("%.1f", s.Height))

	qtyEntry := widget.NewEntry()
	qtyEntry.SetText(fmt.Sprintf("%d", s.Quantity))

	form := dialog.NewForm("Edit Stock Sheet", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			w, _ := strconv.ParseFloat(widthEntry.Text, 64)
			h, _ := strconv.ParseFloat(heightEntry.Text, 64)
			q, _ := strconv.Atoi(qtyEntry.Text)
			if w <= 0 || h <= 0 || q <= 0 {
				dialog.ShowError(fmt.Errorf("width, height, and quantity must be > 0"), a.window)
				return
			}
			a.saveState("Edit Stock Sheet")
			a.project.Stocks[idx].Label = labelEntry.Text
			a.project.Stocks[idx].Width = w
			a.project.Stocks[idx].Height = h
			a.project.Stocks[idx].Quantity = q
			a.refreshStockList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 300))
	form.Show()
}

// ─── Settings Panel ────────────────────────────────────────

func (a *App) buildSettingsPanel() fyne.CanvasObject {
	s := &a.project.Settings

	// Helper to create a bound float entry
	floatEntry := func(val *float64) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(fmt.Sprintf("%.1f", *val))
		e.OnChanged = func(text string) {
			if v, err := strconv.ParseFloat(text, 64); err == nil {
				*val = v
			}
		}
		return e
	}

	intEntry := func(val *int) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(fmt.Sprintf("%d", *val))
		e.OnChanged = func(text string) {
			if v, err := strconv.Atoi(text); err == nil {
				*val = v
			}
		}
		return e
	}

	algorithmSelect := widget.NewSelect([]string{"Guillotine (Fast)", "Genetic Algorithm (Better)"}, func(selected string) {
		switch selected {
		case "Genetic Algorithm (Better)":
			s.Algorithm = model.AlgorithmGenetic
		default:
			s.Algorithm = model.AlgorithmGuillotine
		}
	})
	switch s.Algorithm {
	case model.AlgorithmGenetic:
		algorithmSelect.SetSelected("Genetic Algorithm (Better)")
	default:
		algorithmSelect.SetSelected("Guillotine (Fast)")
	}

	optimizerSection := widget.NewCard("Optimizer", "", container.NewGridWithColumns(2,
		widget.NewLabel("Algorithm"), algorithmSelect,
		widget.NewLabel("Kerf / Blade Width (mm)"), floatEntry(&s.KerfWidth),
		widget.NewLabel("Edge Trim (mm)"), floatEntry(&s.EdgeTrim),
		widget.NewLabel("Guillotine Cuts Only"), widget.NewCheck("", func(b bool) { s.GuillotineOnly = b }),
	))

	cncSection := widget.NewCard("CNC / GCode", "", container.NewGridWithColumns(2,
		widget.NewLabel("GCode Profile"), a.buildProfileSelector(),
		widget.NewLabel("Tool Diameter (mm)"), floatEntry(&s.ToolDiameter),
		widget.NewLabel("Feed Rate (mm/min)"), floatEntry(&s.FeedRate),
		widget.NewLabel("Plunge Rate (mm/min)"), floatEntry(&s.PlungeRate),
		widget.NewLabel("Spindle Speed (RPM)"), intEntry(&s.SpindleSpeed),
		widget.NewLabel("Safe Z Height (mm)"), floatEntry(&s.SafeZ),
		widget.NewLabel("Material Thickness (mm)"), floatEntry(&s.CutDepth),
		widget.NewLabel("Pass Depth (mm)"), floatEntry(&s.PassDepth),
	))

	// Stock sheet holding tabs (for securing sheet to CNC bed)
	stockTabEnabled := widget.NewCheck("", func(b bool) { s.StockTabs.Enabled = b })
	stockTabEnabled.Checked = s.StockTabs.Enabled

	stockTabSection := widget.NewCard("Stock Holding Tabs", "",
		container.NewVBox(
			container.NewGridWithColumns(2,
				widget.NewLabel("Enable Stock Tabs"), stockTabEnabled,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Top Padding (mm)"), floatEntry(&s.StockTabs.TopPadding),
				widget.NewLabel("Bottom Padding (mm)"), floatEntry(&s.StockTabs.BottomPadding),
				widget.NewLabel("Left Padding (mm)"), floatEntry(&s.StockTabs.LeftPadding),
				widget.NewLabel("Right Padding (mm)"), floatEntry(&s.StockTabs.RightPadding),
			),
		),
	)

	return container.NewVScroll(container.NewVBox(
		optimizerSection,
		cncSection,
		stockTabSection,
	))
}

func (a *App) buildProfileSelector() *widget.Select {
	profileNames := model.GetProfileNames()
	selector := widget.NewSelect(profileNames, func(selected string) {
		a.project.Settings.GCodeProfile = selected
	})
	selector.SetSelected(a.project.Settings.GCodeProfile)
	return selector
}

// ─── Results Panel ─────────────────────────────────────────

func (a *App) buildResultsPanel() fyne.CanvasObject {
	a.resultContainer = container.NewStack(
		widget.NewLabel("No results yet. Add parts and stock, then click Optimize."),
	)
	return a.resultContainer
}

func (a *App) refreshResults() {
	a.resultContainer.RemoveAll()
	a.resultContainer.Add(widgets.RenderSheetResults(a.project.Result, a.project.Settings))
	a.resultContainer.Refresh()
}

// ─── Actions ───────────────────────────────────────────────

func (a *App) runOptimize() {
	if len(a.project.Parts) == 0 {
		dialog.ShowInformation("Nothing to optimize", "Add at least one part first.", a.window)
		return
	}
	if len(a.project.Stocks) == 0 {
		dialog.ShowInformation("No stock sheets", "Add at least one stock sheet first.", a.window)
		return
	}

	opt := engine.New(a.project.Settings)
	result := opt.Optimize(a.project.Parts, a.project.Stocks)
	a.project.Result = &result
	a.refreshResults()
}

func (a *App) saveProject() {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		path := writer.URI().Path()
		if err := project.Save(path, a.project); err != nil {
			dialog.ShowError(err, a.window)
		}
	}, a.window)
	d.SetFileName(a.project.Name + ".cnccalc")
	d.Show()
}

func (a *App) loadProject() {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		proj, err := project.Load(reader.URI().Path())
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		a.saveState("Load Project")
		a.project = proj
		a.refreshPartsList()
		a.refreshStockList()
		if a.project.Result != nil {
			a.refreshResults()
		}
	}, a.window)
	d.Show()
}

func (a *App) exportGCode() {
	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		dialog.ShowInformation("No results", "Run the optimizer first before exporting GCode.", a.window)
		return
	}

	gen := gcode.New(a.project.Settings)
	codes := gen.GenerateAll(*a.project.Result)

	// If single sheet, save one file. If multiple, ask which or save all.
	if len(codes) == 1 {
		a.saveGCodeFile(codes[0], "sheet1.gcode")
		return
	}

	// For multiple sheets, save each one
	for i, code := range codes {
		filename := fmt.Sprintf("sheet%d.gcode", i+1)
		a.saveGCodeFile(code, filename)
	}
}

func (a *App) saveGCodeFile(code, defaultName string) {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		if err := project.ExportGCode(writer.URI().Path(), code); err != nil {
			dialog.ShowError(err, a.window)
		} else {
			dialog.ShowInformation("Export Complete",
				fmt.Sprintf("GCode saved to %s", writer.URI().Path()), a.window)
		}
	}, a.window)
	d.SetFileName(defaultName)
	d.Show()
}

func (a *App) exportPDF() {
	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		dialog.ShowInformation("No results", "Run the optimizer first before exporting PDF.", a.window)
		return
	}

	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		// Close the writer immediately since ExportPDF writes directly to the file path
		writer.Close()
		path := writer.URI().Path()
		if exportErr := export.ExportPDF(path, *a.project.Result, a.project.Settings); exportErr != nil {
			dialog.ShowError(exportErr, a.window)
		} else {
			dialog.ShowInformation("Export Complete",
				fmt.Sprintf("PDF saved to %s", path), a.window)
		}
	}, a.window)
	d.SetFileName("cut-layout.pdf")
	d.Show()
}

// ─── Import Functions ───────────────────────────────────────

func (a *App) importCSV() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		result := partimporter.ImportCSV(reader.URI().Path())
		a.handleImportResult(result)
	}, a.window)
}

func (a *App) importExcel() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		result := partimporter.ImportExcel(reader.URI().Path())
		a.handleImportResult(result)
	}, a.window)
}

func (a *App) importDXF() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		result := partimporter.ImportDXF(reader.URI().Path())
		a.handleImportResult(result)
	}, a.window)
}

func (a *App) handleImportResult(result partimporter.ImportResult) {
	// Build a comprehensive summary message
	var summary strings.Builder

	// Results summary
	summary.WriteString(fmt.Sprintf("Parts imported: %d", len(result.Parts)))

	if len(result.Errors) > 0 {
		summary.WriteString(fmt.Sprintf("\nRows skipped: %d", len(result.Errors)))
	}

	// Warnings section
	if len(result.Warnings) > 0 {
		summary.WriteString("\n\nWarnings:\n")
		for _, w := range result.Warnings {
			summary.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	// Errors section
	if len(result.Errors) > 0 {
		summary.WriteString("\nErrors:\n")
		maxErrors := 10
		for i, e := range result.Errors {
			if i >= maxErrors {
				summary.WriteString(fmt.Sprintf("  ... and %d more errors\n", len(result.Errors)-maxErrors))
				break
			}
			summary.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	// Add imported parts to the project
	if len(result.Parts) > 0 {
		a.saveState("Import Parts")
		a.project.Parts = append(a.project.Parts, result.Parts...)
		a.refreshPartsList()
	}

	// Show the summary dialog
	if len(result.Parts) == 0 && len(result.Errors) > 0 {
		dialog.ShowError(fmt.Errorf("import failed\n\n%s", summary.String()), a.window)
	} else {
		dialog.ShowInformation("Import Summary", summary.String(), a.window)
	}
}
