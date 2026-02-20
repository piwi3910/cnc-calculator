// Package export provides functionality for exporting cut optimization results
// to various file formats.
package export

import (
	"fmt"
	"math"

	"github.com/go-pdf/fpdf"
	"github.com/piwi3910/SlabCut/internal/model"
)

// partColor represents an RGB color for a placed part.
type partColor struct {
	R, G, B int
}

// partColors mirrors the color scheme used in the UI sheet canvas widget.
var partColors = []partColor{
	{R: 76, G: 175, B: 80},  // green
	{R: 33, G: 150, B: 243}, // blue
	{R: 255, G: 152, B: 0},  // orange
	{R: 156, G: 39, B: 176}, // purple
	{R: 0, G: 188, B: 212},  // cyan
	{R: 244, G: 67, B: 54},  // red
	{R: 255, G: 235, B: 59}, // yellow
	{R: 121, G: 85, B: 72},  // brown
}

// Page layout constants (A4 landscape in mm).
const (
	pageWidth    = 297.0
	pageHeight   = 210.0
	marginLeft   = 15.0
	marginRight  = 15.0
	marginTop    = 15.0
	marginBottom = 15.0
	headerHeight = 12.0
	statsHeight  = 20.0
	drawAreaTop  = marginTop + headerHeight + 5.0
)

// ExportPDF generates a PDF document containing the cut optimization results.
// Each sheet result is rendered on its own page with a visual layout diagram,
// followed by a summary page with overall statistics.
func ExportPDF(path string, result model.OptimizeResult, settings model.CutSettings) error {
	if len(result.Sheets) == 0 {
		return fmt.Errorf("no sheets to export")
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, marginBottom)

	// Render each sheet on its own page
	for i, sheet := range result.Sheets {
		pdf.AddPage()
		renderSheetPage(pdf, sheet, settings, i+1)
	}

	// Summary page
	pdf.AddPage()
	renderSummaryPage(pdf, result, settings)

	return pdf.OutputFileAndClose(path)
}

// renderSheetPage draws a single sheet result on the current PDF page.
func renderSheetPage(pdf *fpdf.Fpdf, sheet model.SheetResult, settings model.CutSettings, sheetNum int) {
	// Title
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(marginLeft, marginTop)
	title := fmt.Sprintf("Sheet %d: %s (%.0f x %.0f mm)", sheetNum, sheet.Stock.Label, sheet.Stock.Width, sheet.Stock.Height)
	pdf.CellFormat(pageWidth-marginLeft-marginRight, headerHeight, title, "", 0, "L", false, 0, "")

	// Stats line
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetXY(marginLeft, marginTop+headerHeight)
	stats := fmt.Sprintf("Parts: %d | Used area: %.0f mm² | Total area: %.0f mm² | Efficiency: %.1f%%",
		len(sheet.Placements), sheet.UsedArea(), sheet.TotalArea(), sheet.Efficiency())
	pdf.CellFormat(pageWidth-marginLeft-marginRight, 5, stats, "", 0, "L", false, 0, "")

	// Calculate drawing area
	drawWidth := pageWidth - marginLeft - marginRight
	drawHeight := pageHeight - drawAreaTop - marginBottom - statsHeight

	// Calculate scale to fit stock sheet within drawing area
	scaleX := drawWidth / sheet.Stock.Width
	scaleY := drawHeight / sheet.Stock.Height
	scale := math.Min(scaleX, scaleY)

	canvasW := sheet.Stock.Width * scale
	canvasH := sheet.Stock.Height * scale

	// Center the drawing horizontally
	offsetX := marginLeft + (drawWidth-canvasW)/2
	offsetY := drawAreaTop

	// Draw stock sheet background (wood color)
	pdf.SetFillColor(210, 180, 140)
	pdf.SetDrawColor(100, 100, 100)
	pdf.SetLineWidth(0.5)
	pdf.Rect(offsetX, offsetY, canvasW, canvasH, "FD")

	// Draw stock holding tab exclusion zones
	drawStockTabs(pdf, sheet.Stock, settings, scale, offsetX, offsetY)

	// Draw placed parts
	for i, p := range sheet.Placements {
		col := partColors[i%len(partColors)]
		pw := p.PlacedWidth() * scale
		ph := p.PlacedHeight() * scale
		px := offsetX + p.X*scale
		py := offsetY + p.Y*scale

		// Part fill
		pdf.SetFillColor(col.R, col.G, col.B)
		pdf.SetDrawColor(30, 30, 30)
		pdf.SetLineWidth(0.3)
		pdf.Rect(px, py, pw, ph, "FD")

		// Part label (only if rectangle is large enough)
		if pw > 15 && ph > 8 {
			pdf.SetFont("Helvetica", "", labelFontSize(pw, ph))
			pdf.SetTextColor(0, 0, 0)

			label := p.Part.Label
			dims := fmt.Sprintf("%.0fx%.0f", p.Part.Width, p.Part.Height)

			// Draw label centered in the part rectangle
			labelW := pdf.GetStringWidth(label)
			dimsW := pdf.GetStringWidth(dims)

			// First line: label
			if labelW < pw-2 {
				pdf.SetXY(px+(pw-labelW)/2, py+ph/2-4)
				pdf.CellFormat(labelW, 4, label, "", 0, "C", false, 0, "")
			}

			// Second line: dimensions
			if ph > 14 && dimsW < pw-2 {
				pdf.SetXY(px+(pw-dimsW)/2, py+ph/2)
				pdf.CellFormat(dimsW, 4, dims, "", 0, "C", false, 0, "")
			}
		}
	}

	// Dimension annotations along the edges
	drawDimensionAnnotations(pdf, sheet.Stock, scale, offsetX, offsetY, canvasW, canvasH)

	// Parts legend at bottom of page
	drawPartsLegend(pdf, sheet, offsetY+canvasH+5)
}

// drawStockTabs renders the stock sheet holding tab exclusion zones.
func drawStockTabs(pdf *fpdf.Fpdf, stock model.StockSheet, settings model.CutSettings, scale, offsetX, offsetY float64) {
	tabConfig := stock.Tabs
	if !tabConfig.Enabled {
		tabConfig = settings.StockTabs
		if !tabConfig.Enabled {
			return
		}
	}

	var zones []model.TabZone

	if tabConfig.AdvancedMode {
		zones = tabConfig.CustomZones
	} else {
		if tabConfig.TopPadding > 0 {
			zones = append(zones, model.TabZone{X: 0, Y: 0, Width: stock.Width, Height: tabConfig.TopPadding})
		}
		if tabConfig.BottomPadding > 0 {
			zones = append(zones, model.TabZone{X: 0, Y: stock.Height - tabConfig.BottomPadding, Width: stock.Width, Height: tabConfig.BottomPadding})
		}
		if tabConfig.LeftPadding > 0 {
			zones = append(zones, model.TabZone{X: 0, Y: 0, Width: tabConfig.LeftPadding, Height: stock.Height})
		}
		if tabConfig.RightPadding > 0 {
			zones = append(zones, model.TabZone{X: stock.Width - tabConfig.RightPadding, Y: 0, Width: tabConfig.RightPadding, Height: stock.Height})
		}
	}

	for _, zone := range zones {
		zx := offsetX + zone.X*scale
		zy := offsetY + zone.Y*scale
		zw := zone.Width * scale
		zh := zone.Height * scale

		// Semi-transparent red zone (simulate with light fill + hatching)
		pdf.SetFillColor(255, 200, 200)
		pdf.SetDrawColor(200, 0, 0)
		pdf.SetLineWidth(0.3)
		pdf.Rect(zx, zy, zw, zh, "FD")

		// Draw diagonal hatch lines for visual distinction
		drawHatchPattern(pdf, zx, zy, zw, zh)

		// Label for larger zones
		if zw > 20 && zh > 8 {
			pdf.SetFont("Helvetica", "B", 6)
			pdf.SetTextColor(180, 0, 0)
			labelW := pdf.GetStringWidth("NO CUT")
			pdf.SetXY(zx+(zw-labelW)/2, zy+zh/2-2)
			pdf.CellFormat(labelW, 4, "NO CUT", "", 0, "C", false, 0, "")
		}
	}

	// Reset text color
	pdf.SetTextColor(0, 0, 0)
}

// drawHatchPattern draws diagonal lines inside a rectangle to indicate exclusion zones.
func drawHatchPattern(pdf *fpdf.Fpdf, x, y, w, h float64) {
	pdf.SetDrawColor(200, 0, 0)
	pdf.SetLineWidth(0.15)

	spacing := 4.0
	maxDist := w + h

	for d := spacing; d < maxDist; d += spacing {
		// Line from bottom-left to top-right diagonal
		x1 := x + math.Max(0, d-h)
		y1 := y + math.Min(h, d)
		x2 := x + math.Min(w, d)
		y2 := y + math.Max(0, d-w)

		pdf.Line(x1, y1, x2, y2)
	}
}

// drawDimensionAnnotations adds width and height dimension labels outside the sheet rectangle.
func drawDimensionAnnotations(pdf *fpdf.Fpdf, stock model.StockSheet, scale, offsetX, offsetY, canvasW, canvasH float64) {
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(80, 80, 80)

	// Width annotation (below the sheet)
	widthLabel := fmt.Sprintf("%.0f mm", stock.Width)
	wLabelW := pdf.GetStringWidth(widthLabel)
	pdf.SetXY(offsetX+(canvasW-wLabelW)/2, offsetY+canvasH+1)
	pdf.CellFormat(wLabelW, 4, widthLabel, "", 0, "C", false, 0, "")

	// Height annotation (to the left of the sheet, rotated)
	heightLabel := fmt.Sprintf("%.0f mm", stock.Height)
	pdf.TransformBegin()
	pdf.TransformRotate(90, offsetX-3, offsetY+canvasH/2)
	hLabelW := pdf.GetStringWidth(heightLabel)
	pdf.SetXY(offsetX-3-hLabelW/2, offsetY+canvasH/2-2)
	pdf.CellFormat(hLabelW, 4, heightLabel, "", 0, "C", false, 0, "")
	pdf.TransformEnd()

	// Reset text color
	pdf.SetTextColor(0, 0, 0)
}

// drawPartsLegend renders a compact legend of placed parts at the bottom of the sheet page.
func drawPartsLegend(pdf *fpdf.Fpdf, sheet model.SheetResult, startY float64) {
	if len(sheet.Placements) == 0 {
		return
	}

	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(marginLeft, startY)
	pdf.CellFormat(30, 4, "Parts placed:", "", 0, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 7)
	xPos := marginLeft + 32
	maxX := pageWidth - marginRight

	for i, p := range sheet.Placements {
		col := partColors[i%len(partColors)]
		label := fmt.Sprintf("%s (%.0fx%.0f)", p.Part.Label, p.Part.Width, p.Part.Height)
		if p.Rotated {
			label += " R"
		}
		labelW := pdf.GetStringWidth(label) + 6

		// Wrap to next line if needed
		if xPos+labelW > maxX {
			startY += 5
			xPos = marginLeft
		}

		// Color swatch
		pdf.SetFillColor(col.R, col.G, col.B)
		pdf.Rect(xPos, startY+0.5, 3, 3, "F")

		// Label text
		pdf.SetXY(xPos+4, startY)
		pdf.CellFormat(labelW-4, 4, label, "", 0, "L", false, 0, "")

		xPos += labelW + 2
	}
}

// renderSummaryPage draws the final summary page with overall statistics.
func renderSummaryPage(pdf *fpdf.Fpdf, result model.OptimizeResult, settings model.CutSettings) {
	// Title
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetXY(marginLeft, marginTop)
	pdf.CellFormat(pageWidth-marginLeft-marginRight, 10, "Cut Optimization Summary", "", 0, "L", false, 0, "")

	// Separator line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(marginLeft, marginTop+12, pageWidth-marginRight, marginTop+12)

	y := marginTop + 18

	// Overall statistics
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetXY(marginLeft, y)
	pdf.CellFormat(100, 7, "Overall Statistics", "", 0, "L", false, 0, "")
	y += 9

	summaryItems := []struct {
		label string
		value string
	}{
		{"Total Sheets Used", fmt.Sprintf("%d", len(result.Sheets))},
		{"Overall Efficiency", fmt.Sprintf("%.1f%%", result.TotalEfficiency())},
		{"Total Parts Placed", fmt.Sprintf("%d", countParts(result))},
		{"Unplaced Parts", fmt.Sprintf("%d", len(result.UnplacedParts))},
	}

	pdf.SetFont("Helvetica", "", 10)
	for _, item := range summaryItems {
		pdf.SetXY(marginLeft+5, y)
		pdf.CellFormat(60, 6, item.label+":", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(40, 6, item.value, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		y += 7
	}

	y += 5

	// Per-sheet breakdown table
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetXY(marginLeft, y)
	pdf.CellFormat(100, 7, "Sheet Breakdown", "", 0, "L", false, 0, "")
	y += 9

	// Table header
	colWidths := []float64{20, 60, 50, 50, 35, 50}
	headers := []string{"Sheet", "Stock", "Dimensions", "Parts", "Efficiency", "Used / Total Area"}

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	xPos := marginLeft
	for i, header := range headers {
		pdf.SetXY(xPos, y)
		pdf.CellFormat(colWidths[i], 6, header, "1", 0, "C", true, 0, "")
		xPos += colWidths[i]
	}
	y += 6

	// Table rows
	pdf.SetFont("Helvetica", "", 9)
	for i, sheet := range result.Sheets {
		xPos = marginLeft
		rowData := []string{
			fmt.Sprintf("%d", i+1),
			sheet.Stock.Label,
			fmt.Sprintf("%.0f x %.0f mm", sheet.Stock.Width, sheet.Stock.Height),
			fmt.Sprintf("%d", len(sheet.Placements)),
			fmt.Sprintf("%.1f%%", sheet.Efficiency()),
			fmt.Sprintf("%.0f / %.0f mm²", sheet.UsedArea(), sheet.TotalArea()),
		}

		// Alternate row background
		if i%2 == 0 {
			pdf.SetFillColor(245, 245, 245)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		for j, cell := range rowData {
			pdf.SetXY(xPos, y)
			pdf.CellFormat(colWidths[j], 6, cell, "1", 0, "C", true, 0, "")
			xPos += colWidths[j]
		}
		y += 6
	}

	// Unplaced parts warning
	if len(result.UnplacedParts) > 0 {
		y += 8
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetTextColor(200, 0, 0)
		pdf.SetXY(marginLeft, y)
		pdf.CellFormat(200, 7, "WARNING: Unplaced Parts", "", 0, "L", false, 0, "")
		y += 8

		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(0, 0, 0)

		for _, part := range result.UnplacedParts {
			pdf.SetXY(marginLeft+5, y)
			text := fmt.Sprintf("- %s: %.0f x %.0f mm (qty: %d)", part.Label, part.Width, part.Height, part.Quantity)
			pdf.CellFormat(200, 5, text, "", 0, "L", false, 0, "")
			y += 5
		}
	}

	// Cut settings summary
	y += 8
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(marginLeft, y)
	pdf.CellFormat(100, 7, "Cut Settings", "", 0, "L", false, 0, "")
	y += 9

	settingsItems := []struct {
		label string
		value string
	}{
		{"Kerf Width", fmt.Sprintf("%.1f mm", settings.KerfWidth)},
		{"Edge Trim", fmt.Sprintf("%.1f mm", settings.EdgeTrim)},
		{"Tool Diameter", fmt.Sprintf("%.1f mm", settings.ToolDiameter)},
		{"Material Thickness", fmt.Sprintf("%.1f mm", settings.CutDepth)},
		{"Pass Depth", fmt.Sprintf("%.1f mm", settings.PassDepth)},
	}

	pdf.SetFont("Helvetica", "", 9)
	for _, item := range settingsItems {
		pdf.SetXY(marginLeft+5, y)
		pdf.CellFormat(50, 5, item.label+":", "", 0, "L", false, 0, "")
		pdf.CellFormat(30, 5, item.value, "", 0, "L", false, 0, "")
		y += 5
	}

	// Footer
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetXY(marginLeft, pageHeight-marginBottom)
	pdf.CellFormat(pageWidth-marginLeft-marginRight, 4, "Generated by CNCCalculator - CNC Cut List Optimizer", "", 0, "C", false, 0, "")
}

// labelFontSize returns an appropriate font size based on the rectangle dimensions.
func labelFontSize(w, h float64) float64 {
	minDim := math.Min(w, h)
	switch {
	case minDim > 40:
		return 8
	case minDim > 20:
		return 7
	default:
		return 6
	}
}

// countParts returns the total number of placed parts across all sheets.
func countParts(result model.OptimizeResult) int {
	total := 0
	for _, s := range result.Sheets {
		total += len(s.Placements)
	}
	return total
}
