package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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

	// Template management
	templates model.TemplateStore

	// Auto-optimization
	optimizeTimer *time.Timer
	optimizeMu    sync.Mutex

	// UI references for dynamic updates
	partsContainer  *fyne.Container
	stockContainer  *fyne.Container
	resultContainer *fyne.Container
	profileSelector *widget.Select

	// New UI references for OrcaSlicer layout
	sheetCanvas       *widgets.SheetCanvas
	sheetSelectorBox  *fyne.Container
	statusLabel       *widget.Label
	gcodePreviewBox   *fyne.Container
	selectedSheetIdx  int
	settingsContainer *fyne.Container

	// Dust shoe collision results from last optimization
	lastCollisions []model.DustShoeCollision
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
	app.loadTemplates()
	app.applyTheme()
	return app
}

// applyTheme sets the compact SlabCut theme with the appropriate light/dark variant.
func (a *App) applyTheme() {
	var variant fyne.ThemeVariant
	switch a.config.Theme {
	case "light":
		variant = theme.VariantLight
	case "dark":
		variant = theme.VariantDark
	default:
		variant = theme.VariantDark // default to system (use dark as fallback)
	}
	a.app.Settings().SetTheme(NewSlabCutThemeWithVariant(variant))
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

// loadTemplates loads project templates from disk on startup.
func (a *App) loadTemplates() {
	store, err := project.LoadDefaultTemplates()
	if err != nil {
		fmt.Printf("Warning: could not load templates: %v\n", err)
		a.templates = model.NewTemplateStore()
		return
	}
	a.templates = store
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

// ─── Menus ──────────────────────────────────────────────────

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
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Share Project...", func() {
			a.shareProject()
		}),
		fyne.NewMenuItem("Import Shared Project...", func() {
			a.importSharedProject()
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
			a.scheduleOptimize()
		}),
		fyne.NewMenuItem("Clear All Stock Sheets", func() {
			a.saveState("Clear All Stock Sheets")
			a.project.Stocks = nil
			a.refreshStockList()
			a.scheduleOptimize()
		}),
	)

	// Tools Menu
	toolsMenu := fyne.NewMenu("Tools",
		fyne.NewMenuItem("Force Re-Optimize", func() {
			a.runOptimize()
		}),
		fyne.NewMenuItem("Compare Settings...", func() {
			a.showCompareDialog()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Purchasing Calculator...", func() {
			a.showPurchasingCalculator()
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
		fyne.NewMenuItem("Project Templates...", func() {
			a.showTemplateManager()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Advanced Settings...", func() {
			a.showAdvancedSettingsDialog()
		}),
		fyne.NewMenuItem("GCode Profiles...", func() {
			a.showProfileManager()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Import/Export Data...", func() {
			a.showImportExportDialog()
		}),
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

// ─── Build (OrcaSlicer-Inspired Layout) ─────────────────────

// Build constructs the full UI and returns the root container.
func (a *App) Build() fyne.CanvasObject {
	// Tab 1: Layout Editor (three-pane)
	layoutTab := container.NewTabItem("Layout Editor", a.buildLayoutEditor())

	// Tab 2: GCode Preview
	gcodeTab := container.NewTabItem("GCode Preview", a.buildGCodePreviewTab())

	a.tabs = container.NewAppTabs(layoutTab, gcodeTab)
	a.tabs.SetTabLocation(container.TabLocationTop)

	a.registerShortcuts()

	// Status bar
	a.statusLabel = widget.NewLabel("No optimization yet")

	versionLabel := widget.NewLabelWithStyle(
		"SlabCut "+version.Short(),
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)

	exportGCodeBtn := widget.NewButtonWithIcon("Export GCode", theme.DocumentSaveIcon(), func() {
		a.exportGCode()
	})
	exportPDFBtn := widget.NewButtonWithIcon("Export PDF", theme.DocumentSaveIcon(), func() {
		a.exportPDF()
	})

	statusBar := container.NewHBox(
		versionLabel,
		layout.NewSpacer(),
		a.statusLabel,
		layout.NewSpacer(),
		exportGCodeBtn,
		exportPDFBtn,
	)

	return container.NewBorder(nil, statusBar, nil, nil, a.tabs)
}

// buildLayoutEditor creates the three-pane Layout Editor tab.
func (a *App) buildLayoutEditor() fyne.CanvasObject {
	leftPanel := a.buildQuickSettingsPanel()
	centerPanel := a.buildCenterCanvas()
	rightPanel := a.buildPartsStockPanel()

	// Left + Center split
	leftCenter := container.NewHSplit(leftPanel, centerPanel)
	leftCenter.SetOffset(0.22)

	// (Left+Center) + Right split
	threePanes := container.NewHSplit(leftCenter, rightPanel)
	threePanes.SetOffset(0.75)

	return threePanes
}

// ─── Left Panel: Quick Settings ─────────────────────────────

func (a *App) buildQuickSettingsPanel() fyne.CanvasObject {
	s := &a.project.Settings

	// Helper to create float entry with scheduleOptimize on change
	floatEntry := func(val *float64) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(fmt.Sprintf("%.1f", *val))
		e.OnChanged = func(text string) {
			if v, err := strconv.ParseFloat(text, 64); err == nil {
				*val = v
				a.scheduleOptimize()
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
				a.scheduleOptimize()
			}
		}
		return e
	}

	// --- Tool Section ---
	toolNames := a.inventory.ToolNames()
	var toolProfileSelect *widget.Select
	if len(toolNames) > 0 {
		toolProfileSelect = widget.NewSelect(toolNames, func(selected string) {
			tool := a.inventory.FindToolByName(selected)
			if tool == nil {
				return
			}
			tool.ApplyToSettings(&a.project.Settings)
			// Rebuild the quick settings panel to reflect new values
			a.rebuildSettingsPanel()
			a.scheduleOptimize()
		})
		toolProfileSelect.PlaceHolder = "Load Tool Profile..."
	}

	toolDiameterEntry := floatEntry(&s.ToolDiameter)
	feedRateEntry := floatEntry(&s.FeedRate)
	plungeRateEntry := floatEntry(&s.PlungeRate)
	rpmEntry := intEntry(&s.SpindleSpeed)

	toolContent := container.NewVBox()
	if toolProfileSelect != nil {
		toolContent.Add(container.NewGridWithColumns(2,
			widget.NewLabel("Tool Profile"), toolProfileSelect,
		))
	}
	toolContent.Add(container.NewGridWithColumns(2,
		widget.NewLabel("Diameter (mm)"), toolDiameterEntry,
		widget.NewLabel("Feed Rate (mm/min)"), feedRateEntry,
		widget.NewLabel("Plunge Rate (mm/min)"), plungeRateEntry,
		widget.NewLabel("RPM"), rpmEntry,
	))

	// --- Material Section ---
	stockNames := a.inventory.StockNames()
	var stockPresetSelect *widget.Select
	if len(stockNames) > 0 {
		stockPresetSelect = widget.NewSelect(stockNames, func(selected string) {
			preset := a.inventory.FindStockByName(selected)
			if preset == nil {
				return
			}
			// Apply stock preset values to kerf/thickness context
			// (stock sheets are added in the right panel)
		})
		stockPresetSelect.PlaceHolder = "Load Stock Preset..."
	}

	kerfEntry := floatEntry(&s.KerfWidth)
	edgeTrimEntry := floatEntry(&s.EdgeTrim)

	materialContent := container.NewVBox()
	if stockPresetSelect != nil {
		materialContent.Add(container.NewGridWithColumns(2,
			widget.NewLabel("Stock Preset"), stockPresetSelect,
		))
	}
	materialContent.Add(container.NewGridWithColumns(2,
		widget.NewLabel("Kerf Width (mm)"), kerfEntry,
		widget.NewLabel("Edge Trim (mm)"), edgeTrimEntry,
	))

	// --- Cutting Section ---
	safeZEntry := floatEntry(&s.SafeZ)
	cutDepthEntry := floatEntry(&s.CutDepth)
	passDepthEntry := floatEntry(&s.PassDepth)
	tabsCheck := widget.NewCheck("Enable Tabs", func(b bool) {
		s.StockTabs.Enabled = b
		a.scheduleOptimize()
	})
	tabsCheck.Checked = s.StockTabs.Enabled

	cuttingContent := container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Safe Z (mm)"), safeZEntry,
			widget.NewLabel("Cut Depth (mm)"), cutDepthEntry,
			widget.NewLabel("Pass Depth (mm)"), passDepthEntry,
		),
		tabsCheck,
	)

	// --- Optimizer Section ---
	algorithmSelect := widget.NewSelect([]string{"Guillotine (Fast)", "Genetic Algorithm (Better)"}, func(selected string) {
		switch selected {
		case "Genetic Algorithm (Better)":
			s.Algorithm = model.AlgorithmGenetic
		default:
			s.Algorithm = model.AlgorithmGuillotine
		}
		a.scheduleOptimize()
	})
	switch s.Algorithm {
	case model.AlgorithmGenetic:
		algorithmSelect.SetSelected("Genetic Algorithm (Better)")
	default:
		algorithmSelect.SetSelected("Guillotine (Fast)")
	}

	guillotineCheck := widget.NewCheck("Guillotine Cuts Only", func(b bool) {
		s.GuillotineOnly = b
		a.scheduleOptimize()
	})
	guillotineCheck.Checked = s.GuillotineOnly

	optimizerContent := container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Algorithm"), algorithmSelect,
		),
		guillotineCheck,
	)

	// Build accordion
	toolItem := widget.NewAccordionItem("Tool", toolContent)
	materialItem := widget.NewAccordionItem("Material", materialContent)
	cuttingItem := widget.NewAccordionItem("Cutting", cuttingContent)
	optimizerItem := widget.NewAccordionItem("Optimizer", optimizerContent)

	accordion := widget.NewAccordion(toolItem, materialItem, cuttingItem, optimizerItem)
	accordion.MultiOpen = true
	// Open all sections by default
	accordion.Open(0)
	accordion.Open(1)
	accordion.Open(2)
	accordion.Open(3)

	// Advanced settings button at bottom
	advancedBtn := newIconButtonWithTooltip(theme.SettingsIcon(), "Open Advanced Settings", func() {
		a.showAdvancedSettingsDialog()
	})
	advancedBtn.SetText("Advanced Settings...")

	// Store reference so we can rebuild
	a.settingsContainer = container.NewVBox(accordion, advancedBtn)

	return container.NewBorder(nil, nil, nil, nil,
		container.NewVScroll(a.settingsContainer),
	)
}

// rebuildSettingsPanel rebuilds the quick settings panel when values change externally
// (e.g., loading a tool profile).
func (a *App) rebuildSettingsPanel() {
	if a.tabs != nil && len(a.tabs.Items) > 0 {
		a.tabs.Items[0].Content = a.buildLayoutEditor()
		a.tabs.Refresh()
	}
}

// ─── Center Panel: Interactive Sheet Canvas ─────────────────

func (a *App) buildCenterCanvas() fyne.CanvasObject {
	// Create an empty sheet canvas (will be populated after optimization)
	emptySheet := model.SheetResult{
		Stock: model.StockSheet{Width: 2440, Height: 1220, Label: "Empty"},
	}
	a.sheetCanvas = widgets.NewSheetCanvas(emptySheet, a.project.Settings, 600, 400)

	// Sheet selector buttons
	a.sheetSelectorBox = container.NewHBox()
	a.refreshSheetSelector()

	// Zoom controls
	zoomInBtn := newIconButtonWithTooltip(theme.ZoomInIcon(), "Zoom In", func() {
		a.sheetCanvas.SetZoomCentered(a.sheetCanvas.ZoomLevel() * 1.25)
	})
	zoomOutBtn := newIconButtonWithTooltip(theme.ZoomOutIcon(), "Zoom Out", func() {
		a.sheetCanvas.SetZoomCentered(a.sheetCanvas.ZoomLevel() / 1.25)
	})
	resetZoomBtn := newIconButtonWithTooltip(theme.ViewRestoreIcon(), "Reset Zoom", func() {
		a.sheetCanvas.ResetZoom()
	})

	zoomBar := container.NewHBox(
		zoomOutBtn, zoomInBtn, resetZoomBtn,
		layout.NewSpacer(),
	)

	// Empty state or canvas
	canvasArea := container.NewStack(a.sheetCanvas)

	bottomBar := container.NewVBox(
		container.NewHBox(a.sheetSelectorBox),
		zoomBar,
	)

	return container.NewBorder(nil, bottomBar, nil, nil, canvasArea)
}

// refreshSheetSelector updates the sheet selector buttons below the canvas.
func (a *App) refreshSheetSelector() {
	if a.sheetSelectorBox == nil {
		return
	}
	a.sheetSelectorBox.RemoveAll()

	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		a.sheetSelectorBox.Add(widget.NewLabel("No sheets"))
		return
	}

	for i := range a.project.Result.Sheets {
		idx := i
		label := fmt.Sprintf("Sheet %d", idx+1)
		btn := widget.NewButton(label, func() {
			a.selectedSheetIdx = idx
			a.updateCanvasForSheet(idx)
		})
		if idx == a.selectedSheetIdx {
			btn.Importance = widget.HighImportance
		}
		a.sheetSelectorBox.Add(btn)
	}
}

// updateCanvasForSheet switches the SheetCanvas to display the given sheet index.
func (a *App) updateCanvasForSheet(idx int) {
	if a.project.Result == nil || idx >= len(a.project.Result.Sheets) {
		return
	}
	a.selectedSheetIdx = idx
	sheet := a.project.Result.Sheets[idx]
	a.sheetCanvas.SetSheet(sheet, a.project.Settings)
	a.refreshSheetSelector()
}

// ─── Right Panel: Parts + Stock ─────────────────────────────

func (a *App) buildPartsStockPanel() fyne.CanvasObject {
	a.partsContainer = container.NewVBox()
	a.stockContainer = container.NewVBox()
	a.refreshPartsList()
	a.refreshStockList()

	// --- Parts Quick-Add ---
	qaName := widget.NewEntry()
	qaName.SetPlaceHolder("Name")
	qaWidth := widget.NewEntry()
	qaWidth.SetPlaceHolder("W")
	qaHeight := widget.NewEntry()
	qaHeight.SetPlaceHolder("H")
	qaQty := widget.NewEntry()
	qaQty.SetPlaceHolder("Qty")
	qaQty.SetText("1")
	qaGrain := widget.NewSelect([]string{"None", "Horizontal", "Vertical"}, nil)
	qaGrain.SetSelected("None")

	doPartAdd := func() {
		label := qaName.Text
		if label == "" {
			label = fmt.Sprintf("Part %d", len(a.project.Parts)+1)
		}
		w := parseFloat(qaWidth.Text)
		h := parseFloat(qaHeight.Text)
		if w <= 0 || h <= 0 {
			dialog.ShowError(fmt.Errorf("width and height must be positive numbers"), a.window)
			return
		}
		q := parseInt(qaQty.Text)
		if q <= 0 {
			q = 1
		}
		grain := parseGrain(qaGrain.Selected)
		a.saveState("Quick Add Part")
		a.project.Parts = append(a.project.Parts, model.Part{
			Label:    label,
			Width:    w,
			Height:   h,
			Quantity: q,
			Grain:    grain,
		})
		a.refreshPartsList()
		qaName.SetText("")
		qaWidth.SetText("")
		qaHeight.SetText("")
		qaQty.SetText("1")
		qaGrain.SetSelected("None")
		a.window.Canvas().Focus(qaWidth)
		a.scheduleOptimize()
	}

	qaName.OnSubmitted = func(_ string) { doPartAdd() }
	qaWidth.OnSubmitted = func(_ string) { doPartAdd() }
	qaHeight.OnSubmitted = func(_ string) { doPartAdd() }
	qaQty.OnSubmitted = func(_ string) { doPartAdd() }

	partAddBtn := newEnterButton(theme.ContentAddIcon(), doPartAdd)

	partQuickAdd := container.NewVBox(
		container.NewBorder(nil, nil, nil, partAddBtn, qaName),
		container.NewGridWithColumns(4, qaWidth, qaHeight, qaQty, qaGrain),
	)

	addPartMenuBtn := widget.NewButton("More...", nil)
	addPartMenu := fyne.NewMenu("",
		fyne.NewMenuItem("Add Part (detailed)...", func() {
			a.showAddPartDialog()
		}),
		fyne.NewMenuItem("Add from Library...", func() {
			a.showAddFromLibraryDialog()
		}),
		fyne.NewMenuItem("Import from DXF...", func() {
			a.importDXF()
		}),
	)
	addPartMenuBtn.OnTapped = func() {
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(addPartMenuBtn)
		pos.Y += addPartMenuBtn.Size().Height
		widget.ShowPopUpMenuAtPosition(addPartMenu, a.window.Canvas(), pos)
	}

	partsHeader := container.NewHBox(
		widget.NewLabelWithStyle(fmt.Sprintf("Parts (%d)", len(a.project.Parts)),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		addPartMenuBtn,
	)

	partsContent := container.NewVBox(
		partsHeader,
		partQuickAdd,
		widget.NewSeparator(),
		container.NewVScroll(a.partsContainer),
	)

	// --- Stock Quick-Add ---
	sqaName := widget.NewEntry()
	sqaName.SetPlaceHolder("Name")
	sqaName.SetText("Plywood")
	sqaWidth := widget.NewEntry()
	sqaWidth.SetPlaceHolder("W")
	sqaHeight := widget.NewEntry()
	sqaHeight.SetPlaceHolder("H")
	sqaThick := widget.NewEntry()
	sqaThick.SetPlaceHolder("Thick")
	sqaThick.SetText("18")
	sqaQty := widget.NewEntry()
	sqaQty.SetPlaceHolder("Qty")
	sqaQty.SetText("1")

	doStockAdd := func() {
		label := sqaName.Text
		if label == "" {
			label = fmt.Sprintf("Sheet %d", len(a.project.Stocks)+1)
		}
		w := parseFloat(sqaWidth.Text)
		h := parseFloat(sqaHeight.Text)
		if w <= 0 || h <= 0 {
			dialog.ShowError(fmt.Errorf("width and height must be positive numbers"), a.window)
			return
		}
		th := parseFloat(sqaThick.Text)
		if th <= 0 {
			th = 18
		}
		q := parseInt(sqaQty.Text)
		if q <= 0 {
			q = 1
		}
		a.saveState("Quick Add Stock")
		a.project.Stocks = append(a.project.Stocks, model.StockSheet{
			Label:     label,
			Width:     w,
			Height:    h,
			Thickness: th,
			Quantity:  q,
			Grain:     model.GrainNone,
		})
		a.refreshStockList()
		sqaName.SetText("Plywood")
		sqaWidth.SetText("")
		sqaHeight.SetText("")
		sqaThick.SetText("18")
		sqaQty.SetText("1")
		a.window.Canvas().Focus(sqaWidth)
		a.scheduleOptimize()
	}

	sqaName.OnSubmitted = func(_ string) { doStockAdd() }
	sqaWidth.OnSubmitted = func(_ string) { doStockAdd() }
	sqaHeight.OnSubmitted = func(_ string) { doStockAdd() }
	sqaThick.OnSubmitted = func(_ string) { doStockAdd() }
	sqaQty.OnSubmitted = func(_ string) { doStockAdd() }

	stockAddBtn := newEnterButton(theme.ContentAddIcon(), doStockAdd)

	stockQuickAdd := container.NewVBox(
		container.NewBorder(nil, nil, nil, stockAddBtn, sqaName),
		container.NewGridWithColumns(4, sqaWidth, sqaHeight, sqaThick, sqaQty),
	)

	addStockMenuBtn := widget.NewButton("More...", nil)
	addStockMenu := fyne.NewMenu("",
		fyne.NewMenuItem("Add Stock (detailed)...", func() {
			a.showAddStockDialog()
		}),
		fyne.NewMenuItem("Add from Inventory...", func() {
			a.showAddStockFromInventory()
		}),
	)
	addStockMenuBtn.OnTapped = func() {
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(addStockMenuBtn)
		pos.Y += addStockMenuBtn.Size().Height
		widget.ShowPopUpMenuAtPosition(addStockMenu, a.window.Canvas(), pos)
	}

	stockHeader := container.NewHBox(
		widget.NewLabelWithStyle(fmt.Sprintf("Stock Sheets (%d)", len(a.project.Stocks)),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		addStockMenuBtn,
	)

	stockContent := container.NewVBox(
		stockHeader,
		stockQuickAdd,
		widget.NewSeparator(),
		container.NewVScroll(a.stockContainer),
	)

	// Build accordion
	partsItem := widget.NewAccordionItem("Parts", partsContent)
	stockItem := widget.NewAccordionItem("Stock Sheets", stockContent)
	rightAccordion := widget.NewAccordion(partsItem, stockItem)
	rightAccordion.MultiOpen = true
	rightAccordion.Open(0)
	rightAccordion.Open(1)

	return container.NewVScroll(rightAccordion)
}

// refreshPartsList rebuilds the parts card list in the right panel.
func (a *App) refreshPartsList() {
	if a.partsContainer == nil {
		return
	}
	a.partsContainer.RemoveAll()

	if len(a.project.Parts) == 0 {
		a.partsContainer.Add(widget.NewLabel("No parts added yet."))
		return
	}

	for i := range a.project.Parts {
		idx := i
		p := a.project.Parts[idx]

		// Part card: name + dimensions on two lines, with edit/delete buttons
		nameLabel := widget.NewLabelWithStyle(p.Label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		detailText := fmt.Sprintf("%.0f x %.0f mm  x%d", p.Width, p.Height, p.Quantity)
		if p.Grain != model.GrainNone {
			detailText += fmt.Sprintf("  Grain: %s", p.Grain.String())
		}
		if p.Material != "" {
			detailText += fmt.Sprintf("  [%s]", p.Material)
		}
		detailLabel := widget.NewLabel(detailText)

		editBtn := newIconButtonWithTooltip(theme.DocumentCreateIcon(), "Edit Part", func() {
			a.showEditPartDialog(idx)
		})
		deleteBtn := newIconButtonWithTooltip(theme.DeleteIcon(), "Delete Part", func() {
			a.saveState("Delete Part")
			a.project.Parts = append(a.project.Parts[:idx], a.project.Parts[idx+1:]...)
			a.refreshPartsList()
			a.scheduleOptimize()
		})
		saveBtn := newIconButtonWithTooltip(theme.DownloadIcon(), "Save to Library", func() {
			a.showSaveToLibraryDialog(a.project.Parts[idx])
		})

		buttons := container.NewHBox(editBtn, saveBtn, deleteBtn)
		topRow := container.NewBorder(nil, nil, nil, buttons, nameLabel)

		card := container.NewVBox(topRow, detailLabel, widget.NewSeparator())
		a.partsContainer.Add(card)
	}
}

// refreshStockList rebuilds the stock card list in the right panel.
func (a *App) refreshStockList() {
	if a.stockContainer == nil {
		return
	}
	a.stockContainer.RemoveAll()

	if len(a.project.Stocks) == 0 {
		a.stockContainer.Add(widget.NewLabel("No stock sheets defined."))
		return
	}

	for i := range a.project.Stocks {
		idx := i
		s := a.project.Stocks[idx]

		nameLabel := widget.NewLabelWithStyle(s.Label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		thicknessVal := s.Thickness
		if thicknessVal <= 0 {
			thicknessVal = 18
		}
		detailText := fmt.Sprintf("%.0f x %.0f mm  %.0fmm thick  x%d", s.Width, s.Height, thicknessVal, s.Quantity)
		if s.Grain != model.GrainNone {
			detailText += fmt.Sprintf("  Grain: %s", s.Grain.String())
		}
		if s.Material != "" {
			detailText += fmt.Sprintf("  [%s]", s.Material)
		}
		detailLabel := widget.NewLabel(detailText)

		editBtn := newIconButtonWithTooltip(theme.DocumentCreateIcon(), "Edit Stock Sheet", func() {
			a.showEditStockDialog(idx)
		})
		deleteBtn := newIconButtonWithTooltip(theme.DeleteIcon(), "Delete Stock Sheet", func() {
			a.saveState("Delete Stock Sheet")
			a.project.Stocks = append(a.project.Stocks[:idx], a.project.Stocks[idx+1:]...)
			a.refreshStockList()
			a.scheduleOptimize()
		})

		buttons := container.NewHBox(editBtn, deleteBtn)
		topRow := container.NewBorder(nil, nil, nil, buttons, nameLabel)

		card := container.NewVBox(topRow, detailLabel, widget.NewSeparator())
		a.stockContainer.Add(card)
	}
}

// ─── GCode Preview Tab ──────────────────────────────────────

func (a *App) buildGCodePreviewTab() fyne.CanvasObject {
	a.gcodePreviewBox = container.NewStack(
		container.NewCenter(
			widget.NewLabel("Run optimization first, then switch here to preview GCode toolpaths."),
		),
	)
	return a.gcodePreviewBox
}

// refreshGCodePreview rebuilds the GCode preview tab content.
func (a *App) refreshGCodePreview() {
	if a.gcodePreviewBox == nil {
		return
	}
	a.gcodePreviewBox.RemoveAll()

	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		a.gcodePreviewBox.Add(container.NewCenter(
			widget.NewLabel("Run optimization first, then switch here to preview GCode toolpaths."),
		))
		a.gcodePreviewBox.Refresh()
		return
	}

	gen := gcode.New(a.project.Settings)
	codes := gen.GenerateAll(*a.project.Result)

	// Sheet selector dropdown
	sheetNames := make([]string, len(a.project.Result.Sheets))
	for i := range a.project.Result.Sheets {
		sheetNames[i] = fmt.Sprintf("Sheet %d", i+1)
	}

	// Profile selector
	profileNames := model.GetProfileNames()
	profileSelect := widget.NewSelect(profileNames, func(selected string) {
		a.project.Settings.GCodeProfile = selected
	})
	profileSelect.SetSelected(a.project.Settings.GCodeProfile)

	// Default to first sheet
	previewIdx := 0
	if previewIdx < len(codes) {
		sheet := a.project.Result.Sheets[previewIdx]
		sim := widgets.RenderGCodeSimulation(sheet, a.project.Settings, codes[previewIdx])

		var sheetSelect *widget.Select
		sheetSelect = widget.NewSelect(sheetNames, func(selected string) {
			for i, name := range sheetNames {
				if name == selected && i < len(codes) {
					a.gcodePreviewBox.RemoveAll()
					newSheet := a.project.Result.Sheets[i]
					newSim := widgets.RenderGCodeSimulation(newSheet, a.project.Settings, codes[i])
					newTopBar := container.NewHBox(
						widget.NewLabel("Sheet:"),
						sheetSelect,
						layout.NewSpacer(),
						widget.NewLabel("GCode Profile:"),
						profileSelect,
					)
					a.gcodePreviewBox.Add(container.NewBorder(newTopBar, nil, nil, nil, newSim))
					a.gcodePreviewBox.Refresh()
					break
				}
			}
		})
		sheetSelect.SetSelected(sheetNames[previewIdx])

		topBar := container.NewHBox(
			widget.NewLabel("Sheet:"),
			sheetSelect,
			layout.NewSpacer(),
			widget.NewLabel("GCode Profile:"),
			profileSelect,
		)

		a.gcodePreviewBox.Add(container.NewBorder(topBar, nil, nil, nil, sim))
	}
	a.gcodePreviewBox.Refresh()
}

// ─── Auto-Optimize ──────────────────────────────────────────

// scheduleOptimize debounces optimization with a 500ms delay.
func (a *App) scheduleOptimize() {
	a.optimizeMu.Lock()
	defer a.optimizeMu.Unlock()
	if a.optimizeTimer != nil {
		a.optimizeTimer.Stop()
	}
	a.optimizeTimer = time.AfterFunc(500*time.Millisecond, func() {
		a.runAutoOptimize()
	})
}

// runAutoOptimize runs the optimizer in a goroutine and updates the UI on the main thread.
func (a *App) runAutoOptimize() {
	if len(a.project.Parts) == 0 || len(a.project.Stocks) == 0 {
		// Clear results if nothing to optimize
		a.project.Result = nil
		a.lastCollisions = nil
		a.updateStatusBar()
		a.refreshSheetSelector()
		if a.sheetCanvas != nil {
			emptySheet := model.SheetResult{
				Stock: model.StockSheet{Width: 2440, Height: 1220, Label: "Empty"},
			}
			a.sheetCanvas.SetSheet(emptySheet, a.project.Settings)
		}
		return
	}

	// Update status
	if a.statusLabel != nil {
		a.statusLabel.SetText("Optimizing...")
	}

	go func() {
		opt := engine.New(a.project.Settings)
		result := opt.Optimize(a.project.Parts, a.project.Stocks)

		// Run dust shoe collision detection
		collisions := gcode.CheckDustShoeCollisions(result, a.project.Settings)

		// Update on UI thread
		a.project.Result = &result
		a.lastCollisions = collisions
		a.updateStatusBar()
		a.refreshSheetSelector()
		a.refreshGCodePreview()

		// Update canvas with first sheet
		if len(result.Sheets) > 0 {
			if a.selectedSheetIdx >= len(result.Sheets) {
				a.selectedSheetIdx = 0
			}
			a.updateCanvasForSheet(a.selectedSheetIdx)
		}
	}()
}

// updateStatusBar updates the center status label with optimization summary.
func (a *App) updateStatusBar() {
	if a.statusLabel == nil {
		return
	}
	if a.project.Result == nil || len(a.project.Result.Sheets) == 0 {
		a.statusLabel.SetText("No optimization yet")
		return
	}

	r := a.project.Result
	text := fmt.Sprintf("%d sheet(s), %.1f%% efficiency", len(r.Sheets), r.TotalEfficiency())
	if len(r.UnplacedParts) > 0 {
		text += fmt.Sprintf(" | %d unplaced!", len(r.UnplacedParts))
	}
	if r.HasPricing() {
		text += fmt.Sprintf(" | Cost: %.2f", r.TotalCost())
	}
	a.statusLabel.SetText(text)
}

// refreshResults is a compatibility shim that triggers the new UI updates.
func (a *App) refreshResults() {
	a.updateStatusBar()
	a.refreshSheetSelector()
	if a.project.Result != nil && len(a.project.Result.Sheets) > 0 {
		if a.selectedSheetIdx >= len(a.project.Result.Sheets) {
			a.selectedSheetIdx = 0
		}
		a.updateCanvasForSheet(a.selectedSheetIdx)
	}
	a.refreshGCodePreview()
}

// ─── History (Undo/Redo) ────────────────────────────────────

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
	a.scheduleOptimize()
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
	a.scheduleOptimize()
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

// ─── Helpers ────────────────────────────────────────────────

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func parseGrain(s string) model.Grain {
	switch s {
	case "Horizontal":
		return model.GrainHorizontal
	case "Vertical":
		return model.GrainVertical
	default:
		return model.GrainNone
	}
}

// enterButton is a button that also responds to Enter/Return key when focused.
type enterButton struct {
	widget.Button
}

func newEnterButton(icon fyne.Resource, tapped func()) *enterButton {
	b := &enterButton{}
	b.SetIcon(icon)
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

func (b *enterButton) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
		if b.OnTapped != nil {
			b.OnTapped()
		}
		return
	}
	b.Button.TypedKey(ev)
}

// ─── Part Dialogs ───────────────────────────────────────────

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
			a.scheduleOptimize()
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
			a.scheduleOptimize()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 420))
	form.Show()
}

// ─── Stock Sheet Dialogs ────────────────────────────────────

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

	thicknessEntry := widget.NewEntry()
	thicknessEntry.SetText("18")

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
			widget.NewFormItem("Thickness (mm)", thicknessEntry),
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
			th, _ := strconv.ParseFloat(thicknessEntry.Text, 64)
			if th <= 0 {
				th = 18
			}
			a.saveState("Add Stock Sheet")
			sheet := model.NewStockSheet(labelEntry.Text, w, h, q)
			sheet.Thickness = th
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
			a.scheduleOptimize()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 550))
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

	thicknessVal := s.Thickness
	if thicknessVal <= 0 {
		thicknessVal = 18
	}
	editThicknessEntry := widget.NewEntry()
	editThicknessEntry.SetText(fmt.Sprintf("%.1f", thicknessVal))

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
			widget.NewFormItem("Thickness (mm)", editThicknessEntry),
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
			th, _ := strconv.ParseFloat(editThicknessEntry.Text, 64)
			if th <= 0 {
				th = 18
			}
			a.saveState("Edit Stock Sheet")
			a.project.Stocks[idx].Label = labelEntry.Text
			a.project.Stocks[idx].Width = w
			a.project.Stocks[idx].Height = h
			a.project.Stocks[idx].Thickness = th
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
			a.scheduleOptimize()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 500))
	form.Show()
}

// ─── Settings Panel (legacy, now in advanced_settings.go) ───

// buildProfileSelector creates the GCode profile selector widget.
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
		clampZoneListContainer.Add(widget.NewLabel("No clamp zones defined."))
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

	// Run dust shoe collision detection after optimization
	collisions := gcode.CheckDustShoeCollisions(result, a.project.Settings)
	a.lastCollisions = collisions

	a.refreshResults()

	// Show collision warning dialog if collisions detected
	if len(collisions) > 0 {
		warnings := gcode.FormatCollisionWarnings(collisions)
		var msg strings.Builder
		fmt.Fprintf(&msg, "WARNING: %d potential dust shoe collision(s) detected!\n\n", len(collisions))
		maxShow := 5
		for i, w := range warnings {
			if i >= maxShow {
				fmt.Fprintf(&msg, "\n... and %d more collision(s)", len(warnings)-maxShow)
				break
			}
			msg.WriteString(w + "\n")
		}
		msg.WriteString("\nConsider moving clamps or adjusting part positions.")
		dialog.ShowInformation("Dust Shoe Collision Warning", msg.String(), a.window)
	}
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

	if len(codes) == 1 {
		a.saveGCodeFile(codes[0], "sheet1.gcode")
		return
	}

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

// ─── Sharing Functions ──────────────────────────────────────

func (a *App) shareProject() {
	authorEntry := widget.NewEntry()
	authorEntry.SetPlaceHolder("Your name")
	if a.project.Metadata.Author != "" {
		authorEntry.SetText(a.project.Metadata.Author)
	}

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Optional notes for the recipient")
	notesEntry.SetMinRowsVisible(3)
	if a.project.Metadata.Notes != "" {
		notesEntry.SetText(a.project.Metadata.Notes)
	}

	form := dialog.NewForm("Share Project", "Export", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Author", authorEntry),
			widget.NewFormItem("Notes", notesEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				writer.Close()
				path := writer.URI().Path()
				if exportErr := project.ExportShared(path, a.project, authorEntry.Text, notesEntry.Text); exportErr != nil {
					dialog.ShowError(exportErr, a.window)
				} else {
					dialog.ShowInformation("Shared",
						fmt.Sprintf("Project shared to:\n%s", path), a.window)
				}
			}, a.window)
			d.SetFileName(a.project.Name + ".slabshare")
			d.Show()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 300))
	form.Show()
}

func (a *App) importSharedProject() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		proj, importErr := project.ImportShared(reader.URI().Path())
		if importErr != nil {
			dialog.ShowError(importErr, a.window)
			return
		}

		info := fmt.Sprintf("Project: %s\nParts: %d\nStock Sheets: %d",
			proj.Name, len(proj.Parts), len(proj.Stocks))
		if proj.Metadata.Author != "" {
			info += fmt.Sprintf("\nShared by: %s", proj.Metadata.Author)
		}
		if proj.Metadata.Notes != "" {
			info += fmt.Sprintf("\nNotes: %s", proj.Metadata.Notes)
		}

		dialog.ShowConfirm("Import Shared Project",
			fmt.Sprintf("Import this shared project?\n\n%s", info),
			func(ok bool) {
				if !ok {
					return
				}
				a.saveState("Import Shared Project")
				a.project = proj
				a.refreshPartsList()
				a.refreshStockList()
				if a.project.Result != nil {
					a.refreshResults()
				}
				dialog.ShowInformation("Imported",
					fmt.Sprintf("Successfully imported project %q.", proj.Name), a.window)
			},
			a.window,
		)
	}, a.window)
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
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Parts imported: %d", len(result.Parts)))

	if len(result.Errors) > 0 {
		summary.WriteString(fmt.Sprintf("\nRows skipped: %d", len(result.Errors)))
	}

	if len(result.Warnings) > 0 {
		summary.WriteString("\n\nWarnings:\n")
		for _, w := range result.Warnings {
			summary.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

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

	if len(result.Parts) > 0 {
		a.saveState("Import Parts")
		a.project.Parts = append(a.project.Parts, result.Parts...)
		a.refreshPartsList()
		a.scheduleOptimize()
	}

	if len(result.Parts) == 0 && len(result.Errors) > 0 {
		dialog.ShowError(fmt.Errorf("import failed\n\n%s", summary.String()), a.window)
	} else {
		dialog.ShowInformation("Import Summary", summary.String(), a.window)
	}
}

// ─── Purchasing Calculator ──────────────────────────────────

func (a *App) showPurchasingCalculator() {
	if len(a.project.Parts) == 0 {
		dialog.ShowInformation("No Parts", "Add parts to the project first.", a.window)
		return
	}

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
