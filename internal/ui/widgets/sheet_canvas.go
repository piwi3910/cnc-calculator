package widgets

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/piwi3910/SlabCut/internal/model"
)

// Part colors — cycle through these for visual distinction.
var partColors = []color.NRGBA{
	{R: 76, G: 175, B: 80, A: 200},  // green
	{R: 33, G: 150, B: 243, A: 200}, // blue
	{R: 255, G: 152, B: 0, A: 200},  // orange
	{R: 156, G: 39, B: 176, A: 200}, // purple
	{R: 0, G: 188, B: 212, A: 200},  // cyan
	{R: 244, G: 67, B: 54, A: 200},  // red
	{R: 255, G: 235, B: 59, A: 200}, // yellow
	{R: 121, G: 85, B: 72, A: 200},  // brown
}

// SheetCanvas renders a visual representation of a single sheet result.
type SheetCanvas struct {
	widget.BaseWidget
	sheet     model.SheetResult
	settings  model.CutSettings
	maxWidth  float32
	maxHeight float32
}

func NewSheetCanvas(sheet model.SheetResult, settings model.CutSettings, maxW, maxH float32) *SheetCanvas {
	sc := &SheetCanvas{
		sheet:     sheet,
		settings:  settings,
		maxWidth:  maxW,
		maxHeight: maxH,
	}
	sc.ExtendBaseWidget(sc)
	return sc
}

func (sc *SheetCanvas) CreateRenderer() fyne.WidgetRenderer {
	return newSheetCanvasRenderer(sc)
}

type sheetCanvasRenderer struct {
	sc      *SheetCanvas
	objects []fyne.CanvasObject
}

func newSheetCanvasRenderer(sc *SheetCanvas) *sheetCanvasRenderer {
	r := &sheetCanvasRenderer{sc: sc}
	r.rebuild()
	return r
}

func (r *sheetCanvasRenderer) rebuild() {
	r.objects = nil

	sheet := r.sc.sheet
	stockW := float32(sheet.Stock.Width)
	stockH := float32(sheet.Stock.Height)

	// Calculate scale to fit within max bounds
	scaleX := r.sc.maxWidth / stockW
	scaleY := r.sc.maxHeight / stockH
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	canvasW := stockW * scale
	canvasH := stockH * scale

	// Stock sheet background
	bg := canvas.NewRectangle(color.NRGBA{R: 210, G: 180, B: 140, A: 255}) // wood color
	bg.Resize(fyne.NewSize(canvasW, canvasH))
	bg.Move(fyne.NewPos(0, 0))
	r.objects = append(r.objects, bg)

	// Stock border
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	border.StrokeWidth = 2
	border.Resize(fyne.NewSize(canvasW, canvasH))
	border.Move(fyne.NewPos(0, 0))
	r.objects = append(r.objects, border)

	// Draw stock holding tabs (exclusion zones)
	r.drawStockTabs(sheet.Stock, scale, canvasW, canvasH)

	// Placed parts
	for i, p := range sheet.Placements {
		col := partColors[i%len(partColors)]
		pw := float32(p.PlacedWidth()) * scale
		ph := float32(p.PlacedHeight()) * scale
		px := float32(p.X) * scale
		py := float32(p.Y) * scale

		// Part rectangle
		partRect := canvas.NewRectangle(col)
		partRect.Resize(fyne.NewSize(pw, ph))
		partRect.Move(fyne.NewPos(px, py))
		r.objects = append(r.objects, partRect)

		// Part border
		partBorder := canvas.NewRectangle(color.Transparent)
		partBorder.StrokeColor = color.NRGBA{R: 30, G: 30, B: 30, A: 255}
		partBorder.StrokeWidth = 1
		partBorder.Resize(fyne.NewSize(pw, ph))
		partBorder.Move(fyne.NewPos(px, py))
		r.objects = append(r.objects, partBorder)

		// Label (only if big enough)
		if pw > 30 && ph > 16 {
			label := canvas.NewText(
				fmt.Sprintf("%s\n%.0fx%.0f", p.Part.Label, p.Part.Width, p.Part.Height),
				color.Black,
			)
			label.TextSize = 10
			label.Move(fyne.NewPos(px+3, py+2))
			r.objects = append(r.objects, label)
		}
	}
}

// drawStockTabs visualizes stock sheet holding tab zones
func (r *sheetCanvasRenderer) drawStockTabs(stock model.StockSheet, scale, canvasW, canvasH float32) {
	// Get tab config for this sheet (use stock's override or global settings)
	tabConfig := stock.Tabs
	if !tabConfig.Enabled {
		tabConfig = r.sc.settings.StockTabs
		if !tabConfig.Enabled {
			return // No tabs enabled
		}
	}

	var zones []model.TabZone

	if tabConfig.AdvancedMode {
		// Use custom zones
		zones = tabConfig.CustomZones
	} else {
		// Simple mode: convert edge padding to zones
		// Each edge becomes a zone along the full length
		if tabConfig.TopPadding > 0 {
			zones = append(zones, model.TabZone{
				X:      0,
				Y:      0,
				Width:  stock.Width,
				Height: tabConfig.TopPadding,
			})
		}
		if tabConfig.BottomPadding > 0 {
			zones = append(zones, model.TabZone{
				X:      0,
				Y:      stock.Height - tabConfig.BottomPadding,
				Width:  stock.Width,
				Height: tabConfig.BottomPadding,
			})
		}
		if tabConfig.LeftPadding > 0 {
			zones = append(zones, model.TabZone{
				X:      0,
				Y:      0,
				Width:  tabConfig.LeftPadding,
				Height: stock.Height,
			})
		}
		if tabConfig.RightPadding > 0 {
			zones = append(zones, model.TabZone{
				X:      stock.Width - tabConfig.RightPadding,
				Y:      0,
				Width:  tabConfig.RightPadding,
				Height: stock.Height,
			})
		}
	}

	// Draw each zone as a semi-transparent red danger zone
	for _, zone := range zones {
		zx := float32(zone.X) * scale
		zy := float32(zone.Y) * scale
		zw := float32(zone.Width) * scale
		zh := float32(zone.Height) * scale

		// Tab/exclusion zone background
		zoneRect := canvas.NewRectangle(color.NRGBA{R: 255, G: 50, B: 50, A: 120}) // red warning
		zoneRect.Resize(fyne.NewSize(zw, zh))
		zoneRect.Move(fyne.NewPos(zx, zy))
		r.objects = append(r.objects, zoneRect)

		// Zone border (diagonal hatch pattern simulated with border)
		zoneBorder := canvas.NewRectangle(color.Transparent)
		zoneBorder.StrokeColor = color.NRGBA{R: 200, G: 0, B: 0, A: 255}
		zoneBorder.StrokeWidth = 2
		zoneBorder.Resize(fyne.NewSize(zw, zh))
		zoneBorder.Move(fyne.NewPos(zx, zy))
		r.objects = append(r.objects, zoneBorder)

		// Add label for larger zones
		if zw > 40 && zh > 15 {
			label := canvas.NewText("NO CUT", color.White)
			label.TextSize = 8
			label.TextStyle = fyne.TextStyle{Bold: true}
			label.Move(fyne.NewPos(zx+5, zy+2))
			r.objects = append(r.objects, label)
		}
	}
}

func (r *sheetCanvasRenderer) Layout(size fyne.Size)        {}
func (r *sheetCanvasRenderer) Refresh()                     { r.rebuild() }
func (r *sheetCanvasRenderer) Destroy()                     {}
func (r *sheetCanvasRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *sheetCanvasRenderer) MinSize() fyne.Size {
	sheet := r.sc.sheet
	stockW := float32(sheet.Stock.Width)
	stockH := float32(sheet.Stock.Height)
	scaleX := r.sc.maxWidth / stockW
	scaleY := r.sc.maxHeight / stockH
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	return fyne.NewSize(stockW*scale, stockH*scale)
}

// RenderSheetResults creates a scrollable container of all sheet results.
func RenderSheetResults(result *model.OptimizeResult, settings model.CutSettings) fyne.CanvasObject {
	if result == nil || len(result.Sheets) == 0 {
		return widget.NewLabel("No results yet. Add parts and stock, then click Optimize.")
	}

	var items []fyne.CanvasObject

	for i, sheet := range result.Sheets {
		header := widget.NewLabel(fmt.Sprintf(
			"Sheet %d: %s (%.0f × %.0f) — %d parts, %.1f%% efficiency",
			i+1, sheet.Stock.Label, sheet.Stock.Width, sheet.Stock.Height,
			len(sheet.Placements), sheet.Efficiency(),
		))
		header.TextStyle = fyne.TextStyle{Bold: true}

		sheetCanvas := NewSheetCanvas(sheet, settings, 600, 400)

		items = append(items, header, sheetCanvas, widget.NewSeparator())
	}

	if len(result.UnplacedParts) > 0 {
		warning := widget.NewLabel(fmt.Sprintf(
			"WARNING: %d parts could not be placed! Add more stock sheets.",
			len(result.UnplacedParts),
		))
		warning.Importance = widget.DangerImportance
		items = append(items, warning)
	}

	// Per-stock-size breakdown
	sizeBreakdown := buildStockSizeBreakdown(result)
	if len(sizeBreakdown) > 1 {
		items = append(items, widget.NewSeparator())
		breakdownHeader := widget.NewLabel("Stock Size Breakdown:")
		breakdownHeader.TextStyle = fyne.TextStyle{Bold: true}
		items = append(items, breakdownHeader)
		for _, line := range sizeBreakdown {
			items = append(items, widget.NewLabel(line))
		}
	}

	summaryText := fmt.Sprintf(
		"Total: %d sheets used, %.1f%% overall efficiency",
		len(result.Sheets), result.TotalEfficiency(),
	)
	if result.HasPricing() {
		summaryText += fmt.Sprintf(" | Estimated material cost: %.2f", result.TotalCost())
	}
	summary := widget.NewLabel(summaryText)
	summary.TextStyle = fyne.TextStyle{Bold: true}
	items = append(items, summary)

	return container.NewVScroll(container.NewVBox(items...))
}

// buildStockSizeBreakdown generates per-stock-size statistics lines.
// Groups sheets by their dimensions and reports count, total parts, and efficiency.
func buildStockSizeBreakdown(result *model.OptimizeResult) []string {
	if result == nil || len(result.Sheets) == 0 {
		return nil
	}

	type sizeKey struct {
		w, h float64
	}
	type sizeStats struct {
		label      string
		count      int
		totalParts int
		usedArea   float64
		totalArea  float64
	}

	// Preserve insertion order with a slice of keys
	var order []sizeKey
	statsMap := make(map[sizeKey]*sizeStats)

	for _, sheet := range result.Sheets {
		key := sizeKey{sheet.Stock.Width, sheet.Stock.Height}
		if _, exists := statsMap[key]; !exists {
			order = append(order, key)
			statsMap[key] = &sizeStats{label: sheet.Stock.Label}
		}
		s := statsMap[key]
		s.count++
		s.totalParts += len(sheet.Placements)
		s.usedArea += sheet.UsedArea()
		s.totalArea += sheet.TotalArea()
	}

	var lines []string
	for _, key := range order {
		s := statsMap[key]
		eff := 0.0
		if s.totalArea > 0 {
			eff = (s.usedArea / s.totalArea) * 100.0
		}
		lines = append(lines, fmt.Sprintf(
			"  %.0f x %.0f: %d sheet(s), %d parts, %.1f%% efficiency",
			key.w, key.h, s.count, s.totalParts, eff,
		))
	}
	return lines
}
