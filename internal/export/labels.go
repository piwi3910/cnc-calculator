// Package export provides functionality for exporting cut optimization results
// to various file formats including QR-coded part labels.
package export

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/piwi3910/SlabCut/internal/model"
	qrcode "github.com/skip2/go-qrcode"
)

// LabelInfo holds the data encoded into each part label's QR code.
type LabelInfo struct {
	PartLabel  string  `json:"label"`
	Width      float64 `json:"width_mm"`
	Height     float64 `json:"height_mm"`
	SheetIndex int     `json:"sheet"`
	SheetLabel string  `json:"sheet_label"`
	Rotated    bool    `json:"rotated"`
	X          float64 `json:"x_mm"`
	Y          float64 `json:"y_mm"`
}

// Label layout constants for Avery 5160-compatible labels (3 columns, 10 rows per page).
// Each label cell is approximately 66.7mm x 25.4mm on US Letter paper.
const (
	labelPageWidth  = 215.9 // US Letter width in mm
	labelPageHeight = 279.4 // US Letter height in mm
	labelMarginTop  = 12.7  // mm
	labelMarginLeft = 4.8   // mm
	labelWidth      = 66.7  // mm per label
	labelHeight     = 25.4  // mm per label
	labelCols       = 3
	labelRows       = 10
	labelsPerPage   = labelCols * labelRows
	qrSize          = 20.0 // QR code size in mm
	labelPadding    = 2.0  // mm internal padding
)

// ExportLabels generates a PDF of QR-coded labels for all placed parts.
// Each label contains the part name, dimensions, and a QR code encoding
// part metadata as JSON. Labels are laid out on a standard label sheet
// format (Avery 5160 / 3 columns x 10 rows on US Letter).
func ExportLabels(path string, result model.OptimizeResult) error {
	if len(result.Sheets) == 0 {
		return fmt.Errorf("no sheets to generate labels for")
	}

	// Collect all placed parts across all sheets
	var labels []LabelInfo
	for sheetIdx, sheet := range result.Sheets {
		for _, p := range sheet.Placements {
			labels = append(labels, LabelInfo{
				PartLabel:  p.Part.Label,
				Width:      p.Part.Width,
				Height:     p.Part.Height,
				SheetIndex: sheetIdx + 1,
				SheetLabel: sheet.Stock.Label,
				Rotated:    p.Rotated,
				X:          p.X,
				Y:          p.Y,
			})
		}
	}

	if len(labels) == 0 {
		return fmt.Errorf("no parts placed to generate labels for")
	}

	pdf := fpdf.New("P", "mm", "Letter", "")
	pdf.SetAutoPageBreak(false, 0)

	for i, label := range labels {
		// Add new page when needed
		if i%labelsPerPage == 0 {
			pdf.AddPage()
		}

		posOnPage := i % labelsPerPage
		col := posOnPage % labelCols
		row := posOnPage / labelCols

		x := labelMarginLeft + float64(col)*labelWidth
		y := labelMarginTop + float64(row)*labelHeight

		if err := renderLabel(pdf, x, y, label); err != nil {
			return fmt.Errorf("failed to render label for %q: %w", label.PartLabel, err)
		}
	}

	return pdf.OutputFileAndClose(path)
}

// renderLabel draws a single label at the given position.
func renderLabel(pdf *fpdf.Fpdf, x, y float64, info LabelInfo) error {
	// Draw light border for cutting guide
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.1)
	pdf.Rect(x, y, labelWidth, labelHeight, "D")

	// Generate QR code PNG bytes
	qrData, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal label info: %w", err)
	}

	qrPNG, err := qrcode.Encode(string(qrData), qrcode.Medium, 256)
	if err != nil {
		return fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Register QR image with a unique name
	imgName := fmt.Sprintf("qr_%s_%d_%d", info.PartLabel, info.SheetIndex, int(info.X*1000+info.Y))
	pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qrPNG))

	// Place QR code on the right side of the label
	qrX := x + labelWidth - qrSize - labelPadding
	qrY := y + (labelHeight-qrSize)/2
	pdf.ImageOptions(imgName, qrX, qrY, qrSize, qrSize, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Text area (left side of label)
	textX := x + labelPadding
	textW := labelWidth - qrSize - 3*labelPadding

	// Part label (bold, larger)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(textX, y+labelPadding)

	// Truncate label if too long
	partLabel := info.PartLabel
	if pdf.GetStringWidth(partLabel) > textW {
		for len(partLabel) > 0 && pdf.GetStringWidth(partLabel+"...") > textW {
			partLabel = partLabel[:len(partLabel)-1]
		}
		partLabel += "..."
	}
	pdf.CellFormat(textW, 4.5, partLabel, "", 1, "L", false, 0, "")

	// Dimensions
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetXY(textX, y+labelPadding+5)
	dims := fmt.Sprintf("%.0f x %.0f mm", info.Width, info.Height)
	pdf.CellFormat(textW, 3.5, dims, "", 1, "L", false, 0, "")

	// Sheet and position info
	pdf.SetFont("Helvetica", "", 6)
	pdf.SetTextColor(100, 100, 100)
	pdf.SetXY(textX, y+labelPadding+9)
	sheetInfo := fmt.Sprintf("Sheet %d @ (%.0f, %.0f)", info.SheetIndex, info.X, info.Y)
	pdf.CellFormat(textW, 3, sheetInfo, "", 1, "L", false, 0, "")

	// Rotation indicator
	if info.Rotated {
		pdf.SetXY(textX, y+labelPadding+12.5)
		pdf.SetFont("Helvetica", "I", 6)
		pdf.SetTextColor(150, 100, 0)
		pdf.CellFormat(textW, 3, "Rotated 90\xb0", "", 0, "L", false, 0, "")
	}

	// Reset text color
	pdf.SetTextColor(0, 0, 0)

	return nil
}

// CollectLabelInfos extracts label information from an optimization result
// for use in testing or alternative export formats.
func CollectLabelInfos(result model.OptimizeResult) []LabelInfo {
	var labels []LabelInfo
	for sheetIdx, sheet := range result.Sheets {
		for _, p := range sheet.Placements {
			labels = append(labels, LabelInfo{
				PartLabel:  p.Part.Label,
				Width:      p.Part.Width,
				Height:     p.Part.Height,
				SheetIndex: sheetIdx + 1,
				SheetLabel: sheet.Stock.Label,
				Rotated:    p.Rotated,
				X:          p.X,
				Y:          p.Y,
			})
		}
	}
	return labels
}
