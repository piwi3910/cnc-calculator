package model

import (
	"math"
	"testing"
)

func TestCalculatePurchaseEstimateBasic(t *testing.T) {
	parts := []Part{
		{Label: "Part1", Width: 500, Height: 300, Quantity: 4},
	}
	est := CalculatePurchaseEstimate(parts, 2440, 1220, 3.0, 15.0, 45.00)

	// Each part with kerf: 503 x 303 = 152409 sq mm, x4 = 609636
	expectedArea := 503.0 * 303.0 * 4
	if math.Abs(est.TotalPartArea-expectedArea) > 0.1 {
		t.Errorf("expected total area %.1f, got %.1f", expectedArea, est.TotalPartArea)
	}

	if est.TotalBoardFeet <= 0 {
		t.Error("expected positive board feet")
	}

	if est.SheetsNeededMin < 1 {
		t.Error("expected at least 1 sheet")
	}

	if est.SheetsWithWaste < est.SheetsNeededMin {
		t.Error("sheets with waste should be >= minimum sheets")
	}

	if est.EstimatedCost <= 0 {
		t.Error("expected positive cost")
	}
}

func TestCalculatePurchaseEstimateZeroSheetArea(t *testing.T) {
	parts := []Part{
		{Label: "P1", Width: 100, Height: 100, Quantity: 1},
	}
	est := CalculatePurchaseEstimate(parts, 0, 0, 0, 10, 0)
	if est.SheetsNeededMin != 0 {
		t.Errorf("expected 0 sheets for zero sheet area, got %d", est.SheetsNeededMin)
	}
	if est.TotalPartArea <= 0 {
		t.Error("expected positive total part area even with zero sheet")
	}
}

func TestCalculatePurchaseEstimateMultipleParts(t *testing.T) {
	parts := []Part{
		{Label: "Shelf", Width: 800, Height: 300, Quantity: 6},
		{Label: "Side", Width: 600, Height: 400, Quantity: 2},
		{Label: "Back", Width: 1200, Height: 800, Quantity: 1},
	}
	est := CalculatePurchaseEstimate(parts, 2440, 1220, 3.2, 20.0, 55.00)

	if est.SheetsNeededMin < 1 {
		t.Error("expected at least 1 sheet")
	}
	if est.SheetsWithWaste < est.SheetsNeededMin {
		t.Errorf("waste sheets (%d) < min sheets (%d)", est.SheetsWithWaste, est.SheetsNeededMin)
	}
	if est.EstimatedCost != float64(est.SheetsWithWaste)*55.00 {
		t.Errorf("expected cost %.2f, got %.2f", float64(est.SheetsWithWaste)*55.00, est.EstimatedCost)
	}
}

func TestCalculatePurchaseEstimateNoWaste(t *testing.T) {
	parts := []Part{
		{Label: "P1", Width: 500, Height: 300, Quantity: 1},
	}
	est := CalculatePurchaseEstimate(parts, 2440, 1220, 0, 0, 0)
	if est.SheetsNeededMin != 1 {
		t.Errorf("expected 1 sheet, got %d", est.SheetsNeededMin)
	}
	if est.SheetsWithWaste != 1 {
		t.Errorf("expected 1 sheet with 0%% waste, got %d", est.SheetsWithWaste)
	}
}

func TestCalculatePurchaseEstimateExactFit(t *testing.T) {
	// Parts that exactly fill one sheet
	parts := []Part{
		{Label: "Full", Width: 2440, Height: 1220, Quantity: 1},
	}
	est := CalculatePurchaseEstimate(parts, 2440, 1220, 0, 0, 30.00)
	if est.SheetsNeededMin != 1 {
		t.Errorf("expected exactly 1 sheet, got %d", est.SheetsNeededMin)
	}
}

func TestBoardFeetConversion(t *testing.T) {
	// 1 board foot = 144 sq in = 92903.04 sq mm
	parts := []Part{
		{Label: "P1", Width: 304.8, Height: 304.8, Quantity: 1}, // ~12" x 12" = 1 board foot
	}
	est := CalculatePurchaseEstimate(parts, 2440, 1220, 0, 0, 0)
	// 304.8 * 304.8 = 92903.04 sq mm = exactly 1 board foot
	if math.Abs(est.TotalBoardFeet-1.0) > 0.01 {
		t.Errorf("expected ~1.0 board feet, got %.4f", est.TotalBoardFeet)
	}
}
