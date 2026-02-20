package model

import "math"

// EdgeBandingSummary holds the calculated edge banding requirements for a project.
type EdgeBandingSummary struct {
	TotalLinearMM    float64 `json:"total_linear_mm"`     // Total banding length in mm (no waste)
	TotalLinearM     float64 `json:"total_linear_m"`      // Total banding length in meters (no waste)
	WastePercent     float64 `json:"waste_percent"`       // Waste percentage applied
	TotalWithWasteMM float64 `json:"total_with_waste_mm"` // Total with waste in mm
	TotalWithWasteM  float64 `json:"total_with_waste_m"`  // Total with waste in meters
	PartCount        int     `json:"part_count"`          // Number of individual pieces needing banding
	EdgeCount        int     `json:"edge_count"`          // Total number of edges needing banding
}

// CalculateEdgeBanding computes the total edge banding needed for a list of parts.
// wastePercent is the additional percentage to add for waste (e.g., 10 for 10%).
func CalculateEdgeBanding(parts []Part, wastePercent float64) EdgeBandingSummary {
	var totalMM float64
	var partCount, edgeCount int

	for _, p := range parts {
		if !p.EdgeBanding.HasAny() {
			continue
		}
		lengthPerPiece := p.EdgeBanding.LinearLength(p.Width, p.Height)
		edgesPerPiece := p.EdgeBanding.EdgeCount()

		totalMM += lengthPerPiece * float64(p.Quantity)
		partCount += p.Quantity
		edgeCount += edgesPerPiece * p.Quantity
	}

	wasteFactor := 1.0 + (wastePercent / 100.0)
	totalWithWaste := totalMM * wasteFactor

	return EdgeBandingSummary{
		TotalLinearMM:    totalMM,
		TotalLinearM:     totalMM / 1000.0,
		WastePercent:     wastePercent,
		TotalWithWasteMM: math.Ceil(totalWithWaste), // Round up
		TotalWithWasteM:  math.Ceil(totalWithWaste) / 1000.0,
		PartCount:        partCount,
		EdgeCount:        edgeCount,
	}
}

// PerPartEdgeBanding returns a per-part breakdown of edge banding needs.
type PerPartEdgeBanding struct {
	Label         string  `json:"label"`
	Width         float64 `json:"width"`
	Height        float64 `json:"height"`
	Quantity      int     `json:"quantity"`
	Edges         string  `json:"edges"`           // e.g., "T+B+L+R"
	LengthPerUnit float64 `json:"length_per_unit"` // mm per piece
	TotalLength   float64 `json:"total_length"`    // mm for all pieces
}

// CalculatePerPartEdgeBanding returns a breakdown of banding per part type.
func CalculatePerPartEdgeBanding(parts []Part) []PerPartEdgeBanding {
	var results []PerPartEdgeBanding
	for _, p := range parts {
		if !p.EdgeBanding.HasAny() {
			continue
		}
		lengthPerUnit := p.EdgeBanding.LinearLength(p.Width, p.Height)
		results = append(results, PerPartEdgeBanding{
			Label:         p.Label,
			Width:         p.Width,
			Height:        p.Height,
			Quantity:      p.Quantity,
			Edges:         p.EdgeBanding.String(),
			LengthPerUnit: lengthPerUnit,
			TotalLength:   lengthPerUnit * float64(p.Quantity),
		})
	}
	return results
}
