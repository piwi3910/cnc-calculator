package model

import (
	"math"
	"sort"

	"github.com/google/uuid"
)

// Offcut represents a usable rectangular remnant area left over after cutting.
type Offcut struct {
	ID            string  `json:"id"`
	SheetLabel    string  `json:"sheet_label"`    // Which sheet it came from
	SheetIndex    int     `json:"sheet_index"`    // Index of the source sheet in the result
	X             float64 `json:"x"`              // Position on the sheet (mm from left)
	Y             float64 `json:"y"`              // Position on the sheet (mm from top)
	Width         float64 `json:"width"`          // Usable width (mm)
	Height        float64 `json:"height"`         // Usable height (mm)
	PricePerSheet float64 `json:"price_per_sheet"` // Inherited price proportional to area (0 if not set)
}

// Area returns the area of the offcut in square mm.
func (o Offcut) Area() float64 {
	return o.Width * o.Height
}

// ToStockSheet converts an offcut into a stock sheet for reuse in future projects.
func (o Offcut) ToStockSheet() StockSheet {
	label := "Offcut " + o.SheetLabel
	sheet := NewStockSheet(label, o.Width, o.Height, 1)
	sheet.PricePerSheet = o.PricePerSheet
	return sheet
}

// MinOffcutDimension is the minimum width or height (in mm) for a remnant
// to be considered a usable offcut. Remnants smaller than this are waste.
const MinOffcutDimension = 50.0

// MinOffcutArea is the minimum area (in sq mm) for a remnant to be considered usable.
const MinOffcutArea = 10000.0 // 100mm x 100mm equivalent

// DetectOffcuts analyzes a SheetResult and identifies rectangular remnant areas
// that are large enough to be reused. It uses a skyline-based approach to find
// the largest usable rectangles in the unused areas.
func DetectOffcuts(sr SheetResult, sheetIndex int, kerf float64) []Offcut {
	sheetW := sr.Stock.Width
	sheetH := sr.Stock.Height

	if len(sr.Placements) == 0 {
		// Entire sheet is an offcut (unlikely but handle it)
		return []Offcut{{
			ID:            uuid.New().String()[:8],
			SheetLabel:    sr.Stock.Label,
			SheetIndex:    sheetIndex,
			X:             0,
			Y:             0,
			Width:         sheetW,
			Height:        sheetH,
			PricePerSheet: sr.Stock.PricePerSheet,
		}}
	}

	// Find the bounding box of all placed parts to identify large unused strips
	var maxPartRight, maxPartBottom float64
	for _, p := range sr.Placements {
		right := p.X + p.PlacedWidth() + kerf
		bottom := p.Y + p.PlacedHeight() + kerf
		if right > maxPartRight {
			maxPartRight = right
		}
		if bottom > maxPartBottom {
			maxPartBottom = bottom
		}
	}

	var offcuts []Offcut

	// Right strip: area to the right of all parts
	rightStripW := sheetW - maxPartRight
	if rightStripW >= MinOffcutDimension && sheetH >= MinOffcutDimension && rightStripW*sheetH >= MinOffcutArea {
		offcuts = append(offcuts, Offcut{
			ID:         uuid.New().String()[:8],
			SheetLabel: sr.Stock.Label,
			SheetIndex: sheetIndex,
			X:          maxPartRight,
			Y:          0,
			Width:      rightStripW,
			Height:     sheetH,
		})
	}

	// Bottom strip: area below all parts (only up to the right edge of parts to avoid overlap with right strip)
	bottomStripH := sheetH - maxPartBottom
	usableBottomW := math.Min(maxPartRight, sheetW)
	if bottomStripH >= MinOffcutDimension && usableBottomW >= MinOffcutDimension && bottomStripH*usableBottomW >= MinOffcutArea {
		offcuts = append(offcuts, Offcut{
			ID:         uuid.New().String()[:8],
			SheetLabel: sr.Stock.Label,
			SheetIndex: sheetIndex,
			X:          0,
			Y:          maxPartBottom,
			Width:      usableBottomW,
			Height:     bottomStripH,
		})
	}

	// Assign proportional pricing to offcuts
	if sr.Stock.PricePerSheet > 0 {
		totalSheetArea := sheetW * sheetH
		for i := range offcuts {
			offcuts[i].PricePerSheet = (offcuts[i].Area() / totalSheetArea) * sr.Stock.PricePerSheet
		}
	}

	// Sort by area descending (largest offcuts first)
	sort.Slice(offcuts, func(i, j int) bool {
		return offcuts[i].Area() > offcuts[j].Area()
	})

	return offcuts
}

// DetectAllOffcuts finds offcuts across all sheets in an optimization result.
func DetectAllOffcuts(result OptimizeResult, kerf float64) []Offcut {
	var all []Offcut
	for i, sheet := range result.Sheets {
		offcuts := DetectOffcuts(sheet, i, kerf)
		all = append(all, offcuts...)
	}
	return all
}

// TotalOffcutArea returns the total area of all offcuts in square mm.
func TotalOffcutArea(offcuts []Offcut) float64 {
	var total float64
	for _, o := range offcuts {
		total += o.Area()
	}
	return total
}
