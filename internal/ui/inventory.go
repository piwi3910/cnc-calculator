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
	"github.com/piwi3910/SlabCut/internal/project"
)

// ─── Tool Inventory Dialog ─────────────────────────────────

func (a *App) showToolInventoryDialog() {
	toolList := container.NewVBox()
	var refreshList func()

	refreshList = func() {
		toolList.RemoveAll()

		if len(a.inventory.Tools) == 0 {
			toolList.Add(widget.NewLabel("No tool profiles defined."))
			return
		}

		header := container.NewGridWithColumns(6,
			widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Diameter", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Feed Rate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("RPM", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
			widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		)
		toolList.Add(header)
		toolList.Add(widget.NewSeparator())

		for i := range a.inventory.Tools {
			idx := i
			t := a.inventory.Tools[idx]
			row := container.NewGridWithColumns(6,
				widget.NewLabel(t.Name),
				widget.NewLabel(fmt.Sprintf("%.2f mm", t.ToolDiameter)),
				widget.NewLabel(fmt.Sprintf("%.0f mm/min", t.FeedRate)),
				widget.NewLabel(fmt.Sprintf("%d", t.SpindleSpeed)),
				widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
					a.showEditToolDialog(idx, refreshList)
				}),
				widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
					a.inventory.Tools = append(a.inventory.Tools[:idx], a.inventory.Tools[idx+1:]...)
					a.saveInventory()
					refreshList()
				}),
			)
			toolList.Add(row)
		}
	}

	refreshList()

	addBtn := widget.NewButtonWithIcon("Add Tool Profile", theme.ContentAddIcon(), func() {
		a.showAddToolDialog(refreshList)
	})

	importBtn := widget.NewButtonWithIcon("Import...", theme.FolderOpenIcon(), func() {
		a.importInventory(refreshList)
	})

	exportBtn := widget.NewButtonWithIcon("Export...", theme.DocumentSaveIcon(), func() {
		a.exportInventory()
	})

	toolbar := container.NewHBox(addBtn, layout.NewSpacer(), importBtn, exportBtn)

	content := container.NewBorder(
		toolbar,
		nil, nil, nil,
		container.NewVScroll(toolList),
	)

	d := dialog.NewCustom("Tool Inventory", "Close", content, a.window)
	d.Resize(fyne.NewSize(700, 500))
	d.Show()
}

func (a *App) showAddToolDialog(onDone func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Tool profile name")
	nameEntry.SetText("New End Mill")

	diameterEntry := widget.NewEntry()
	diameterEntry.SetText("6.0")

	feedEntry := widget.NewEntry()
	feedEntry.SetText("1500")

	plungeEntry := widget.NewEntry()
	plungeEntry.SetText("500")

	rpmEntry := widget.NewEntry()
	rpmEntry.SetText("18000")

	safeZEntry := widget.NewEntry()
	safeZEntry.SetText("5.0")

	cutDepthEntry := widget.NewEntry()
	cutDepthEntry.SetText("18.0")

	passDepthEntry := widget.NewEntry()
	passDepthEntry.SetText("6.0")

	form := dialog.NewForm("Add Tool Profile", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", nameEntry),
			widget.NewFormItem("Tool Diameter (mm)", diameterEntry),
			widget.NewFormItem("Feed Rate (mm/min)", feedEntry),
			widget.NewFormItem("Plunge Rate (mm/min)", plungeEntry),
			widget.NewFormItem("Spindle Speed (RPM)", rpmEntry),
			widget.NewFormItem("Safe Z (mm)", safeZEntry),
			widget.NewFormItem("Material Thickness (mm)", cutDepthEntry),
			widget.NewFormItem("Pass Depth (mm)", passDepthEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			diameter, _ := strconv.ParseFloat(diameterEntry.Text, 64)
			feed, _ := strconv.ParseFloat(feedEntry.Text, 64)
			plunge, _ := strconv.ParseFloat(plungeEntry.Text, 64)
			rpm, _ := strconv.Atoi(rpmEntry.Text)
			safeZ, _ := strconv.ParseFloat(safeZEntry.Text, 64)
			cutDepth, _ := strconv.ParseFloat(cutDepthEntry.Text, 64)
			passDepth, _ := strconv.ParseFloat(passDepthEntry.Text, 64)

			if diameter <= 0 || feed <= 0 || rpm <= 0 {
				dialog.ShowError(fmt.Errorf("diameter, feed rate, and RPM must be > 0"), a.window)
				return
			}

			tool := model.NewToolProfile(nameEntry.Text, diameter, feed, plunge, rpm, safeZ, cutDepth, passDepth)
			a.inventory.Tools = append(a.inventory.Tools, tool)
			a.saveInventory()
			onDone()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 500))
	form.Show()
}

func (a *App) showEditToolDialog(idx int, onDone func()) {
	t := a.inventory.Tools[idx]

	nameEntry := widget.NewEntry()
	nameEntry.SetText(t.Name)

	diameterEntry := widget.NewEntry()
	diameterEntry.SetText(fmt.Sprintf("%.2f", t.ToolDiameter))

	feedEntry := widget.NewEntry()
	feedEntry.SetText(fmt.Sprintf("%.0f", t.FeedRate))

	plungeEntry := widget.NewEntry()
	plungeEntry.SetText(fmt.Sprintf("%.0f", t.PlungeRate))

	rpmEntry := widget.NewEntry()
	rpmEntry.SetText(fmt.Sprintf("%d", t.SpindleSpeed))

	safeZEntry := widget.NewEntry()
	safeZEntry.SetText(fmt.Sprintf("%.1f", t.SafeZ))

	cutDepthEntry := widget.NewEntry()
	cutDepthEntry.SetText(fmt.Sprintf("%.1f", t.CutDepth))

	passDepthEntry := widget.NewEntry()
	passDepthEntry.SetText(fmt.Sprintf("%.1f", t.PassDepth))

	form := dialog.NewForm("Edit Tool Profile", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", nameEntry),
			widget.NewFormItem("Tool Diameter (mm)", diameterEntry),
			widget.NewFormItem("Feed Rate (mm/min)", feedEntry),
			widget.NewFormItem("Plunge Rate (mm/min)", plungeEntry),
			widget.NewFormItem("Spindle Speed (RPM)", rpmEntry),
			widget.NewFormItem("Safe Z (mm)", safeZEntry),
			widget.NewFormItem("Material Thickness (mm)", cutDepthEntry),
			widget.NewFormItem("Pass Depth (mm)", passDepthEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			a.inventory.Tools[idx].Name = nameEntry.Text
			a.inventory.Tools[idx].ToolDiameter, _ = strconv.ParseFloat(diameterEntry.Text, 64)
			a.inventory.Tools[idx].FeedRate, _ = strconv.ParseFloat(feedEntry.Text, 64)
			a.inventory.Tools[idx].PlungeRate, _ = strconv.ParseFloat(plungeEntry.Text, 64)
			a.inventory.Tools[idx].SpindleSpeed, _ = strconv.Atoi(rpmEntry.Text)
			a.inventory.Tools[idx].SafeZ, _ = strconv.ParseFloat(safeZEntry.Text, 64)
			a.inventory.Tools[idx].CutDepth, _ = strconv.ParseFloat(cutDepthEntry.Text, 64)
			a.inventory.Tools[idx].PassDepth, _ = strconv.ParseFloat(passDepthEntry.Text, 64)
			a.saveInventory()
			onDone()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 500))
	form.Show()
}

// ─── Stock Inventory Dialog ────────────────────────────────

func (a *App) showStockInventoryDialog() {
	stockList := container.NewVBox()
	var refreshList func()

	refreshList = func() {
		stockList.RemoveAll()

		if len(a.inventory.Stocks) == 0 {
			stockList.Add(widget.NewLabel("No stock presets defined."))
			return
		}

		header := container.NewGridWithColumns(7,
			widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Width", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Height", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Material", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Price/Sheet", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
			widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		)
		stockList.Add(header)
		stockList.Add(widget.NewSeparator())

		for i := range a.inventory.Stocks {
			idx := i
			s := a.inventory.Stocks[idx]
			priceLabel := "-"
			if s.PricePerSheet > 0 {
				priceLabel = fmt.Sprintf("%.2f", s.PricePerSheet)
			}
			row := container.NewGridWithColumns(7,
				widget.NewLabel(s.Name),
				widget.NewLabel(fmt.Sprintf("%.0f mm", s.Width)),
				widget.NewLabel(fmt.Sprintf("%.0f mm", s.Height)),
				widget.NewLabel(s.Material),
				widget.NewLabel(priceLabel),
				widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
					a.showEditStockPresetDialog(idx, refreshList)
				}),
				widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
					a.inventory.Stocks = append(a.inventory.Stocks[:idx], a.inventory.Stocks[idx+1:]...)
					a.saveInventory()
					refreshList()
				}),
			)
			stockList.Add(row)
		}
	}

	refreshList()

	addBtn := widget.NewButtonWithIcon("Add Stock Preset", theme.ContentAddIcon(), func() {
		a.showAddStockPresetDialog(refreshList)
	})

	importBtn := widget.NewButtonWithIcon("Import...", theme.FolderOpenIcon(), func() {
		a.importInventory(refreshList)
	})

	exportBtn := widget.NewButtonWithIcon("Export...", theme.DocumentSaveIcon(), func() {
		a.exportInventory()
	})

	toolbar := container.NewHBox(addBtn, layout.NewSpacer(), importBtn, exportBtn)

	content := container.NewBorder(
		toolbar,
		nil, nil, nil,
		container.NewVScroll(stockList),
	)

	d := dialog.NewCustom("Stock Inventory", "Close", content, a.window)
	d.Resize(fyne.NewSize(700, 500))
	d.Show()
}

func (a *App) showAddStockPresetDialog(onDone func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Stock preset name")
	nameEntry.SetText("New Sheet")

	widthEntry := widget.NewEntry()
	widthEntry.SetText("2440")

	heightEntry := widget.NewEntry()
	heightEntry.SetText("1220")

	materialEntry := widget.NewEntry()
	materialEntry.SetPlaceHolder("e.g., Plywood, MDF, Acrylic")
	materialEntry.SetText("Plywood")

	priceEntry := widget.NewEntry()
	priceEntry.SetPlaceHolder("0.00 (optional)")
	priceEntry.SetText("0")

	form := dialog.NewForm("Add Stock Preset", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", nameEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Material", materialEntry),
			widget.NewFormItem("Price per Sheet", priceEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			w, _ := strconv.ParseFloat(widthEntry.Text, 64)
			h, _ := strconv.ParseFloat(heightEntry.Text, 64)
			if w <= 0 || h <= 0 {
				dialog.ShowError(fmt.Errorf("width and height must be > 0"), a.window)
				return
			}

			price, _ := strconv.ParseFloat(priceEntry.Text, 64)
			preset := model.NewStockPresetWithPrice(nameEntry.Text, w, h, materialEntry.Text, price)
			a.inventory.Stocks = append(a.inventory.Stocks, preset)
			a.saveInventory()
			onDone()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 400))
	form.Show()
}

func (a *App) showEditStockPresetDialog(idx int, onDone func()) {
	s := a.inventory.Stocks[idx]

	nameEntry := widget.NewEntry()
	nameEntry.SetText(s.Name)

	widthEntry := widget.NewEntry()
	widthEntry.SetText(fmt.Sprintf("%.0f", s.Width))

	heightEntry := widget.NewEntry()
	heightEntry.SetText(fmt.Sprintf("%.0f", s.Height))

	materialEntry := widget.NewEntry()
	materialEntry.SetText(s.Material)

	priceEntry := widget.NewEntry()
	priceEntry.SetText(fmt.Sprintf("%.2f", s.PricePerSheet))

	form := dialog.NewForm("Edit Stock Preset", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", nameEntry),
			widget.NewFormItem("Width (mm)", widthEntry),
			widget.NewFormItem("Height (mm)", heightEntry),
			widget.NewFormItem("Material", materialEntry),
			widget.NewFormItem("Price per Sheet", priceEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			a.inventory.Stocks[idx].Name = nameEntry.Text
			a.inventory.Stocks[idx].Width, _ = strconv.ParseFloat(widthEntry.Text, 64)
			a.inventory.Stocks[idx].Height, _ = strconv.ParseFloat(heightEntry.Text, 64)
			a.inventory.Stocks[idx].Material = materialEntry.Text
			a.inventory.Stocks[idx].PricePerSheet, _ = strconv.ParseFloat(priceEntry.Text, 64)
			a.saveInventory()
			onDone()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 400))
	form.Show()
}

// ─── Import / Export ───────────────────────────────────────

func (a *App) importInventory(onDone func()) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		merged, err := project.ImportInventory(reader.URI().Path(), a.inventory)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		a.inventory = merged
		a.saveInventory()
		onDone()
		dialog.ShowInformation("Import Complete",
			fmt.Sprintf("Inventory now contains %d tools and %d stock presets.",
				len(a.inventory.Tools), len(a.inventory.Stocks)),
			a.window)
	}, a.window)
}

func (a *App) exportInventory() {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		if err := project.ExportInventory(writer.URI().Path(), a.inventory); err != nil {
			dialog.ShowError(err, a.window)
		} else {
			dialog.ShowInformation("Export Complete",
				fmt.Sprintf("Inventory exported to %s", writer.URI().Path()),
				a.window)
		}
	}, a.window)
	d.SetFileName("inventory.json")
	d.Show()
}

// ─── Inventory Integration Helpers ─────────────────────────

// saveInventory persists the current inventory to disk.
func (a *App) saveInventory() {
	if a.inventoryPath == "" {
		return
	}
	if err := project.SaveInventory(a.inventoryPath, a.inventory); err != nil {
		dialog.ShowError(fmt.Errorf("failed to save inventory: %w", err), a.window)
	}
}

// showAddStockFromInventory shows a picker to add a stock sheet from inventory presets.
func (a *App) showAddStockFromInventory() {
	if len(a.inventory.Stocks) == 0 {
		dialog.ShowInformation("No Presets",
			"No stock presets defined. Use Admin > Stock Inventory to add presets.",
			a.window)
		return
	}

	names := a.inventory.StockNames()
	stockSelect := widget.NewSelect(names, nil)
	stockSelect.SetSelected(names[0])

	qtyEntry := widget.NewEntry()
	qtyEntry.SetText("1")

	form := dialog.NewForm("Add from Inventory", "Add", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Stock Preset", stockSelect),
			widget.NewFormItem("Quantity", qtyEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			preset := a.inventory.FindStockByName(stockSelect.Selected)
			if preset == nil {
				return
			}
			qty, _ := strconv.Atoi(qtyEntry.Text)
			if qty <= 0 {
				qty = 1
			}
			sheet := preset.ToStockSheet(qty)
			a.project.Stocks = append(a.project.Stocks, sheet)
			a.refreshStockList()
		},
		a.window,
	)
	form.Resize(fyne.NewSize(400, 250))
	form.Show()
}

// buildToolProfileSelector creates a dropdown to load tool profile settings.
func (a *App) buildToolProfileSelector() fyne.CanvasObject {
	names := a.inventory.ToolNames()
	if len(names) == 0 {
		return widget.NewLabel("No tool profiles. Use Admin > Tool Inventory to add profiles.")
	}

	toolSelect := widget.NewSelect(names, func(selected string) {
		tool := a.inventory.FindToolByName(selected)
		if tool == nil {
			return
		}
		tool.ApplyToSettings(&a.project.Settings)
		// Rebuild the settings panel to reflect new values
		a.tabs.Items[2].Content = a.buildSettingsPanel()
		a.tabs.Refresh()
	})
	toolSelect.PlaceHolder = "Load from Tool Profile..."

	return toolSelect
}
