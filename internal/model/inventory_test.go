package model

import (
	"testing"
)

func TestNewStockPresetWithPrice(t *testing.T) {
	sp := NewStockPresetWithPrice("Plywood 8x4", 2440, 1220, "Plywood", 45.99)
	if sp.PricePerSheet != 45.99 {
		t.Errorf("expected price 45.99, got %.2f", sp.PricePerSheet)
	}
	if sp.Name != "Plywood 8x4" {
		t.Errorf("expected name 'Plywood 8x4', got %s", sp.Name)
	}
	if sp.Material != "Plywood" {
		t.Errorf("expected material 'Plywood', got %s", sp.Material)
	}
}

func TestToStockSheetCarriesPrice(t *testing.T) {
	sp := NewStockPresetWithPrice("MDF 8x4", 2440, 1220, "MDF", 32.50)
	sheet := sp.ToStockSheet(2)
	if sheet.PricePerSheet != 32.50 {
		t.Errorf("expected sheet price 32.50, got %.2f", sheet.PricePerSheet)
	}
	if sheet.Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", sheet.Quantity)
	}
}

func TestNewStockPresetDefaultZeroPrice(t *testing.T) {
	sp := NewStockPreset("No Price", 1000, 500, "Wood")
	if sp.PricePerSheet != 0 {
		t.Errorf("expected default price 0, got %.2f", sp.PricePerSheet)
	}
}

func TestOptimizeResultTotalCost(t *testing.T) {
	result := OptimizeResult{
		Sheets: []SheetResult{
			{
				Stock: StockSheet{Label: "Sheet A", Width: 2440, Height: 1220, PricePerSheet: 45.00},
				Placements: []Placement{
					{Part: Part{Label: "Part1", Width: 500, Height: 300}, X: 0, Y: 0},
				},
			},
			{
				Stock: StockSheet{Label: "Sheet B", Width: 2440, Height: 1220, PricePerSheet: 45.00},
				Placements: []Placement{
					{Part: Part{Label: "Part2", Width: 400, Height: 200}, X: 0, Y: 0},
				},
			},
		},
	}

	cost := result.TotalCost()
	if cost != 90.00 {
		t.Errorf("expected total cost 90.00, got %.2f", cost)
	}
}

func TestOptimizeResultHasPricing(t *testing.T) {
	withPrice := OptimizeResult{
		Sheets: []SheetResult{
			{Stock: StockSheet{PricePerSheet: 10.0}},
		},
	}
	if !withPrice.HasPricing() {
		t.Error("expected HasPricing() to return true when sheets have pricing")
	}

	withoutPrice := OptimizeResult{
		Sheets: []SheetResult{
			{Stock: StockSheet{PricePerSheet: 0}},
		},
	}
	if withoutPrice.HasPricing() {
		t.Error("expected HasPricing() to return false when no sheets have pricing")
	}

	empty := OptimizeResult{}
	if empty.HasPricing() {
		t.Error("expected HasPricing() to return false for empty result")
	}
}

func TestOptimizeResultTotalCostZeroWhenNoPricing(t *testing.T) {
	result := OptimizeResult{
		Sheets: []SheetResult{
			{Stock: StockSheet{Label: "No Price", Width: 1000, Height: 500}},
		},
	}
	if result.TotalCost() != 0 {
		t.Errorf("expected 0 cost when no pricing, got %.2f", result.TotalCost())
	}
}
