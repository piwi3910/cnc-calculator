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
	"github.com/piwi3910/SlabCut/internal/version"
)

// App holds all application state and UI references.
type App struct {
	app     fyne.App
	window  fyne.Window
	project model.Project
	config  model.AppConfig
	library model.PartsLibrary
	tabs    *container.AppTabs
	history *History

	// Inventory management
	inventory     model.Inventory
	inventoryPath string

	// UI references for dynamic updates
	partsContainer  *fyne.Container
	stockContainer  *fyne.Container
	resultContainer *fyne.Container
	profileSelector *widget.Select
}

func NewApp(application fyne.App, window fyne.Window) *App {
	cfg, err := project.LoadAppConfig(project.DefaultConfigPath())
	if err != nil {
		cfg = model.DefaultAppConfig()
	}

	proj := model.NewProject()
	cfg.ApplyToSettings(&proj.Settings)

	lib, libErr := project.LoadDefaultLibrary()
	if libErr != nil {
		fmt.Printf("Warning: could not load parts library: %v\n", libErr)
		lib = model.NewPartsLibrary()
	}

	app := &App{
		app:     application,
		window:  window,
		project: proj,
		config:  cfg,
		library: lib,
		history: NewHistory(),
	}
	app.loadCustomProfiles()
	app.loadInventory()
	app.applyTheme()
	return app
}

// applyTheme sets the Fyne theme based on the current config.
func (a *App) applyTheme() {
	switch a.config.Theme {
	case "light":
		a.app.Settings().SetTheme(theme.LightTheme())
	case "dark":
		a.app.Settings().SetTheme(theme.DarkTheme())
	default:
		a.app.Settings().SetTheme(theme.DefaultTheme())
	}
}

// loadInventory loads tool and stock inventory from the default path.
func (a *App) loadInventory() {
	inv, path, err := project.LoadOrCreateInventory()
	if err != nil {
		fmt.Printf("Warning: could not load inventory: %v\n", err)
		a.inventory = model.DefaultInventory()
		return
	}
	a.inventory = inv
	a.inventoryPath = path
}

// loadCustomProfiles loads user-defined GCode profiles from disk on startup.
func (a *App) loadCustomProfiles() {
	profiles, err := project.LoadCustomProfilesFromDefault()
	if err != nil {
		fmt.Printf("Warning: failed to load custom profiles: %v\n", err)
		return
	}
	model.CustomProfiles = profiles
}

// refreshProfileSelector updates the profile dropdown with all available profiles.
func (a *App) refreshProfileSelector() {
	if a.profileSelector != nil {
		a.profileSelector.Options = model.GetProfileNames()
		a.profileSelector.Refresh()
	}
}

// SetupMenus creates the native menu bar for the application.
func (a *App) SetupMenus() {
	// File Menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Project", func() {
			a.saveState("New Project")
			a.project = model.NewProject()
			a.config.ApplyToSettings(&a.project.Settings)
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
		fyne.NewMenuItem("Preview GCode...", func() {
			a.previewGCode()
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
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Purchasing Calculator...", func() {
			a.showPurchasingCalculator()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Manage GCode Profiles...", func() {
			a.showProfileManager()
		}),
	)

	// Admin Menu
	adminMenu := fyne.NewMenu("Admin",
		fyne.NewMenuItem("Parts Library...", func() {
			a.showLibraryManager()
		}),
		fyne.NewMenuItem("Tool Inventory...", func() {
			a.showToolInventoryDialog()
		}),
		fyne.NewMenuItem("Stock Inventory...", func() {
			a.showStockInventoryDialog()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Import/Export Data...", func() {
			a.showImportExportDialog()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Settings...", func() {
			a.showSettingsDialog()
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
		adminMenu,
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
			version.Short(),
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

	statusBar := widget.NewLabelWithStyle(
		"SlabCut "+version.Short(),
		fyne.TextAlignTrailing,
		fyne.TextStyle{Italic: true},
	)

	return container.NewBorder(nil, statusBar, nil, nil, a.tabs)
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

	addFromLibBtn := widget.NewButtonWithIcon("Add from Library", theme.FolderOpenIcon(), func() {
		a.showAddFromLibraryDialog()
	})

	return container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("Required Parts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			addFromLibBtn,
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
	header := container.NewGridWithColumns(10,
		widget.NewLabelWithStyle("Label", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Width (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Height (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Qty", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Grain", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Material", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Banding", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
	)
	a.partsContainer.Add(header)
	a.partsContainer.Add(widget.NewSeparator())

	for i := range a.project.Parts {
		idx := i // capture
		p := a.project.Parts[idx]
		matLabel := "-"
		if p.Material != "" {
			matLabel = p.Material
		}
		row := container.NewGridWithColumns(10,
			widget.NewLabel(p.Label),
			widget.NewLabel(fmt.Sprintf("%.1f", p.Width)),
			widget.NewLabel(fmt.Sprintf("%.1f", p.Height)),
			widget.NewLabel(fmt.Sprintf("%d", p.Quantity)),
			widget.NewLabel(p.Grain.String()),
			widget.NewLabel(matLabel),
			widget.NewLabel(p.EdgeBanding.String()),
			widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
				a.showEditPartDialog(idx)
			}),
			widget.NewButtonWithIcon("", theme.DownloadIcon(), func() {
				a.showSaveToLibraryDialog(a.project.Parts[idx])
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

	materialEntry := widget.NewEntry()
	materialEntry.SetPlaceHolder("e.g., Plywood, MDF (optional)")

	bandTop := widget.NewCheck("Top", nil)
	bandBottom := widget.NewCheck("Bottom", nil)
	bandLeft := widget.NewCheck("Left", nil)
	bandRight := widget.NewCheck("Right", nil)
	bandingRow := container.NewHBox(bandTop, bandBottom, bandLeft, bandRight)

	form := dialog.NewForm("Add Part", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
			widget.NewFormItem("Grain", grainSelect),
			widget.NewFormItem("Material", materialEntry),
			widget.NewFormItem("Edge Banding", bandingRow),
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
			part.Material = strings.TrimSpace(materialEntry.Text)
			part.EdgeBanding = model.EdgeBanding{
				Top:    bandTop.Checked,
				Bottom: bandBottom.Checked,
				Left:   bandLeft.Checked,
				Right:  bandRight.Checked,
			}

			a.saveState("Add Part")
			a.project.Parts = append(a.project.Parts, part)
			a.refreshPartsList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 420))
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

	editMaterialEntry := widget.NewEntry()
	editMaterialEntry.SetPlaceHolder("e.g., Plywood, MDF (optional)")
	editMaterialEntry.SetText(p.Material)

	bandTop := widget.NewCheck("Top", nil)
	bandTop.Checked = p.EdgeBanding.Top
	bandBottom := widget.NewCheck("Bottom", nil)
	bandBottom.Checked = p.EdgeBanding.Bottom
	bandLeft := widget.NewCheck("Left", nil)
	bandLeft.Checked = p.EdgeBanding.Left
	bandRight := widget.NewCheck("Right", nil)
	bandRight.Checked = p.EdgeBanding.Right
	bandingRow := container.NewHBox(bandTop, bandBottom, bandLeft, bandRight)

	form := dialog.NewForm("Edit Part", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
			widget.NewFormItem("Grain", grainSelect),
			widget.NewFormItem("Material", editMaterialEntry),
			widget.NewFormItem("Edge Banding", bandingRow),
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
			a.project.Parts[idx].Material = strings.TrimSpace(editMaterialEntry.Text)
			a.project.Parts[idx].EdgeBanding = model.EdgeBanding{
				Top:    bandTop.Checked,
				Bottom: bandBottom.Checked,
				Left:   bandLeft.Checked,
				Right:  bandRight.Checked,
			}
			a.refreshPartsList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 420))
	form.Show()
}

// ─── Stock Sheets Panel ────────────────────────────────────

func (a *App) buildStockPanel() fyne.CanvasObject {
	a.stockContainer = container.NewVBox()
	a.refreshStockList()

	addBtn := widget.NewButtonWithIcon("Add Stock Sheet", theme.ContentAddIcon(), func() {
		a.showAddStockDialog()
	})

	fromInventoryBtn := widget.NewButtonWithIcon("Add from Inventory", theme.FolderOpenIcon(), func() {
		a.showAddStockFromInventory()
	})

	return container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("Available Stock Sheets", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			fromInventoryBtn,
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

	header := container.NewGridWithColumns(9,
		widget.NewLabelWithStyle("Label", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Width (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Height (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Qty", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Grain", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Material", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Price/Sheet", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
	)
	a.stockContainer.Add(header)
	a.stockContainer.Add(widget.NewSeparator())

	for i := range a.project.Stocks {
		idx := i
		s := a.project.Stocks[idx]
		priceLabel := "-"
		if s.PricePerSheet > 0 {
			priceLabel = fmt.Sprintf("%.2f", s.PricePerSheet)
		}
		stockMatLabel := "-"
		if s.Material != "" {
			stockMatLabel = s.Material
		}
		row := container.NewGridWithColumns(9,
			widget.NewLabel(s.Label),
			widget.NewLabel(fmt.Sprintf("%.1f", s.Width)),
			widget.NewLabel(fmt.Sprintf("%.1f", s.Height)),
			widget.NewLabel(fmt.Sprintf("%d", s.Quantity)),
			widget.NewLabel(s.Grain.String()),
			widget.NewLabel(stockMatLabel),
			widget.NewLabel(priceLabel),
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

	grainSelect := widget.NewSelect([]string{"None", "Horizontal", "Vertical"}, nil)
	grainSelect.SetSelected("None")

	stockMaterialEntry := widget.NewEntry()
	stockMaterialEntry.SetPlaceHolder("e.g., Plywood, MDF (optional)")

	priceEntry := widget.NewEntry()
	priceEntry.SetPlaceHolder("0.00 (optional)")
	priceEntry.SetText("0")

	form := dialog.NewForm("Add Stock Sheet", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Preset Size", presetSelect),
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
			widget.NewFormItem("Grain Direction", grainSelect),
			widget.NewFormItem("Material", stockMaterialEntry),
			widget.NewFormItem("Price per Sheet", priceEntry),
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
			sheet := model.NewStockSheet(labelEntry.Text, w, h, q)
			switch grainSelect.Selected {
			case "Horizontal":
				sheet.Grain = model.GrainHorizontal
			case "Vertical":
				sheet.Grain = model.GrainVertical
			}
			sheet.Material = strings.TrimSpace(stockMaterialEntry.Text)
			sheet.PricePerSheet, _ = strconv.ParseFloat(priceEntry.Text, 64)
			a.project.Stocks = append(a.project.Stocks, sheet)
			a.refreshStockList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 500))
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

	grainSelect := widget.NewSelect([]string{"None", "Horizontal", "Vertical"}, nil)
	grainSelect.SetSelected(s.Grain.String())

	editStockMaterialEntry := widget.NewEntry()
	editStockMaterialEntry.SetPlaceHolder("e.g., Plywood, MDF (optional)")
	editStockMaterialEntry.SetText(s.Material)

	priceEntry := widget.NewEntry()
	priceEntry.SetText(fmt.Sprintf("%.2f", s.PricePerSheet))

	form := dialog.NewForm("Edit Stock Sheet", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Quantity", qtyEntry),
			widget.NewFormItem("Grain Direction", grainSelect),
			widget.NewFormItem("Material", editStockMaterialEntry),
			widget.NewFormItem("Price per Sheet", priceEntry),
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
			switch grainSelect.Selected {
			case "Horizontal":
				a.project.Stocks[idx].Grain = model.GrainHorizontal
			case "Vertical":
				a.project.Stocks[idx].Grain = model.GrainVertical
			default:
				a.project.Stocks[idx].Grain = model.GrainNone
			}
			a.project.Stocks[idx].Material = strings.TrimSpace(editStockMaterialEntry.Text)
			a.project.Stocks[idx].PricePerSheet, _ = strconv.ParseFloat(priceEntry.Text, 64)
			a.refreshStockList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 400))
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

	optimizeToolpathCheck := widget.NewCheck("", func(b bool) { s.OptimizeToolpath = b })
	optimizeToolpathCheck.Checked = s.OptimizeToolpath

	cncSection := widget.NewCard("CNC / GCode", "", container.NewGridWithColumns(2,
		widget.NewLabel("Load Tool Profile"), a.buildToolProfileSelector(),
		widget.NewLabel("GCode Profile"), a.buildProfileSelector(),
		widget.NewLabel("Tool Diameter (mm)"), floatEntry(&s.ToolDiameter),
		widget.NewLabel("Feed Rate (mm/min)"), floatEntry(&s.FeedRate),
		widget.NewLabel("Plunge Rate (mm/min)"), floatEntry(&s.PlungeRate),
		widget.NewLabel("Spindle Speed (RPM)"), intEntry(&s.SpindleSpeed),
		widget.NewLabel("Safe Z Height (mm)"), floatEntry(&s.SafeZ),
		widget.NewLabel("Material Thickness (mm)"), floatEntry(&s.CutDepth),
		widget.NewLabel("Pass Depth (mm)"), floatEntry(&s.PassDepth),
		widget.NewLabel("Optimize Toolpath Order"), optimizeToolpathCheck,
	))

	leadInOutSection := widget.NewCard("Lead-In / Lead-Out Arcs", "Arc approach and exit for smoother cuts", container.NewGridWithColumns(2,
		widget.NewLabel("Lead-In Radius (mm)"), floatEntry(&s.LeadInRadius),
		widget.NewLabel("Lead-Out Radius (mm)"), floatEntry(&s.LeadOutRadius),
		widget.NewLabel("Approach Angle (degrees)"), floatEntry(&s.LeadInAngle),
	))

	plungeTypeSelect := widget.NewSelect(model.PlungeTypeOptions(), func(selected string) {
		s.PlungeType = model.PlungeTypeFromString(selected)
	})
	plungeTypeSelect.SetSelected(s.PlungeType.String())

	plungeSection := widget.NewCard("Plunge Entry Strategy", "How the tool enters the material", container.NewGridWithColumns(2,
		widget.NewLabel("Plunge Type"), plungeTypeSelect,
		widget.NewLabel("Ramp Angle (degrees)"), floatEntry(&s.RampAngle),
		widget.NewLabel("Helix Diameter (mm)"), floatEntry(&s.HelixDiameter),
		widget.NewLabel("Helix Depth/Rev (%)"), floatEntry(&s.HelixRevPercent),
	))

	cornerOvercutSelect := widget.NewSelect(model.CornerOvercutOptions(), func(selected string) {
		s.CornerOvercut = model.CornerOvercutFromString(selected)
	})
	cornerOvercutSelect.SetSelected(s.CornerOvercut.String())

	cornerSection := widget.NewCard("Corner Overcuts", "Relief cuts for square interior corners", container.NewGridWithColumns(2,
		widget.NewLabel("Corner Type"), cornerOvercutSelect,
	))

	onionSkinCheck := widget.NewCheck("", func(b bool) { s.OnionSkinEnabled = b })
	onionSkinCheck.Checked = s.OnionSkinEnabled
	onionCleanupCheck := widget.NewCheck("", func(b bool) { s.OnionSkinCleanup = b })
	onionCleanupCheck.Checked = s.OnionSkinCleanup

	onionSkinSection := widget.NewCard("Onion Skinning", "Leave thin layer on final pass to prevent part movement", container.NewGridWithColumns(2,
		widget.NewLabel("Enable Onion Skin"), onionSkinCheck,
		widget.NewLabel("Skin Thickness (mm)"), floatEntry(&s.OnionSkinDepth),
		widget.NewLabel("Generate Cleanup Pass"), onionCleanupCheck,
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

	clampZoneSection := a.buildClampZoneSection()

	return container.NewVScroll(container.NewVBox(
		optimizerSection,
		cncSection,
		plungeSection,
		leadInOutSection,
		cornerSection,
		onionSkinSection,
		stockTabSection,
		clampZoneSection,
	))
}

// clampZoneListContainer holds the dynamic clamp zone list for refresh.
var clampZoneListContainer *fyne.Container

// buildClampZoneSection creates the settings card for clamp/fixture exclusion zones.
func (a *App) buildClampZoneSection() fyne.CanvasObject {
	clampZoneListContainer = container.NewVBox()
	a.refreshClampZoneList()

	addBtn := widget.NewButtonWithIcon("Add Clamp Zone", theme.ContentAddIcon(), func() {
		a.showAddClampZoneDialog()
	})

	return widget.NewCard("Fixture / Clamp Zones",
		"Define exclusion zones where clamps or fixtures are placed on the stock sheet",
		container.NewVBox(
			container.NewHBox(layout.NewSpacer(), addBtn),
			clampZoneListContainer,
		),
	)
}

// refreshClampZoneList rebuilds the clamp zone list display.
func (a *App) refreshClampZoneList() {
	if clampZoneListContainer == nil {
		return
	}
	clampZoneListContainer.RemoveAll()

	zones := a.project.Settings.ClampZones
	if len(zones) == 0 {
		clampZoneListContainer.Add(widget.NewLabel("No clamp zones defined. Parts can be placed anywhere on the sheet."))
		return
	}

	header := container.NewGridWithColumns(7,
		widget.NewLabelWithStyle("Label", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("X (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Y (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Width (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Height (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Z Height (mm)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
	)
	clampZoneListContainer.Add(header)
	clampZoneListContainer.Add(widget.NewSeparator())

	for i := range zones {
		idx := i
		cz := zones[idx]
		row := container.NewGridWithColumns(7,
			widget.NewLabel(cz.Label),
			widget.NewLabel(fmt.Sprintf("%.0f", cz.X)),
			widget.NewLabel(fmt.Sprintf("%.0f", cz.Y)),
			widget.NewLabel(fmt.Sprintf("%.0f", cz.Width)),
			widget.NewLabel(fmt.Sprintf("%.0f", cz.Height)),
			widget.NewLabel(fmt.Sprintf("%.1f", cz.ZHeight)),
			widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				a.project.Settings.ClampZones = append(
					a.project.Settings.ClampZones[:idx],
					a.project.Settings.ClampZones[idx+1:]...,
				)
				a.refreshClampZoneList()
			}),
		)
		clampZoneListContainer.Add(row)
	}
}

// showAddClampZoneDialog shows a form to define a new clamp/fixture zone.
func (a *App) showAddClampZoneDialog() {
	labelEntry := widget.NewEntry()
	labelEntry.SetText(fmt.Sprintf("Clamp %d", len(a.project.Settings.ClampZones)+1))

	xEntry := widget.NewEntry()
	xEntry.SetPlaceHolder("X from left edge (mm)")
	xEntry.SetText("0")

	yEntry := widget.NewEntry()
	yEntry.SetPlaceHolder("Y from top edge (mm)")
	yEntry.SetText("0")

	wEntry := widget.NewEntry()
	wEntry.SetPlaceHolder("Width (mm)")
	wEntry.SetText("50")

	hEntry := widget.NewEntry()
	hEntry.SetPlaceHolder("Height (mm)")
	hEntry.SetText("50")

	zEntry := widget.NewEntry()
	zEntry.SetPlaceHolder("Height above stock (mm)")
	zEntry.SetText("25")

	// Preset clamp positions for common setups
	presetSelect := widget.NewSelect([]string{
		"Custom",
		"Front-Left Corner (50x50)",
		"Front-Right Corner (50x50)",
		"Back-Left Corner (50x50)",
		"Back-Right Corner (50x50)",
		"Center-Left (50x50)",
		"Center-Right (50x50)",
	}, func(selected string) {
		stockW := 2440.0
		stockH := 1220.0
		if len(a.project.Stocks) > 0 {
			stockW = a.project.Stocks[0].Width
			stockH = a.project.Stocks[0].Height
		}
		switch selected {
		case "Front-Left Corner (50x50)":
			xEntry.SetText("0")
			yEntry.SetText(fmt.Sprintf("%.0f", stockH-50))
			wEntry.SetText("50")
			hEntry.SetText("50")
			labelEntry.SetText("Front-Left Clamp")
		case "Front-Right Corner (50x50)":
			xEntry.SetText(fmt.Sprintf("%.0f", stockW-50))
			yEntry.SetText(fmt.Sprintf("%.0f", stockH-50))
			wEntry.SetText("50")
			hEntry.SetText("50")
			labelEntry.SetText("Front-Right Clamp")
		case "Back-Left Corner (50x50)":
			xEntry.SetText("0")
			yEntry.SetText("0")
			wEntry.SetText("50")
			hEntry.SetText("50")
			labelEntry.SetText("Back-Left Clamp")
		case "Back-Right Corner (50x50)":
			xEntry.SetText(fmt.Sprintf("%.0f", stockW-50))
			yEntry.SetText("0")
			wEntry.SetText("50")
			hEntry.SetText("50")
			labelEntry.SetText("Back-Right Clamp")
		case "Center-Left (50x50)":
			xEntry.SetText("0")
			yEntry.SetText(fmt.Sprintf("%.0f", stockH/2-25))
			wEntry.SetText("50")
			hEntry.SetText("50")
			labelEntry.SetText("Center-Left Clamp")
		case "Center-Right (50x50)":
			xEntry.SetText(fmt.Sprintf("%.0f", stockW-50))
			yEntry.SetText(fmt.Sprintf("%.0f", stockH/2-25))
			wEntry.SetText("50")
			hEntry.SetText("50")
			labelEntry.SetText("Center-Right Clamp")
		}
	})
	presetSelect.PlaceHolder = "Select a preset position..."

	form := dialog.NewForm("Add Clamp Zone", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Preset", presetSelect),
			widget.NewFormItem("Label", labelEntry),
			widget.NewFormItem("X Position (mm)", xEntry),
			widget.NewFormItem("Y Position (mm)", yEntry),
			widget.NewFormItem("Width (mm)", wEntry),
			widget.NewFormItem("Height (mm)", hEntry),
			widget.NewFormItem("Z Height (mm)", zEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			x, _ := strconv.ParseFloat(xEntry.Text, 64)
			y, _ := strconv.ParseFloat(yEntry.Text, 64)
			w, _ := strconv.ParseFloat(wEntry.Text, 64)
			h, _ := strconv.ParseFloat(hEntry.Text, 64)
			z, _ := strconv.ParseFloat(zEntry.Text, 64)

			if w <= 0 || h <= 0 {
				dialog.ShowError(fmt.Errorf("clamp zone width and height must be > 0"), a.window)
				return
			}

			zone := model.ClampZone{
				Label:   labelEntry.Text,
				X:       x,
				Y:       y,
				Width:   w,
				Height:  h,
				ZHeight: z,
			}
			a.project.Settings.ClampZones = append(a.project.Settings.ClampZones, zone)
			a.refreshClampZoneList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 520))
	form.Show()
}

func (a *App) buildProfileSelector() fyne.CanvasObject {
	profileNames := model.GetProfileNames()
	a.profileSelector = widget.NewSelect(profileNames, func(selected string) {
		a.project.Settings.GCodeProfile = selected
	})
	a.profileSelector.SetSelected(a.project.Settings.GCodeProfile)

	manageBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		a.showProfileManager()
	})

	return container.NewBorder(nil, nil, nil, manageBtn, a.profileSelector)
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

	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		a.resultContainer.Add(widget.NewLabel("No results yet. Add parts and stock, then click Optimize."))
		a.resultContainer.Refresh()
		return
	}

	// Action buttons toolbar
	simulateBtn := widget.NewButtonWithIcon("Simulate GCode", theme.MediaPlayIcon(), func() {
		a.previewGCode()
	})
	exportBtn := widget.NewButtonWithIcon("Export GCode", theme.DocumentSaveIcon(), func() {
		a.exportGCode()
	})
	saveOffcutsBtn := widget.NewButtonWithIcon("Save Offcuts to Inventory", theme.ContentAddIcon(), func() {
		a.saveOffcutsToInventory()
	})
	labelsBtn := widget.NewButtonWithIcon("Generate Labels", theme.ListIcon(), func() {
		a.exportLabels()
	})
	toolbar := container.NewHBox(layout.NewSpacer(), saveOffcutsBtn, labelsBtn, simulateBtn, exportBtn)

	// Build both cut layout and inline simulation views
	sheetResults := widgets.RenderSheetResults(a.project.Result, a.project.Settings, a.project.Parts)

	// Inline simulation viewport for the first sheet (most common use case)
	gen := gcode.New(a.project.Settings)
	var inlineSimItems []fyne.CanvasObject

	legendLabel := widget.NewLabelWithStyle("Simulation Legend:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	legendText := widget.NewLabel("Green = Completed  |  Dim = Remaining  |  Red circle = Tool position")
	inlineSimItems = append(inlineSimItems, container.NewHBox(legendLabel, legendText), widget.NewSeparator())

	for i, sheet := range a.project.Result.Sheets {
		gcodeStr := gen.GenerateSheet(sheet, i+1)
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Sheet %d: %s (%.0f x %.0f)",
				i+1, sheet.Stock.Label, sheet.Stock.Width, sheet.Stock.Height),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true},
		)
		simView := widgets.RenderGCodeSimulation(sheet, a.project.Settings, gcodeStr)
		inlineSimItems = append(inlineSimItems, header, simView, widget.NewSeparator())
	}
	inlineSim := container.NewVScroll(container.NewVBox(inlineSimItems...))

	// Create tabs for Cut Layout vs Simulation viewport
	viewTabs := container.NewAppTabs(
		container.NewTabItem("Cut Layout", sheetResults),
		container.NewTabItem("Simulation", inlineSim),
	)
	viewTabs.SetTabLocation(container.TabLocationBottom)

	// Use Border layout: toolbar pinned at top, tabbed views fill remaining space
	a.resultContainer.Add(container.NewBorder(
		container.NewVBox(toolbar, widget.NewSeparator()),
		nil, nil, nil,
		viewTabs,
	))
	a.resultContainer.Refresh()
}

func (a *App) previewGCode() {
	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		dialog.ShowInformation("No results", "Run the optimizer first before previewing GCode.", a.window)
		return
	}

	gen := gcode.New(a.project.Settings)

	var previewItems []fyne.CanvasObject

	// Legend header
	legendLabel := widget.NewLabelWithStyle("Legend:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	legendText := widget.NewRichTextFromMarkdown(
		"**Green** = Completed cuts  |  **Dim blue** = Remaining cuts  |  **Red circle** = Tool position  |  **Orange** = Tab positions")
	previewItems = append(previewItems, container.NewHBox(legendLabel, legendText), widget.NewSeparator())

	for i, sheet := range a.project.Result.Sheets {
		gcodeStr := gen.GenerateSheet(sheet, i+1)

		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Sheet %d: %s (%.0f x %.0f) — Toolpath Simulation",
				i+1, sheet.Stock.Label, sheet.Stock.Width, sheet.Stock.Height),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true},
		)

		// Use simulation view with slider, play/pause, step, and speed controls
		simView := widgets.RenderGCodeSimulation(sheet, a.project.Settings, gcodeStr)

		previewItems = append(previewItems, header, simView, widget.NewSeparator())
	}

	content := container.NewVScroll(container.NewVBox(previewItems...))
	content.SetMinSize(fyne.NewSize(750, 550))

	d := dialog.NewCustom("GCode Toolpath Simulation", "Close", content, a.window)
	d.Resize(fyne.NewSize(850, 650))
	d.Show()
}

// showPurchasingCalculator displays a dialog that calculates how many sheets to purchase.
func (a *App) showPurchasingCalculator() {
	if len(a.project.Parts) == 0 {
		dialog.ShowInformation("No Parts", "Add parts to the project first.", a.window)
		return
	}

	// Default sheet size from first stock or inventory
	defaultW := 2440.0
	defaultH := 1220.0
	defaultPrice := 0.0
	if len(a.project.Stocks) > 0 {
		defaultW = a.project.Stocks[0].Width
		defaultH = a.project.Stocks[0].Height
		defaultPrice = a.project.Stocks[0].PricePerSheet
	}

	sheetWidthEntry := widget.NewEntry()
	sheetWidthEntry.SetText(fmt.Sprintf("%.0f", defaultW))

	sheetHeightEntry := widget.NewEntry()
	sheetHeightEntry.SetText(fmt.Sprintf("%.0f", defaultH))

	wasteEntry := widget.NewEntry()
	wasteEntry.SetText("15")

	priceEntry := widget.NewEntry()
	priceEntry.SetText(fmt.Sprintf("%.2f", defaultPrice))

	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	calculateBtn := widget.NewButton("Calculate", func() {
		sw, _ := strconv.ParseFloat(sheetWidthEntry.Text, 64)
		sh, _ := strconv.ParseFloat(sheetHeightEntry.Text, 64)
		waste, _ := strconv.ParseFloat(wasteEntry.Text, 64)
		price, _ := strconv.ParseFloat(priceEntry.Text, 64)

		if sw <= 0 || sh <= 0 {
			resultLabel.SetText("Sheet dimensions must be > 0")
			return
		}

		est := model.CalculatePurchaseEstimate(a.project.Parts, sw, sh,
			a.project.Settings.KerfWidth, waste, price)

		var text strings.Builder
		text.WriteString(fmt.Sprintf("Total part area: %.0f sq mm (%.2f board feet)\n", est.TotalPartArea, est.TotalBoardFeet))
		text.WriteString(fmt.Sprintf("Sheet area: %.0f sq mm (%.0f x %.0f)\n", est.SheetArea, sw, sh))
		text.WriteString(fmt.Sprintf("Kerf width: %.1f mm\n\n", est.KerfWidth))
		text.WriteString(fmt.Sprintf("Sheets needed (minimum): %d\n", est.SheetsNeededMin))
		text.WriteString(fmt.Sprintf("Sheets recommended (with %.0f%% waste): %d\n", waste, est.SheetsWithWaste))
		if price > 0 {
			text.WriteString(fmt.Sprintf("\nEstimated cost: %.2f (%d sheets x %.2f/sheet)\n",
				est.EstimatedCost, est.SheetsWithWaste, price))
		}

		resultLabel.SetText(text.String())
	})
	calculateBtn.Importance = widget.HighImportance

	// Stock preset dropdown for quick selection
	presetNames := a.inventory.StockNames()
	presetSelect := widget.NewSelect(presetNames, func(selected string) {
		preset := a.inventory.FindStockByName(selected)
		if preset == nil {
			return
		}
		sheetWidthEntry.SetText(fmt.Sprintf("%.0f", preset.Width))
		sheetHeightEntry.SetText(fmt.Sprintf("%.0f", preset.Height))
		if preset.PricePerSheet > 0 {
			priceEntry.SetText(fmt.Sprintf("%.2f", preset.PricePerSheet))
		}
	})
	presetSelect.PlaceHolder = "Load from stock inventory..."

	content := container.NewVBox(
		widget.NewLabelWithStyle("Purchasing Calculator", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("Parts in project: %d types, %d total pieces",
			len(a.project.Parts), countTotalParts(a.project.Parts))),
		widget.NewSeparator(),
		widget.NewFormItem("Stock Preset", presetSelect).Widget,
		container.NewGridWithColumns(2,
			widget.NewLabel("Sheet Width (mm)"), sheetWidthEntry,
			widget.NewLabel("Sheet Height (mm)"), sheetHeightEntry,
			widget.NewLabel("Waste Factor (%)"), wasteEntry,
			widget.NewLabel("Price per Sheet"), priceEntry,
		),
		calculateBtn,
		widget.NewSeparator(),
		resultLabel,
	)

	d := dialog.NewCustom("Purchasing Calculator", "Close", content, a.window)
	d.Resize(fyne.NewSize(500, 550))
	d.Show()
}

// countTotalParts sums up all part quantities.
func countTotalParts(parts []model.Part) int {
	total := 0
	for _, p := range parts {
		total += p.Quantity
	}
	return total
}

// saveOffcutsToInventory detects usable offcuts from the current result and saves them
// as stock presets in the inventory for future projects.
func (a *App) saveOffcutsToInventory() {
	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		dialog.ShowInformation("No Results", "Run the optimizer first to detect offcuts.", a.window)
		return
	}

	offcuts := model.DetectAllOffcuts(*a.project.Result, a.project.Settings.KerfWidth)
	if len(offcuts) == 0 {
		dialog.ShowInformation("No Offcuts", "No usable remnant areas were detected.", a.window)
		return
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Found %d usable offcut(s):\n\n", len(offcuts)))
	for i, o := range offcuts {
		summary.WriteString(fmt.Sprintf("%d. Sheet %d (%s): %.0f x %.0f mm",
			i+1, o.SheetIndex+1, o.SheetLabel, o.Width, o.Height))
		if o.PricePerSheet > 0 {
			summary.WriteString(fmt.Sprintf(" (~%.2f value)", o.PricePerSheet))
		}
		summary.WriteString("\n")
	}
	summary.WriteString("\nSave these as stock presets in your inventory?")

	dialog.ShowConfirm("Save Offcuts to Inventory", summary.String(), func(ok bool) {
		if !ok {
			return
		}
		count := 0
		for _, o := range offcuts {
			sheet := o.ToStockSheet()
			preset := model.NewStockPresetWithPrice(sheet.Label, sheet.Width, sheet.Height, "Offcut", sheet.PricePerSheet)
			a.inventory.Stocks = append(a.inventory.Stocks, preset)
			count++
		}
		a.saveInventory()
		dialog.ShowInformation("Offcuts Saved",
			fmt.Sprintf("%d offcut(s) added to stock inventory.", count), a.window)
	}, a.window)
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

func (a *App) exportLabels() {
	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		dialog.ShowInformation("No results", "Run the optimizer first before generating labels.", a.window)
		return
	}

	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		writer.Close()
		path := writer.URI().Path()
		if exportErr := export.ExportLabels(path, *a.project.Result); exportErr != nil {
			dialog.ShowError(exportErr, a.window)
		} else {
			dialog.ShowInformation("Export Complete",
				fmt.Sprintf("QR code labels saved to %s", path), a.window)
		}
	}, a.window)
	d.SetFileName("part-labels.pdf")
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
