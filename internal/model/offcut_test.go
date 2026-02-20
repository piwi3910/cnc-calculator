package model

import (
	"testing"
)

func TestDetectOffcutsEmptySheet(t *testing.T) {
	sr := SheetResult{
		Stock:      StockSheet{Label: "Test", Width: 2440, Height: 1220},
		Placements: nil,
	}
	offcuts := DetectOffcuts(sr, 0, 3.0)
	if len(offcuts) != 1 {
		t.Fatalf("expected 1 offcut for empty sheet, got %d", len(offcuts))
	}
	if offcuts[0].Width != 2440 || offcuts[0].Height != 1220 {
		t.Errorf("expected full sheet as offcut, got %.0fx%.0f", offcuts[0].Width, offcuts[0].Height)
	}
}

func TestDetectOffcutsRightStrip(t *testing.T) {
	sr := SheetResult{
		Stock: StockSheet{Label: "Sheet1", Width: 2440, Height: 1220},
		Placements: []Placement{
			{Part: Part{Label: "P1", Width: 1000, Height: 1220}, X: 0, Y: 0},
		},
	}
	offcuts := DetectOffcuts(sr, 0, 3.0)
	// Should find a right strip: X=1003, Width=1437, Height=1220
	foundRight := false
	for _, o := range offcuts {
		if o.X > 900 && o.Width > 1000 {
			foundRight = true
			break
		}
	}
	if !foundRight {
		t.Error("expected to find right strip offcut")
	}
}

func TestDetectOffcutsBottomStrip(t *testing.T) {
	sr := SheetResult{
		Stock: StockSheet{Label: "Sheet1", Width: 2440, Height: 1220},
		Placements: []Placement{
			{Part: Part{Label: "P1", Width: 2440, Height: 500}, X: 0, Y: 0},
		},
	}
	offcuts := DetectOffcuts(sr, 0, 3.0)
	// Should find a bottom strip: Y=503, Height=717, Width=2440
	foundBottom := false
	for _, o := range offcuts {
		if o.Y > 400 && o.Height > 600 {
			foundBottom = true
			break
		}
	}
	if !foundBottom {
		t.Error("expected to find bottom strip offcut")
	}
}

func TestDetectOffcutsSmallRemnantIgnored(t *testing.T) {
	sr := SheetResult{
		Stock: StockSheet{Label: "Sheet1", Width: 500, Height: 500},
		Placements: []Placement{
			{Part: Part{Label: "P1", Width: 480, Height: 480}, X: 0, Y: 0},
		},
	}
	offcuts := DetectOffcuts(sr, 0, 3.0)
	// Remaining strips are ~17mm wide, below MinOffcutDimension
	if len(offcuts) != 0 {
		t.Errorf("expected 0 offcuts for near-full sheet, got %d", len(offcuts))
	}
}

func TestDetectAllOffcuts(t *testing.T) {
	result := OptimizeResult{
		Sheets: []SheetResult{
			{
				Stock: StockSheet{Label: "S1", Width: 2440, Height: 1220},
				Placements: []Placement{
					{Part: Part{Label: "P1", Width: 1000, Height: 600}, X: 0, Y: 0},
				},
			},
			{
				Stock: StockSheet{Label: "S2", Width: 2440, Height: 1220},
				Placements: []Placement{
					{Part: Part{Label: "P2", Width: 500, Height: 400}, X: 0, Y: 0},
				},
			},
		},
	}
	offcuts := DetectAllOffcuts(result, 3.0)
	if len(offcuts) == 0 {
		t.Error("expected at least some offcuts from two partially-used sheets")
	}
}

func TestOffcutArea(t *testing.T) {
	o := Offcut{Width: 500, Height: 300}
	if o.Area() != 150000 {
		t.Errorf("expected area 150000, got %.0f", o.Area())
	}
}

func TestOffcutToStockSheet(t *testing.T) {
	o := Offcut{
		ID:            "abc",
		SheetLabel:    "Plywood",
		Width:         800,
		Height:        400,
		PricePerSheet: 12.50,
	}
	sheet := o.ToStockSheet()
	if sheet.Width != 800 || sheet.Height != 400 {
		t.Errorf("expected 800x400, got %.0fx%.0f", sheet.Width, sheet.Height)
	}
	if sheet.PricePerSheet != 12.50 {
		t.Errorf("expected price 12.50, got %.2f", sheet.PricePerSheet)
	}
	if sheet.Quantity != 1 {
		t.Errorf("expected quantity 1, got %d", sheet.Quantity)
	}
}

func TestTotalOffcutArea(t *testing.T) {
	offcuts := []Offcut{
		{Width: 500, Height: 300},
		{Width: 200, Height: 100},
	}
	total := TotalOffcutArea(offcuts)
	expected := 500*300 + 200*100.0
	if total != expected {
		t.Errorf("expected total area %.0f, got %.0f", expected, total)
	}
}

func TestDetectOffcutsPricingProportional(t *testing.T) {
	sr := SheetResult{
		Stock: StockSheet{Label: "Sheet1", Width: 2000, Height: 1000, PricePerSheet: 100.0},
		Placements: []Placement{
			{Part: Part{Label: "P1", Width: 1000, Height: 500}, X: 0, Y: 0},
		},
	}
	offcuts := DetectOffcuts(sr, 0, 0)
	// All offcuts should have non-zero pricing
	for _, o := range offcuts {
		if o.PricePerSheet <= 0 {
			t.Errorf("expected positive pricing for offcut, got %.2f", o.PricePerSheet)
		}
	}
}
