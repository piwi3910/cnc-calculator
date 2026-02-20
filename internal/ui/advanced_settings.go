package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/piwi3910/SlabCut/internal/model"
)

// showAdvancedSettingsDialog opens a dialog with all advanced CNC/optimizer settings
// that are not shown in the quick settings sidebar.
func (a *App) showAdvancedSettingsDialog() {
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

	// --- Optimization Weights ---
	weightsSection := widget.NewCard("Optimization Objectives",
		"Weight priorities for multi-objective optimization (0 = disabled, higher = more important)",
		container.NewGridWithColumns(2,
			widget.NewLabel("Minimize Waste"), floatEntry(&s.OptimizeWeights.MinimizeWaste),
			widget.NewLabel("Minimize Sheets"), floatEntry(&s.OptimizeWeights.MinimizeSheets),
			widget.NewLabel("Minimize Cut Length"), floatEntry(&s.OptimizeWeights.MinimizeCutLen),
			widget.NewLabel("Minimize Job Time"), floatEntry(&s.OptimizeWeights.MinimizeJobTime),
		))

	// --- GCode Profile ---
	profileNames := model.GetProfileNames()
	profileSelect := widget.NewSelect(profileNames, func(selected string) {
		s.GCodeProfile = selected
	})
	profileSelect.SetSelected(s.GCodeProfile)

	manageProfileBtn := widget.NewButtonWithIcon("Manage Profiles", theme.SettingsIcon(), func() {
		a.showProfileManager()
	})

	profileSection := widget.NewCard("GCode Profile", "",
		container.NewVBox(
			container.NewGridWithColumns(2,
				widget.NewLabel("Active Profile"), container.NewBorder(nil, nil, nil, manageProfileBtn, profileSelect),
			),
		))

	// --- Lead-In / Lead-Out Arcs ---
	leadInOutSection := widget.NewCard("Lead-In / Lead-Out Arcs",
		"Arc approach and exit for smoother cuts",
		container.NewGridWithColumns(2,
			widget.NewLabel("Lead-In Radius (mm)"), floatEntry(&s.LeadInRadius),
			widget.NewLabel("Lead-Out Radius (mm)"), floatEntry(&s.LeadOutRadius),
			widget.NewLabel("Approach Angle (degrees)"), floatEntry(&s.LeadInAngle),
		))

	// --- Plunge Entry Strategy ---
	plungeTypeSelect := widget.NewSelect(model.PlungeTypeOptions(), func(selected string) {
		s.PlungeType = model.PlungeTypeFromString(selected)
	})
	plungeTypeSelect.SetSelected(s.PlungeType.String())

	plungeSection := widget.NewCard("Plunge Entry Strategy",
		"How the tool enters the material",
		container.NewGridWithColumns(2,
			widget.NewLabel("Plunge Type"), plungeTypeSelect,
			widget.NewLabel("Ramp Angle (degrees)"), floatEntry(&s.RampAngle),
			widget.NewLabel("Helix Diameter (mm)"), floatEntry(&s.HelixDiameter),
			widget.NewLabel("Helix Depth/Rev (%)"), floatEntry(&s.HelixRevPercent),
		))

	// --- Corner Overcuts ---
	cornerOvercutSelect := widget.NewSelect(model.CornerOvercutOptions(), func(selected string) {
		s.CornerOvercut = model.CornerOvercutFromString(selected)
	})
	cornerOvercutSelect.SetSelected(s.CornerOvercut.String())

	cornerSection := widget.NewCard("Corner Overcuts",
		"Relief cuts for square interior corners",
		container.NewGridWithColumns(2,
			widget.NewLabel("Corner Type"), cornerOvercutSelect,
		))

	// --- Onion Skinning ---
	onionSkinCheck := widget.NewCheck("", func(b bool) { s.OnionSkinEnabled = b })
	onionSkinCheck.Checked = s.OnionSkinEnabled
	onionCleanupCheck := widget.NewCheck("", func(b bool) { s.OnionSkinCleanup = b })
	onionCleanupCheck.Checked = s.OnionSkinCleanup

	onionSkinSection := widget.NewCard("Onion Skinning",
		"Leave thin layer on final pass to prevent part movement",
		container.NewGridWithColumns(2,
			widget.NewLabel("Enable Onion Skin"), onionSkinCheck,
			widget.NewLabel("Skin Thickness (mm)"), floatEntry(&s.OnionSkinDepth),
			widget.NewLabel("Generate Cleanup Pass"), onionCleanupCheck,
		))

	// --- Toolpath Ordering ---
	optimizeToolpathCheck := widget.NewCheck("", func(b bool) { s.OptimizeToolpath = b })
	optimizeToolpathCheck.Checked = s.OptimizeToolpath

	structuralOrderCheck := widget.NewCheck("", func(b bool) { s.StructuralOrdering = b })
	structuralOrderCheck.Checked = s.StructuralOrdering

	toolpathSection := widget.NewCard("Toolpath Ordering", "",
		container.NewGridWithColumns(2,
			widget.NewLabel("Optimize Toolpath Order"), optimizeToolpathCheck,
			widget.NewLabel("Structural Cut Ordering"), structuralOrderCheck,
			widget.NewLabel("Nesting Rotations (outline parts)"), intEntry(&s.NestingRotations),
		))

	// --- Stock Holding Tabs ---
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
		))

	// --- Clamp Zones ---
	clampZoneListContainer = container.NewVBox()
	a.refreshClampZoneList()

	addClampBtn := widget.NewButtonWithIcon("Add Clamp Zone", theme.ContentAddIcon(), func() {
		a.showAddClampZoneDialog()
	})

	clampZoneSection := widget.NewCard("Fixture / Clamp Zones",
		"Define exclusion zones where clamps or fixtures are placed on the stock sheet",
		container.NewVBox(
			container.NewHBox(layout.NewSpacer(), addClampBtn),
			clampZoneListContainer,
		))

	// --- Dust Shoe Collision Detection ---
	dustShoeCheck := widget.NewCheck("", func(b bool) { s.DustShoeEnabled = b })
	dustShoeCheck.Checked = s.DustShoeEnabled

	dustShoeSection := widget.NewCard("Dust Shoe Collision Detection",
		"Detect potential collisions between dust shoe and clamp/fixture zones",
		container.NewGridWithColumns(2,
			widget.NewLabel("Enable Collision Detection"), dustShoeCheck,
			widget.NewLabel("Dust Shoe Width (mm)"), floatEntry(&s.DustShoeWidth),
			widget.NewLabel("Minimum Clearance (mm)"), floatEntry(&s.DustShoeClearance),
		))

	// --- Part Holding Tabs ---
	partTabSection := widget.NewCard("Part Holding Tabs",
		"Tabs to keep parts connected during cut",
		container.NewGridWithColumns(2,
			widget.NewLabel("Tab Width (mm)"), floatEntry(&s.PartTabWidth),
			widget.NewLabel("Tab Height (mm)"), floatEntry(&s.PartTabHeight),
			widget.NewLabel("Tabs per Side"), intEntry(&s.PartTabsPerSide),
		))

	// Assemble all sections into a scrollable layout
	content := container.NewVScroll(container.NewVBox(
		weightsSection,
		profileSection,
		toolpathSection,
		plungeSection,
		leadInOutSection,
		cornerSection,
		onionSkinSection,
		partTabSection,
		stockTabSection,
		clampZoneSection,
		dustShoeSection,
	))

	d := dialog.NewCustom("Advanced Settings", "Close", content, a.window)
	d.Resize(fyne.NewSize(650, 700))
	d.Show()
}
