package widgets

import (
	"fmt"
	"image/color"
	"math"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

const (
	minZoom     = 0.25
	maxZoom     = 10.0
	zoomStep    = 1.15 // multiplicative zoom factor per scroll notch
	defaultZoom = 1.0
)

// SheetCanvas renders a visual representation of a single sheet result
// with mouse wheel zoom and click-and-drag panning.
type SheetCanvas struct {
	widget.BaseWidget
	sheet     model.SheetResult
	settings  model.CutSettings
	maxWidth  float32
	maxHeight float32

	// Zoom and pan state (protected by mutex for thread safety)
	mu       sync.Mutex
	zoom     float64
	panX     float64 // pan offset in screen pixels
	panY     float64
	dragging bool
	dragX    float32 // last drag position
	dragY    float32
}

// NewSheetCanvas creates a new zoomable, pannable sheet canvas widget.
func NewSheetCanvas(sheet model.SheetResult, settings model.CutSettings, maxW, maxH float32) *SheetCanvas {
	sc := &SheetCanvas{
		sheet:     sheet,
		settings:  settings,
		maxWidth:  maxW,
		maxHeight: maxH,
		zoom:      defaultZoom,
	}
	sc.ExtendBaseWidget(sc)
	return sc
}

// Scrolled handles mouse wheel zoom, centered on the cursor position.
func (sc *SheetCanvas) Scrolled(ev *fyne.ScrollEvent) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	oldZoom := sc.zoom

	if ev.Scrolled.DY > 0 {
		sc.zoom *= zoomStep
	} else if ev.Scrolled.DY < 0 {
		sc.zoom /= zoomStep
	}
	sc.zoom = math.Max(minZoom, math.Min(maxZoom, sc.zoom))

	// Adjust pan to keep the point under cursor stationary.
	cursorX := float64(ev.Position.X)
	cursorY := float64(ev.Position.Y)
	factor := sc.zoom / oldZoom
	sc.panX = cursorX - (cursorX-sc.panX)*factor
	sc.panY = cursorY - (cursorY-sc.panY)*factor

	sc.Refresh()
}

// MouseDown starts a pan drag operation.
func (sc *SheetCanvas) MouseDown(ev *desktop.MouseEvent) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.dragging = true
	sc.dragX = ev.Position.X
	sc.dragY = ev.Position.Y
}

// MouseUp ends a pan drag operation.
func (sc *SheetCanvas) MouseUp(_ *desktop.MouseEvent) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.dragging = false
}

// MouseMoved pans the view while dragging.
func (sc *SheetCanvas) MouseMoved(ev *desktop.MouseEvent) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.dragging {
		return
	}

	dx := float64(ev.Position.X - sc.dragX)
	dy := float64(ev.Position.Y - sc.dragY)
	sc.panX += dx
	sc.panY += dy
	sc.dragX = ev.Position.X
	sc.dragY = ev.Position.Y

	sc.Refresh()
}

// ResetZoom resets zoom to 1.0 and pan to origin.
func (sc *SheetCanvas) ResetZoom() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.zoom = defaultZoom
	sc.panX = 0
	sc.panY = 0
	sc.Refresh()
}

// ZoomLevel returns the current zoom level.
func (sc *SheetCanvas) ZoomLevel() float64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.zoom
}

// SetZoomCentered zooms in or out centered on the widget's center point.
func (sc *SheetCanvas) SetZoomCentered(newZoom float64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	oldZoom := sc.zoom
	sc.zoom = math.Max(minZoom, math.Min(maxZoom, newZoom))
	centerX := float64(sc.maxWidth) / 2
	centerY := float64(sc.maxHeight) / 2
	factor := sc.zoom / oldZoom
	sc.panX = centerX - (centerX-sc.panX)*factor
	sc.panY = centerY - (centerY-sc.panY)*factor

	sc.Refresh()
}

// CreateRenderer returns the Fyne widget renderer for this canvas.
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

	// Calculate base scale to fit within max bounds
	scaleX := r.sc.maxWidth / stockW
	scaleY := r.sc.maxHeight / stockH
	baseScale := scaleX
	if scaleY < baseScale {
		baseScale = scaleY
	}

	r.sc.mu.Lock()
	zoom := float32(r.sc.zoom)
	panX := float32(r.sc.panX)
	panY := float32(r.sc.panY)
	r.sc.mu.Unlock()

	scale := baseScale * zoom
	canvasW := stockW * scale
	canvasH := stockH * scale

	// Stock sheet background
	bg := canvas.NewRectangle(color.NRGBA{R: 210, G: 180, B: 140, A: 255}) // wood color
	bg.Resize(fyne.NewSize(canvasW, canvasH))
	bg.Move(fyne.NewPos(panX, panY))
	r.objects = append(r.objects, bg)

	// Stock border
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	border.StrokeWidth = 2
	border.Resize(fyne.NewSize(canvasW, canvasH))
	border.Move(fyne.NewPos(panX, panY))
	r.objects = append(r.objects, border)

	// Draw stock holding tabs (exclusion zones)
	r.drawStockTabs(sheet.Stock, scale, panX, panY)

	// Draw clamp/fixture zones
	r.drawClampZones(scale, panX, panY)

	// Placed parts
	for i, p := range sheet.Placements {
		col := partColors[i%len(partColors)]
		pw := float32(p.PlacedWidth()) * scale
		ph := float32(p.PlacedHeight()) * scale
		px := float32(p.X)*scale + panX
		py := float32(p.Y)*scale + panY

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
func (r *sheetCanvasRenderer) drawStockTabs(stock model.StockSheet, scale, panX, panY float32) {
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
		zx := float32(zone.X)*scale + panX
		zy := float32(zone.Y)*scale + panY
		zw := float32(zone.Width) * scale
		zh := float32(zone.Height) * scale

		// Tab/exclusion zone background
		zoneRect := canvas.NewRectangle(color.NRGBA{R: 255, G: 50, B: 50, A: 120}) // red warning
		zoneRect.Resize(fyne.NewSize(zw, zh))
		zoneRect.Move(fyne.NewPos(zx, zy))
		r.objects = append(r.objects, zoneRect)

		// Zone border
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

// drawClampZones visualizes fixture/clamp exclusion zones on the sheet.
func (r *sheetCanvasRenderer) drawClampZones(scale, panX, panY float32) {
	for _, cz := range r.sc.settings.ClampZones {
		cx := float32(cz.X)*scale + panX
		cy := float32(cz.Y)*scale + panY
		cw := float32(cz.Width) * scale
		ch := float32(cz.Height) * scale

		// Clamp zone background (orange warning color, distinct from red tab zones)
		czRect := canvas.NewRectangle(color.NRGBA{R: 255, G: 165, B: 0, A: 140})
		czRect.Resize(fyne.NewSize(cw, ch))
		czRect.Move(fyne.NewPos(cx, cy))
		r.objects = append(r.objects, czRect)

		// Clamp zone border
		czBorder := canvas.NewRectangle(color.Transparent)
		czBorder.StrokeColor = color.NRGBA{R: 200, G: 120, B: 0, A: 255}
		czBorder.StrokeWidth = 2
		czBorder.Resize(fyne.NewSize(cw, ch))
		czBorder.Move(fyne.NewPos(cx, cy))
		r.objects = append(r.objects, czBorder)

		// Label for larger zones
		if cw > 35 && ch > 15 {
			labelText := "CLAMP"
			if cz.Label != "" {
				labelText = cz.Label
			}
			clampLabel := canvas.NewText(labelText, color.White)
			clampLabel.TextSize = 8
			clampLabel.TextStyle = fyne.TextStyle{Bold: true}
			clampLabel.Move(fyne.NewPos(cx+3, cy+2))
			r.objects = append(r.objects, clampLabel)
		}
	}
}

func (r *sheetCanvasRenderer) Layout(size fyne.Size)        {}
func (r *sheetCanvasRenderer) Refresh()                     { r.rebuild() }
func (r *sheetCanvasRenderer) Destroy()                     {}
func (r *sheetCanvasRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *sheetCanvasRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.sc.maxWidth, r.sc.maxHeight)
}

// RenderSheetResults creates a scrollable container of all sheet results
// with zoom controls and interactive canvases.
func RenderSheetResults(result *model.OptimizeResult, settings model.CutSettings, parts ...[]model.Part) fyne.CanvasObject {
	if result == nil || len(result.Sheets) == 0 {
		return widget.NewLabel("No results yet. Add parts and stock, then click Optimize.")
	}

	var items []fyne.CanvasObject

	for i, sheet := range result.Sheets {
		header := widget.NewLabel(fmt.Sprintf(
			"Sheet %d: %s (%.0f x %.0f) — %d parts, %.1f%% efficiency",
			i+1, sheet.Stock.Label, sheet.Stock.Width, sheet.Stock.Height,
			len(sheet.Placements), sheet.Efficiency(),
		))
		header.TextStyle = fyne.TextStyle{Bold: true}

		sheetCanvas := NewSheetCanvas(sheet, settings, 600, 400)

		// Zoom info label showing current zoom percentage
		zoomLabel := widget.NewLabel("100%")

		resetBtn := widget.NewButtonWithIcon("Reset Zoom", theme.ViewRestoreIcon(), func() {
			sheetCanvas.ResetZoom()
			zoomLabel.SetText("100%")
		})

		zoomInBtn := widget.NewButtonWithIcon("", theme.ZoomInIcon(), func() {
			currentZoom := sheetCanvas.ZoomLevel()
			newZoom := math.Min(maxZoom, currentZoom*zoomStep)
			sheetCanvas.SetZoomCentered(newZoom)
			zoomLabel.SetText(fmt.Sprintf("%.0f%%", sheetCanvas.ZoomLevel()*100))
		})

		zoomOutBtn := widget.NewButtonWithIcon("", theme.ZoomOutIcon(), func() {
			currentZoom := sheetCanvas.ZoomLevel()
			newZoom := math.Max(minZoom, currentZoom/zoomStep)
			sheetCanvas.SetZoomCentered(newZoom)
			zoomLabel.SetText(fmt.Sprintf("%.0f%%", sheetCanvas.ZoomLevel()*100))
		})

		zoomControls := container.NewHBox(
			zoomOutBtn,
			zoomLabel,
			zoomInBtn,
			layout.NewSpacer(),
			resetBtn,
		)

		items = append(items, header, sheetCanvas, zoomControls, widget.NewSeparator())
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

	// Offcuts / remnants section
	offcuts := model.DetectAllOffcuts(*result, settings.KerfWidth)
	if len(offcuts) > 0 {
		items = append(items, widget.NewSeparator())
		offcutHeader := widget.NewLabel("Usable Offcuts / Remnants:")
		offcutHeader.TextStyle = fyne.TextStyle{Bold: true}
		items = append(items, offcutHeader)

		for _, o := range offcuts {
			offcutText := fmt.Sprintf(
				"  Sheet %d (%s): %.0f x %.0f mm at position (%.0f, %.0f) — %.0f sq mm",
				o.SheetIndex+1, o.SheetLabel, o.Width, o.Height, o.X, o.Y, o.Area(),
			)
			if o.PricePerSheet > 0 {
				offcutText += fmt.Sprintf(" (~%.2f value)", o.PricePerSheet)
			}
			items = append(items, widget.NewLabel(offcutText))
		}

		totalOffcutArea := model.TotalOffcutArea(offcuts)
		offcutSummary := widget.NewLabel(fmt.Sprintf(
			"  Total reusable remnant area: %.0f sq mm (%d piece(s))",
			totalOffcutArea, len(offcuts),
		))
		items = append(items, offcutSummary)
	}

	// Edge banding summary (if parts are provided and any have banding)
	if len(parts) > 0 && len(parts[0]) > 0 {
		bandingSummary := model.CalculateEdgeBanding(parts[0], 10.0) // Default 10% waste
		if bandingSummary.TotalLinearMM > 0 {
			items = append(items, widget.NewSeparator())
			bandingHeader := widget.NewLabel("Edge Banding Summary:")
			bandingHeader.TextStyle = fyne.TextStyle{Bold: true}
			items = append(items, bandingHeader)

			perPart := model.CalculatePerPartEdgeBanding(parts[0])
			for _, pp := range perPart {
				items = append(items, widget.NewLabel(fmt.Sprintf(
					"  %s (%s): %.0f mm/piece x %d = %.0f mm",
					pp.Label, pp.Edges, pp.LengthPerUnit, pp.Quantity, pp.TotalLength,
				)))
			}

			items = append(items, widget.NewLabel(fmt.Sprintf(
				"  Total: %.1f m (%d edges on %d pieces) | With 10%% waste: %.1f m",
				bandingSummary.TotalLinearM, bandingSummary.EdgeCount, bandingSummary.PartCount,
				bandingSummary.TotalWithWasteM,
			)))
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
