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
	opt := New(defaultTestSettings())

	part := model.NewPart("A", 500, 300, 1)
	part.Grain = model.GrainHorizontal

	stock := model.NewStockSheet("Sheet", 1000, 600, 1)
	stock.Grain = model.GrainVertical

	result := opt.Optimize([]model.Part{part}, []model.StockSheet{stock})

	assert.Len(t, result.UnplacedParts, 1, "part should not be placed on mismatched grain stock")
}

func TestOptimize_StockGrainNoneAllowsAnyPart(t *testing.T) {
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
	opt := New(defaultTestSettings())

	part := model.NewPart("A", 800, 400, 1)
	part.Grain = model.GrainHorizontal

	stock := model.NewStockSheet("Sheet", 500, 1000, 1)
	stock.Grain = model.GrainHorizontal

	result := opt.Optimize([]model.Part{part}, []model.StockSheet{stock})

	assert.Len(t, result.UnplacedParts, 1, "grain-locked part should not fit when rotation is needed")
}

func TestOptimize_NoGrainPartOnGrainStock(t *testing.T) {
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

// ─── Multi-Material Optimization Tests ─────────────────────────────

func TestGroupByMaterial_NoMaterials(t *testing.T) {
	// When no materials are specified, everything goes into one group
	parts := []model.Part{
		model.NewPart("A", 500, 300, 1),
		model.NewPart("B", 400, 200, 1),
	}
	stocks := []model.StockSheet{
		model.NewStockSheet("Sheet1", 1000, 600, 1),
	}

	groups := groupByMaterial(parts, stocks)

	require.Len(t, groups, 1, "should be single group when no materials")
	assert.Len(t, groups[0].parts, 2)
	assert.Len(t, groups[0].stocks, 1)
}

func TestGroupByMaterial_SingleMaterial(t *testing.T) {
	partA := model.NewPart("A", 500, 300, 1)
	partA.Material = "Plywood"
	partB := model.NewPart("B", 400, 200, 1)
	partB.Material = "Plywood"

	stockPly := model.NewStockSheet("Ply", 1000, 600, 1)
	stockPly.Material = "Plywood"

	groups := groupByMaterial([]model.Part{partA, partB}, []model.StockSheet{stockPly})

	require.Len(t, groups, 1, "single material should produce one group")
	assert.Len(t, groups[0].parts, 2)
	assert.Len(t, groups[0].stocks, 1)
	assert.Equal(t, "Plywood", groups[0].material)
}

func TestGroupByMaterial_MultipleMaterials(t *testing.T) {
	partPly := model.NewPart("PlyPart", 500, 300, 1)
	partPly.Material = "Plywood"
	partMDF := model.NewPart("MDFPart", 400, 200, 1)
	partMDF.Material = "MDF"

	stockPly := model.NewStockSheet("PlySheet", 1000, 600, 1)
	stockPly.Material = "Plywood"
	stockMDF := model.NewStockSheet("MDFSheet", 1000, 600, 1)
	stockMDF.Material = "MDF"

	groups := groupByMaterial(
		[]model.Part{partPly, partMDF},
		[]model.StockSheet{stockPly, stockMDF},
	)

	require.Len(t, groups, 2, "two materials should produce two groups")

	// Groups are sorted alphabetically by material
	assert.Equal(t, "MDF", groups[0].material)
	assert.Len(t, groups[0].parts, 1)
	assert.Equal(t, "MDFPart", groups[0].parts[0].Label)

	assert.Equal(t, "Plywood", groups[1].material)
	assert.Len(t, groups[1].parts, 1)
	assert.Equal(t, "PlyPart", groups[1].parts[0].Label)
}

func TestGroupByMaterial_UniversalParts(t *testing.T) {
	// Parts with no material should be placed on any stock
	partUniversal := model.NewPart("Universal", 500, 300, 1)
	partPly := model.NewPart("PlyPart", 400, 200, 1)
	partPly.Material = "Plywood"

	stockPly := model.NewStockSheet("PlySheet", 1000, 600, 2)
	stockPly.Material = "Plywood"

	groups := groupByMaterial(
		[]model.Part{partUniversal, partPly},
		[]model.StockSheet{stockPly},
	)

	// Should have 2 groups: Plywood + universal parts
	require.Len(t, groups, 2)

	// The Plywood group should have the Plywood part
	assert.Equal(t, "Plywood", groups[0].material)
	assert.Len(t, groups[0].parts, 1)
	assert.Equal(t, "PlyPart", groups[0].parts[0].Label)

	// The universal group should have the universal part with all stocks
	assert.Len(t, groups[1].parts, 1)
	assert.Equal(t, "Universal", groups[1].parts[0].Label)
}

func TestGroupByMaterial_UniversalStocks(t *testing.T) {
	// Stocks with no material should be available to all groups
	partPly := model.NewPart("PlyPart", 500, 300, 1)
	partPly.Material = "Plywood"
	partMDF := model.NewPart("MDFPart", 400, 200, 1)
	partMDF.Material = "MDF"

	stockUniversal := model.NewStockSheet("AnySheet", 1000, 600, 5)
	// No material set on stock

	groups := groupByMaterial(
		[]model.Part{partPly, partMDF},
		[]model.StockSheet{stockUniversal},
	)

	require.Len(t, groups, 2)

	// Both groups should have the universal stock available
	for _, g := range groups {
		assert.GreaterOrEqual(t, len(g.stocks), 1,
			"each material group should have the universal stock available")
	}
}

func TestOptimize_MultiMaterial_SeparatePlacement(t *testing.T) {
	opt := New(defaultTestSettings())

	// Plywood parts
	partPly := model.NewPart("PlyPart", 500, 300, 1)
	partPly.Material = "Plywood"

	// MDF parts
	partMDF := model.NewPart("MDFPart", 400, 200, 1)
	partMDF.Material = "MDF"

	// Material-specific stocks
	stockPly := model.NewStockSheet("PlySheet", 1000, 600, 1)
	stockPly.Material = "Plywood"
	stockMDF := model.NewStockSheet("MDFSheet", 1000, 600, 1)
	stockMDF.Material = "MDF"

	result := opt.Optimize(
		[]model.Part{partPly, partMDF},
		[]model.StockSheet{stockPly, stockMDF},
	)

	require.Len(t, result.UnplacedParts, 0, "all parts should be placed")
	require.Len(t, result.Sheets, 2, "each material should use its own sheet")

	// Verify parts are on the correct material sheets
	for _, sheet := range result.Sheets {
		for _, p := range sheet.Placements {
			if p.Part.Material != "" {
				assert.Equal(t, p.Part.Material, sheet.Stock.Material,
					"part material should match stock material")
			}
		}
	}
}

func TestOptimize_MultiMaterial_WrongMaterialNotUsed(t *testing.T) {
	opt := New(defaultTestSettings())

	// Plywood part that should NOT go on MDF stock
	partPly := model.NewPart("PlyPart", 500, 300, 1)
	partPly.Material = "Plywood"

	// Only MDF stock available
	stockMDF := model.NewStockSheet("MDFSheet", 1000, 600, 1)
	stockMDF.Material = "MDF"

	result := opt.Optimize(
		[]model.Part{partPly},
		[]model.StockSheet{stockMDF},
	)

	assert.Len(t, result.UnplacedParts, 1, "plywood part should not be placed on MDF stock")
	assert.Len(t, result.Sheets, 0)
}

func TestOptimize_MultiMaterial_BackwardCompatible(t *testing.T) {
	// When no materials are set, behavior should be identical to before
	opt := New(defaultTestSettings())

	parts := []model.Part{
		model.NewPart("A", 500, 300, 1),
		model.NewPart("B", 400, 200, 1),
	}
	stocks := []model.StockSheet{
		model.NewStockSheet("Sheet", 1000, 600, 1),
	}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "all parts should be placed")
	require.Len(t, result.Sheets, 1)
	assert.Len(t, result.Sheets[0].Placements, 2)
}
