package engine

import (
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultTestSettings() model.CutSettings {
	s := model.DefaultSettings()
	// Simplify for testing: no edge trim, no kerf, no tabs
	s.EdgeTrim = 0
	s.KerfWidth = 0
	s.StockTabs.Enabled = false
	return s
}

func TestOptimize_SingleStockSinglePart(t *testing.T) {
	opt := New(defaultTestSettings())
	parts := []model.Part{model.NewPart("A", 500, 300, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 1000, 600, 1)}

	result := opt.Optimize(parts, stocks)

	assert.Len(t, result.Sheets, 1)
	assert.Len(t, result.UnplacedParts, 0)
	assert.Len(t, result.Sheets[0].Placements, 1)
	assert.Equal(t, "A", result.Sheets[0].Placements[0].Part.Label)
}

func TestOptimize_MultipleStockSizes_SelectsSmallestAdequate(t *testing.T) {
	// When parts fit on a small sheet, the optimizer should prefer the smaller
	// stock over the larger one to minimize waste.
	opt := New(defaultTestSettings())

	parts := []model.Part{
		model.NewPart("Small1", 400, 200, 1),
		model.NewPart("Small2", 300, 200, 1),
	}

	stocks := []model.StockSheet{
		model.NewStockSheet("Large", 2440, 1220, 2),
		model.NewStockSheet("Small", 1220, 610, 2),
	}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "all parts should be placed")
	require.GreaterOrEqual(t, len(result.Sheets), 1)

	// The optimizer should use the small sheet because both parts fit on it
	// and it yields better efficiency than wasting a large sheet.
	firstSheet := result.Sheets[0]
	assert.Equal(t, 1220.0, firstSheet.Stock.Width, "should use the small sheet")
	assert.Equal(t, 610.0, firstSheet.Stock.Height, "should use the small sheet")
}

func TestOptimize_MultipleStockSizes_LargePartForcesLargeSheet(t *testing.T) {
	// When a part is too big for the small sheet, the optimizer must use the large one.
	opt := New(defaultTestSettings())

	parts := []model.Part{
		model.NewPart("Big", 2000, 1000, 1),
	}

	stocks := []model.StockSheet{
		model.NewStockSheet("Small", 1220, 610, 1),
		model.NewStockSheet("Large", 2440, 1220, 1),
	}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0)
	require.Len(t, result.Sheets, 1)
	assert.Equal(t, 2440.0, result.Sheets[0].Stock.Width, "large part should go on large sheet")
}

func TestOptimize_MultipleStockSizes_MixedUsage(t *testing.T) {
	// A mix of large and small parts should use different stock sizes.
	opt := New(defaultTestSettings())

	parts := []model.Part{
		model.NewPart("Large1", 2000, 1000, 1),
		model.NewPart("Small1", 400, 300, 1),
		model.NewPart("Small2", 500, 250, 1),
	}

	stocks := []model.StockSheet{
		model.NewStockSheet("Large", 2440, 1220, 2),
		model.NewStockSheet("Small", 1220, 610, 2),
	}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "all parts should be placed")
	require.GreaterOrEqual(t, len(result.Sheets), 1)

	// The large part must be on a large sheet
	foundLargeOnLarge := false
	for _, sheet := range result.Sheets {
		for _, p := range sheet.Placements {
			if p.Part.Label == "Large1" {
				assert.Equal(t, 2440.0, sheet.Stock.Width)
				foundLargeOnLarge = true
			}
		}
	}
	assert.True(t, foundLargeOnLarge, "large part should be placed on large sheet")
}

func TestOptimize_AllPartsUnplaceable(t *testing.T) {
	// Parts that don't fit on any stock should end up in UnplacedParts.
	opt := New(defaultTestSettings())

	parts := []model.Part{
		model.NewPart("Huge", 5000, 3000, 1),
	}
	stocks := []model.StockSheet{
		model.NewStockSheet("Small", 1000, 500, 1),
	}

	result := opt.Optimize(parts, stocks)

	assert.Len(t, result.Sheets, 0, "no sheets should be used")
	assert.Len(t, result.UnplacedParts, 1)
}

func TestOptimize_EmptyInputs(t *testing.T) {
	opt := New(defaultTestSettings())

	// No parts
	result := opt.Optimize(nil, []model.StockSheet{model.NewStockSheet("S", 1000, 500, 1)})
	assert.Len(t, result.Sheets, 0)
	assert.Len(t, result.UnplacedParts, 0)

	// No stocks
	result = opt.Optimize([]model.Part{model.NewPart("A", 100, 100, 1)}, nil)
	assert.Len(t, result.Sheets, 0)
	assert.Len(t, result.UnplacedParts, 1)
}

func TestOptimize_QuantityExpansion(t *testing.T) {
	// Verify that part quantity is correctly expanded.
	opt := New(defaultTestSettings())

	parts := []model.Part{model.NewPart("A", 500, 300, 3)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 2440, 1220, 2)}

	result := opt.Optimize(parts, stocks)

	totalPlaced := 0
	for _, sheet := range result.Sheets {
		totalPlaced += len(sheet.Placements)
	}
	assert.Equal(t, 3, totalPlaced, "all 3 copies should be placed")
	assert.Len(t, result.UnplacedParts, 0)
}

func TestOptimize_WithKerfAndEdgeTrim(t *testing.T) {
	settings := defaultTestSettings()
	settings.KerfWidth = 3.0
	settings.EdgeTrim = 10.0

	opt := New(settings)

	// Part barely fits with edge trim and kerf accounted for
	// Usable: 1000 - 2*10 = 980 wide, 500 - 2*10 = 480 tall
	// With kerf: part needs 978+3=981 > 980, so 978 should NOT fit
	// Let's use a part that does fit: 970 + 3 = 973 <= 980
	parts := []model.Part{model.NewPart("Tight", 970, 470, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 1000, 500, 1)}

	result := opt.Optimize(parts, stocks)

	assert.Len(t, result.UnplacedParts, 0, "part should fit with kerf and edge trim")
	assert.Len(t, result.Sheets, 1)
}

func TestOptimize_Rotation(t *testing.T) {
	opt := New(defaultTestSettings())

	// Part is 800x400, stock is 500x1000. Part won't fit normally but fits rotated.
	parts := []model.Part{model.NewPart("Rotatable", 800, 400, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 500, 1000, 1)}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "part should fit when rotated")
	require.Len(t, result.Sheets, 1)
	assert.True(t, result.Sheets[0].Placements[0].Rotated, "part should be rotated")
}

func TestOptimize_GrainPreventsRotation(t *testing.T) {
	opt := New(defaultTestSettings())

	// Part is 800x400 with horizontal grain, stock is 500x1000.
	// Part won't fit normally and can't rotate due to grain.
	part := model.NewPart("GrainLocked", 800, 400, 1)
	part.Grain = model.GrainHorizontal
	parts := []model.Part{part}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 500, 1000, 1)}

	result := opt.Optimize(parts, stocks)

	assert.Len(t, result.UnplacedParts, 1, "grain-locked part should not fit")
}

func TestSelectBestStock_TrialPackingPreference(t *testing.T) {
	// Verify that selectBestStock uses trial packing, not just area comparison.
	// With 2 small parts that perfectly fill a small sheet, the optimizer should
	// prefer the small sheet even though a large sheet is also available.
	opt := New(defaultTestSettings())

	stocks := []model.StockSheet{
		model.NewStockSheet("Large", 2440, 1220, 1),
		model.NewStockSheet("Small", 600, 400, 1),
	}

	parts := []model.Part{
		model.NewPart("A", 300, 400, 1),
		model.NewPart("B", 300, 400, 1),
	}

	idx := opt.selectBestStock(stocks, parts)
	require.GreaterOrEqual(t, idx, 0)
	// Both parts fit on the 600x400 small sheet perfectly (300+300=600),
	// so the small sheet should be preferred for better efficiency.
	assert.Equal(t, 600.0, stocks[idx].Width, "trial packing should prefer the small sheet")
}

func TestSelectBestStock_NoCandidates(t *testing.T) {
	opt := New(defaultTestSettings())

	stocks := []model.StockSheet{
		model.NewStockSheet("Tiny", 100, 100, 1),
	}
	parts := []model.Part{
		model.NewPart("Big", 500, 500, 1),
	}

	idx := opt.selectBestStock(stocks, parts)
	assert.Equal(t, -1, idx, "should return -1 when no stock can fit the largest part")
}

func TestSelectBestStock_EmptyInputs(t *testing.T) {
	opt := New(defaultTestSettings())

	assert.Equal(t, -1, opt.selectBestStock(nil, nil))
	assert.Equal(t, -1, opt.selectBestStock([]model.StockSheet{}, nil))
	assert.Equal(t, -1, opt.selectBestStock(nil, []model.Part{}))
}

func TestOptimize_MultipleStockSizes_StockPoolDepletion(t *testing.T) {
	// Verify that when stock is depleted, the optimizer moves to the next size.
	opt := New(defaultTestSettings())

	parts := []model.Part{
		model.NewPart("A", 500, 500, 3), // 3 parts, each 500x500
	}

	stocks := []model.StockSheet{
		model.NewStockSheet("Small", 600, 600, 1),   // Only 1 small sheet (fits 1 part)
		model.NewStockSheet("Large", 1200, 1200, 1), // 1 large sheet (fits remaining)
	}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "all parts should be placed")
	assert.GreaterOrEqual(t, len(result.Sheets), 2, "should use at least 2 sheets")
}

func TestOptimize_Efficiency(t *testing.T) {
	opt := New(defaultTestSettings())

	// One part that's exactly half the sheet
	parts := []model.Part{model.NewPart("Half", 1000, 500, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 1000, 1000, 1)}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.Sheets, 1)
	assert.InDelta(t, 50.0, result.TotalEfficiency(), 0.1, "efficiency should be ~50%%")
}

// ─── Sheet Grain Matching Tests ────────────────────────────────────

func TestCanPlaceWithGrain(t *testing.T) {
	tests := []struct {
		name       string
		partGrain  model.Grain
		stockGrain model.Grain
		wantNormal bool
		wantRotate bool
	}{
		{"None/None", model.GrainNone, model.GrainNone, true, true},
		{"None/Horizontal", model.GrainNone, model.GrainHorizontal, true, true},
		{"None/Vertical", model.GrainNone, model.GrainVertical, true, true},
		{"Horizontal/None", model.GrainHorizontal, model.GrainNone, true, false},
		{"Vertical/None", model.GrainVertical, model.GrainNone, true, false},
		{"Horizontal/Horizontal", model.GrainHorizontal, model.GrainHorizontal, true, false},
		{"Vertical/Vertical", model.GrainVertical, model.GrainVertical, true, false},
		{"Horizontal/Vertical", model.GrainHorizontal, model.GrainVertical, false, false},
		{"Vertical/Horizontal", model.GrainVertical, model.GrainHorizontal, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			canNormal, canRotated := model.CanPlaceWithGrain(tc.partGrain, tc.stockGrain)
			assert.Equal(t, tc.wantNormal, canNormal, "canNormal mismatch")
			assert.Equal(t, tc.wantRotate, canRotated, "canRotated mismatch")
		})
	}
}

func TestOptimize_StockGrainMatchesPart(t *testing.T) {
	// Part with horizontal grain on horizontal stock should be placed normally.
	opt := New(defaultTestSettings())

	part := model.NewPart("A", 500, 300, 1)
	part.Grain = model.GrainHorizontal

	stock := model.NewStockSheet("Sheet", 1000, 600, 1)
	stock.Grain = model.GrainHorizontal

	result := opt.Optimize([]model.Part{part}, []model.StockSheet{stock})

	require.Len(t, result.UnplacedParts, 0, "part should be placed on matching grain stock")
	require.Len(t, result.Sheets, 1)
	assert.False(t, result.Sheets[0].Placements[0].Rotated, "should not be rotated")
}

func TestOptimize_StockGrainMismatchBlocksPlacement(t *testing.T) {
	// Part with horizontal grain on vertical-grain stock should not be placed.
	opt := New(defaultTestSettings())

	part := model.NewPart("A", 500, 300, 1)
	part.Grain = model.GrainHorizontal

	stock := model.NewStockSheet("Sheet", 1000, 600, 1)
	stock.Grain = model.GrainVertical

	result := opt.Optimize([]model.Part{part}, []model.StockSheet{stock})

	assert.Len(t, result.UnplacedParts, 1, "part should not be placed on mismatched grain stock")
}

func TestOptimize_StockGrainNoneAllowsAnyPart(t *testing.T) {
	// Stock with no grain should accept parts with any grain direction.
	opt := New(defaultTestSettings())

	partH := model.NewPart("H", 500, 300, 1)
	partH.Grain = model.GrainHorizontal
	partV := model.NewPart("V", 400, 200, 1)
	partV.Grain = model.GrainVertical

	stock := model.NewStockSheet("Sheet", 2000, 1000, 1)
	stock.Grain = model.GrainNone

	result := opt.Optimize([]model.Part{partH, partV}, []model.StockSheet{stock})

	assert.Len(t, result.UnplacedParts, 0, "all parts should be placed on grain-none stock")
}

func TestOptimize_StockGrainPreventsRotation(t *testing.T) {
	// Part is 800x400, stock is 500x1000 with horizontal grain.
	// Part has horizontal grain. Normally it would need rotation to fit,
	// but grain prevents it.
	opt := New(defaultTestSettings())

	part := model.NewPart("A", 800, 400, 1)
	part.Grain = model.GrainHorizontal

	stock := model.NewStockSheet("Sheet", 500, 1000, 1)
	stock.Grain = model.GrainHorizontal

	result := opt.Optimize([]model.Part{part}, []model.StockSheet{stock})

	assert.Len(t, result.UnplacedParts, 1, "grain-locked part should not fit when rotation is needed")
}

func TestOptimize_NoGrainPartOnGrainStock(t *testing.T) {
	// Part with no grain on a grain stock: should allow rotation freely.
	opt := New(defaultTestSettings())

	part := model.NewPart("A", 800, 400, 1)
	part.Grain = model.GrainNone

	stock := model.NewStockSheet("Sheet", 500, 1000, 1)
	stock.Grain = model.GrainHorizontal

	result := opt.Optimize([]model.Part{part}, []model.StockSheet{stock})

	require.Len(t, result.UnplacedParts, 0, "no-grain part should fit rotated on grain stock")
	require.Len(t, result.Sheets, 1)
	assert.True(t, result.Sheets[0].Placements[0].Rotated, "should be rotated to fit")
}
