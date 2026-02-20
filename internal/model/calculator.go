package model

import "math"

// PurchaseEstimate holds the results of a sheet purchasing calculation.
type PurchaseEstimate struct {
	TotalPartArea     float64 `json:"total_part_area"`     // Total area of all parts (sq mm)
	TotalBoardFeet    float64 `json:"total_board_feet"`    // Total area in board feet (1 bf = 144 sq in = 92903.04 sq mm)
	SheetArea         float64 `json:"sheet_area"`          // Area of one sheet (sq mm)
	SheetsNeededExact float64 `json:"sheets_needed_exact"` // Exact fractional number of sheets
	SheetsNeededMin   int     `json:"sheets_needed_min"`   // Minimum sheets (ceiling of exact)
	SheetsWithWaste   int     `json:"sheets_with_waste"`   // Recommended sheets including waste factor
	WastePercent      float64 `json:"waste_percent"`       // Waste factor applied (e.g., 15 for 15%)
	EstimatedCost     float64 `json:"estimated_cost"`      // Total cost if pricing available
	PricePerSheet     float64 `json:"price_per_sheet"`     // Price used for estimation
	KerfWidth         float64 `json:"kerf_width"`          // Kerf width used in calculation
}

// sqmmPerBoardFoot is the number of square millimeters in one board foot.
// 1 board foot = 12" x 12" x 1" (area) = 144 sq inches = 144 * 645.16 sq mm = 92903.04 sq mm.
const sqmmPerBoardFoot = 92903.04

// CalculatePurchaseEstimate computes how many sheets to buy for a given cut list.
// It accounts for kerf waste and an additional waste percentage factor.
func CalculatePurchaseEstimate(parts []Part, sheetWidth, sheetHeight, kerfWidth, wastePercent, pricePerSheet float64) PurchaseEstimate {
	// Calculate total part area including kerf allowance per part
	var totalPartArea float64
	for _, p := range parts {
		partW := p.Width + kerfWidth
		partH := p.Height + kerfWidth
		totalPartArea += partW * partH * float64(p.Quantity)
	}

	sheetArea := sheetWidth * sheetHeight
	if sheetArea <= 0 {
		return PurchaseEstimate{
			TotalPartArea:  totalPartArea,
			TotalBoardFeet: totalPartArea / sqmmPerBoardFoot,
			WastePercent:   wastePercent,
		}
	}

	exactSheets := totalPartArea / sheetArea
	minSheets := int(math.Ceil(exactSheets))

	// Apply waste factor
	wasteFactor := 1.0 + (wastePercent / 100.0)
	sheetsWithWaste := int(math.Ceil(exactSheets * wasteFactor))
	if sheetsWithWaste < minSheets {
		sheetsWithWaste = minSheets
	}

	estimatedCost := float64(sheetsWithWaste) * pricePerSheet

	return PurchaseEstimate{
		TotalPartArea:     totalPartArea,
		TotalBoardFeet:    totalPartArea / sqmmPerBoardFoot,
		SheetArea:         sheetArea,
		SheetsNeededExact: exactSheets,
		SheetsNeededMin:   minSheets,
		SheetsWithWaste:   sheetsWithWaste,
		WastePercent:      wastePercent,
		EstimatedCost:     estimatedCost,
		PricePerSheet:     pricePerSheet,
		KerfWidth:         kerfWidth,
	}
}
