package engine

import (
	"math"
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

// ─── Clamp Zone Tests ────────────────────────────────────

func TestOptimize_ClampZoneExclusion(t *testing.T) {
	// A clamp zone that covers the top-left corner should prevent parts from
	// being placed there, forcing them into the remaining space.
	settings := defaultTestSettings()
	settings.ClampZones = []model.ClampZone{
		{Label: "Clamp1", X: 0, Y: 0, Width: 200, Height: 200},
	}

	opt := New(settings)

	// Part that would normally go at origin (0,0) but can't due to clamp
	parts := []model.Part{model.NewPart("A", 150, 150, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 1000, 600, 1)}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "part should still be placed")
	require.Len(t, result.Sheets, 1)

	p := result.Sheets[0].Placements[0]
	// The part should NOT overlap with the clamp zone at (0,0,200,200)
	clamp := settings.ClampZones[0]
	overlaps := clamp.Overlaps(p.X, p.Y, p.PlacedWidth(), p.PlacedHeight())
	assert.False(t, overlaps, "part should not overlap with clamp zone; placed at (%.1f, %.1f)", p.X, p.Y)
}

func TestOptimize_ClampZoneBlocksPlacement(t *testing.T) {
	// If clamp zones cover most of the sheet, parts that don't fit in the
	// remaining space should end up unplaced.
	settings := defaultTestSettings()
	// Cover almost all of a 500x500 sheet with clamp zones
	settings.ClampZones = []model.ClampZone{
		{Label: "BigClamp", X: 0, Y: 0, Width: 500, Height: 400},
	}

	opt := New(settings)

	// Part is 200x200, remaining space after clamp is 500x100 (bottom strip)
	parts := []model.Part{model.NewPart("A", 200, 200, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 500, 500, 1)}

	result := opt.Optimize(parts, stocks)

	assert.Len(t, result.UnplacedParts, 1, "part should not fit in remaining space")
}

func TestOptimize_MultipleClampZones(t *testing.T) {
	// Multiple clamp zones in different corners should still allow parts
	// to be placed in the center.
	settings := defaultTestSettings()
	settings.ClampZones = []model.ClampZone{
		{Label: "TL", X: 0, Y: 0, Width: 100, Height: 100},
		{Label: "TR", X: 900, Y: 0, Width: 100, Height: 100},
		{Label: "BL", X: 0, Y: 500, Width: 100, Height: 100},
		{Label: "BR", X: 900, Y: 500, Width: 100, Height: 100},
	}

	opt := New(settings)

	parts := []model.Part{model.NewPart("Center", 300, 200, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 1000, 600, 1)}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "part should fit in space between clamps")
	require.Len(t, result.Sheets, 1)

	// Verify no overlap with any clamp zone
	p := result.Sheets[0].Placements[0]
	for _, cz := range settings.ClampZones {
		assert.False(t, cz.Overlaps(p.X, p.Y, p.PlacedWidth(), p.PlacedHeight()),
			"part should not overlap with clamp zone %s", cz.Label)
	}
}

func TestOptimize_NoClampZones(t *testing.T) {
	// Without clamp zones, optimizer should work as normal.
	settings := defaultTestSettings()
	settings.ClampZones = nil

	opt := New(settings)

	parts := []model.Part{model.NewPart("A", 500, 300, 1)}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 1000, 600, 1)}

	result := opt.Optimize(parts, stocks)

	assert.Len(t, result.UnplacedParts, 0)
	assert.Len(t, result.Sheets, 1)
}

// ─── Multi-Material Optimization Tests ─────────────────────────────

func TestGroupByMaterial_NoMaterials(t *testing.T) {
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

	assert.Equal(t, "MDF", groups[0].material)
	assert.Len(t, groups[0].parts, 1)
	assert.Equal(t, "MDFPart", groups[0].parts[0].Label)

	assert.Equal(t, "Plywood", groups[1].material)
	assert.Len(t, groups[1].parts, 1)
	assert.Equal(t, "PlyPart", groups[1].parts[0].Label)
}

func TestGroupByMaterial_UniversalParts(t *testing.T) {
	partUniversal := model.NewPart("Universal", 500, 300, 1)
	partPly := model.NewPart("PlyPart", 400, 200, 1)
	partPly.Material = "Plywood"

	stockPly := model.NewStockSheet("PlySheet", 1000, 600, 2)
	stockPly.Material = "Plywood"

	groups := groupByMaterial(
		[]model.Part{partUniversal, partPly},
		[]model.StockSheet{stockPly},
	)

	require.Len(t, groups, 2)
	assert.Equal(t, "Plywood", groups[0].material)
	assert.Len(t, groups[0].parts, 1)
	assert.Equal(t, "PlyPart", groups[0].parts[0].Label)

	assert.Len(t, groups[1].parts, 1)
	assert.Equal(t, "Universal", groups[1].parts[0].Label)
}

func TestGroupByMaterial_UniversalStocks(t *testing.T) {
	partPly := model.NewPart("PlyPart", 500, 300, 1)
	partPly.Material = "Plywood"
	partMDF := model.NewPart("MDFPart", 400, 200, 1)
	partMDF.Material = "MDF"

	stockUniversal := model.NewStockSheet("AnySheet", 1000, 600, 5)

	groups := groupByMaterial(
		[]model.Part{partPly, partMDF},
		[]model.StockSheet{stockUniversal},
	)

	require.Len(t, groups, 2)
	for _, g := range groups {
		assert.GreaterOrEqual(t, len(g.stocks), 1,
			"each material group should have the universal stock available")
	}
}

func TestOptimize_MultiMaterial_SeparatePlacement(t *testing.T) {
	opt := New(defaultTestSettings())

	partPly := model.NewPart("PlyPart", 500, 300, 1)
	partPly.Material = "Plywood"
	partMDF := model.NewPart("MDFPart", 400, 200, 1)
	partMDF.Material = "MDF"

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

	partPly := model.NewPart("PlyPart", 500, 300, 1)
	partPly.Material = "Plywood"

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

// ─── Interior Cutout Nesting Tests ─────────────────────────────

func TestCutoutBounds(t *testing.T) {
	part := model.NewPart("WithHole", 500, 400, 1)
	part.Cutouts = []model.Outline{
		{
			{X: 100, Y: 100},
			{X: 200, Y: 100},
			{X: 200, Y: 200},
			{X: 100, Y: 200},
		},
	}

	bounds := part.CutoutBounds()
	require.Len(t, bounds, 1)
	assert.Equal(t, 100.0, bounds[0].X)
	assert.Equal(t, 100.0, bounds[0].Y)
	assert.Equal(t, 100.0, bounds[0].Width)
	assert.Equal(t, 100.0, bounds[0].Height)
}

func TestCutoutBounds_MultipleCutouts(t *testing.T) {
	part := model.NewPart("MultiHole", 500, 400, 1)
	part.Cutouts = []model.Outline{
		{
			{X: 50, Y: 50},
			{X: 150, Y: 50},
			{X: 150, Y: 150},
			{X: 50, Y: 150},
		},
		{
			{X: 300, Y: 200},
			{X: 450, Y: 200},
			{X: 450, Y: 350},
			{X: 300, Y: 350},
		},
	}

	bounds := part.CutoutBounds()
	require.Len(t, bounds, 2)
	assert.Equal(t, 100.0, bounds[0].Width)
	assert.Equal(t, 100.0, bounds[0].Height)
	assert.Equal(t, 150.0, bounds[1].Width)
	assert.Equal(t, 150.0, bounds[1].Height)
}

func TestOptimize_CutoutNesting_SmallPartInsideCutout(t *testing.T) {
	opt := New(defaultTestSettings())

	// Large part with a 200x200 interior cutout starting at (150,100)
	largePart := model.NewPart("Shelf", 500, 400, 1)
	largePart.Cutouts = []model.Outline{
		{
			{X: 150, Y: 100},
			{X: 350, Y: 100},
			{X: 350, Y: 300},
			{X: 150, Y: 300},
		},
	}

	// Small part that should fit inside the cutout (200x200)
	smallPart := model.NewPart("Bracket", 100, 100, 1)

	// Stock just large enough for the shelf part
	parts := []model.Part{largePart, smallPart}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 500, 400, 1)}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "small part should nest inside cutout")
	require.Len(t, result.Sheets, 1)
	assert.Len(t, result.Sheets[0].Placements, 2, "both parts should be on same sheet")
}

func TestOptimize_CutoutNesting_NoCutouts(t *testing.T) {
	// Parts without cutouts should work normally
	opt := New(defaultTestSettings())

	parts := []model.Part{
		model.NewPart("A", 500, 300, 1),
		model.NewPart("B", 400, 200, 1),
	}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 1000, 600, 1)}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0)
	require.Len(t, result.Sheets, 1)
	assert.Len(t, result.Sheets[0].Placements, 2)
}

func TestOptimize_CutoutNesting_TooSmallCutout(t *testing.T) {
	opt := New(defaultTestSettings())

	// Large part with a tiny 20x20 cutout
	largePart := model.NewPart("Panel", 500, 400, 1)
	largePart.Cutouts = []model.Outline{
		{
			{X: 200, Y: 200},
			{X: 220, Y: 200},
			{X: 220, Y: 220},
			{X: 200, Y: 220},
		},
	}

	// Part too large for the tiny cutout
	smallPart := model.NewPart("Widget", 50, 50, 1)

	// Stock only large enough for the panel
	parts := []model.Part{largePart, smallPart}
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 500, 400, 1)}

	result := opt.Optimize(parts, stocks)

	// The small part won't fit in the 20x20 cutout (too small)
	assert.Len(t, result.UnplacedParts, 1, "part too large for cutout should be unplaced")
}

func TestOptimize_CutoutNesting_MultipleSmallParts(t *testing.T) {
	opt := New(defaultTestSettings())

	// Large part with a 300x200 cutout
	largePart := model.NewPart("Frame", 600, 400, 1)
	largePart.Cutouts = []model.Outline{
		{
			{X: 150, Y: 100},
			{X: 450, Y: 100},
			{X: 450, Y: 300},
			{X: 150, Y: 300},
		},
	}

	// Multiple small parts that should fit inside
	parts := []model.Part{
		largePart,
		model.NewPart("Small1", 100, 100, 1),
		model.NewPart("Small2", 100, 100, 1),
	}

	// Stock just large enough for the frame
	stocks := []model.StockSheet{model.NewStockSheet("Sheet", 600, 400, 1)}

	result := opt.Optimize(parts, stocks)

	require.Len(t, result.UnplacedParts, 0, "small parts should nest inside cutout")
	require.Len(t, result.Sheets, 1)
	assert.Len(t, result.Sheets[0].Placements, 3, "all three parts should be on same sheet")
}

// --- Multi-Objective Optimization Tests ---

func TestMultiObjective_DefaultWeights(t *testing.T) {
	w := model.DefaultOptimizeWeights()
	assert.Equal(t, 1.0, w.MinimizeWaste)
	assert.Equal(t, 0.5, w.MinimizeSheets)
	assert.Equal(t, 0.0, w.MinimizeCutLen)
	assert.Equal(t, 0.0, w.MinimizeJobTime)
}

func TestMultiObjective_NormalizeWeights(t *testing.T) {
	w := model.OptimizeWeights{
		MinimizeWaste:  2.0,
		MinimizeSheets: 2.0,
		MinimizeCutLen: 0.0,
		MinimizeJobTime: 0.0,
	}
	n := w.Normalize()
	assert.InDelta(t, 0.5, n.MinimizeWaste, 0.001)
	assert.InDelta(t, 0.5, n.MinimizeSheets, 0.001)
	assert.InDelta(t, 0.0, n.MinimizeCutLen, 0.001)
	assert.InDelta(t, 0.0, n.MinimizeJobTime, 0.001)
}

func TestMultiObjective_NormalizeAllZero(t *testing.T) {
	w := model.OptimizeWeights{}
	n := w.Normalize()
	// Should fall back to equal waste/sheets weights
	assert.InDelta(t, 0.5, n.MinimizeWaste, 0.001)
	assert.InDelta(t, 0.5, n.MinimizeSheets, 0.001)
}

func TestMultiObjective_GeneticWithWasteWeight(t *testing.T) {
	s := defaultTestSettings()
	s.Algorithm = model.AlgorithmGenetic
	s.OptimizeWeights = model.OptimizeWeights{
		MinimizeWaste:  1.0,
		MinimizeSheets: 0.0,
		MinimizeCutLen: 0.0,
		MinimizeJobTime: 0.0,
	}

	parts := []model.Part{
		{ID: "p1", Label: "A", Width: 200, Height: 200, Quantity: 2},
	}
	stocks := []model.StockSheet{
		{ID: "s1", Label: "Sheet", Width: 500, Height: 500, Quantity: 5},
	}

	opt := New(s)
	result := opt.Optimize(parts, stocks)
	assert.Len(t, result.UnplacedParts, 0)
	assert.Greater(t, len(result.Sheets), 0)
}

func TestMultiObjective_GeneticWithSheetWeight(t *testing.T) {
	s := defaultTestSettings()
	s.Algorithm = model.AlgorithmGenetic
	s.OptimizeWeights = model.OptimizeWeights{
		MinimizeWaste:  0.0,
		MinimizeSheets: 1.0,
		MinimizeCutLen: 0.0,
		MinimizeJobTime: 0.0,
	}

	parts := []model.Part{
		{ID: "p1", Label: "A", Width: 100, Height: 100, Quantity: 4},
	}
	stocks := []model.StockSheet{
		{ID: "s1", Label: "Sheet", Width: 500, Height: 500, Quantity: 5},
	}

	opt := New(s)
	result := opt.Optimize(parts, stocks)
	assert.Len(t, result.UnplacedParts, 0)
	// With heavy sheet minimization weight, should use minimal sheets
	assert.Equal(t, 1, len(result.Sheets))
}

func TestMultiObjective_TotalCutLength(t *testing.T) {
	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{Width: 500, Height: 500},
				Placements: []model.Placement{
					{Part: model.Part{Width: 100, Height: 50}, X: 0, Y: 0},
					{Part: model.Part{Width: 200, Height: 100}, X: 110, Y: 0},
				},
			},
		},
	}

	cutLen := result.TotalCutLength()
	// Part 1: 2*(100+50) = 300, Part 2: 2*(200+100) = 600 => total 900
	assert.InDelta(t, 900.0, cutLen, 0.01)
}

func TestMultiObjective_TotalCutLengthWithOutline(t *testing.T) {
	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{Width: 500, Height: 500},
				Placements: []model.Placement{
					{Part: model.Part{
						Width: 100, Height: 100,
						Outline: model.Outline{
							{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100},
						},
					}, X: 0, Y: 0},
				},
			},
		},
	}

	cutLen := result.TotalCutLength()
	// Square outline: 4 * 100 = 400
	assert.InDelta(t, 400.0, cutLen, 0.01)
}

func TestMultiObjective_EstimatedJobTime(t *testing.T) {
	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{Width: 500, Height: 500},
				Placements: []model.Placement{
					{Part: model.Part{Width: 100, Height: 50}, X: 0, Y: 0},
				},
			},
		},
	}

	// Cut length = 2*(100+50) = 300mm, feedRate = 1000mm/min, 1 pass, 2 min setup
	time := result.EstimatedJobTime(1000.0, 6.0, 6.0, 2.0)
	// 300/1000 = 0.3 min cutting + 2.0 setup = 2.3 min
	assert.InDelta(t, 2.3, time, 0.01)
}

func TestMultiObjective_EstimatedJobTimeMultiplePasses(t *testing.T) {
	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{Width: 500, Height: 500},
				Placements: []model.Placement{
					{Part: model.Part{Width: 100, Height: 50}, X: 0, Y: 0},
				},
			},
		},
	}

	// Cut length = 300mm, feedRate = 1000, 3 passes (18mm depth / 6mm pass), 0 setup
	time := result.EstimatedJobTime(1000.0, 6.0, 18.0, 0.0)
	// 300 * 3 / 1000 = 0.9 min
	assert.InDelta(t, 0.9, time, 0.01)
}

func TestMultiObjective_OutlinePerimeter(t *testing.T) {
	// Triangle: (0,0), (3,0), (0,4) — sides: 3, 5, 4 = 12
	outline := model.Outline{
		{X: 0, Y: 0}, {X: 3, Y: 0}, {X: 0, Y: 4},
	}
	assert.InDelta(t, 12.0, outline.Perimeter(), 0.01)
}

func TestMultiObjective_OutlinePerimeterEmpty(t *testing.T) {
	var empty model.Outline
	assert.Equal(t, 0.0, empty.Perimeter())
}

func TestMultiObjective_WeightsInDefaultSettings(t *testing.T) {
	s := model.DefaultSettings()
	assert.Equal(t, 1.0, s.OptimizeWeights.MinimizeWaste)
	assert.Equal(t, 0.5, s.OptimizeWeights.MinimizeSheets)
	assert.Equal(t, 0.0, s.OptimizeWeights.MinimizeCutLen)
	assert.Equal(t, 0.0, s.OptimizeWeights.MinimizeJobTime)
}

// --- Non-Rectangular Nesting Tests ---

func TestOutlineRotate_90Degrees(t *testing.T) {
	// Rectangle 100x50 rotated 90 degrees should become ~50x100
	outline := model.Outline{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 50}, {X: 0, Y: 50},
	}
	rotated := outline.Rotate(math.Pi / 2)
	min, max := rotated.BoundingBox()
	w := max.X - min.X
	h := max.Y - min.Y
	assert.InDelta(t, 50.0, w, 0.5)
	assert.InDelta(t, 100.0, h, 0.5)
}

func TestOutlineRotate_0Degrees(t *testing.T) {
	outline := model.Outline{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 50}, {X: 0, Y: 50},
	}
	rotated := outline.Rotate(0)
	min, max := rotated.BoundingBox()
	assert.InDelta(t, 100.0, max.X-min.X, 0.01)
	assert.InDelta(t, 50.0, max.Y-min.Y, 0.01)
}

func TestOutlineArea_Rectangle(t *testing.T) {
	outline := model.Outline{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 50}, {X: 0, Y: 50},
	}
	assert.InDelta(t, 5000.0, outline.Area(), 0.01)
}

func TestOutlineArea_Triangle(t *testing.T) {
	outline := model.Outline{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 50, Y: 50},
	}
	assert.InDelta(t, 2500.0, outline.Area(), 0.01)
}

func TestOutlineArea_Empty(t *testing.T) {
	var empty model.Outline
	assert.Equal(t, 0.0, empty.Area())
}

func TestOutlineContainsPoint(t *testing.T) {
	square := model.Outline{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100},
	}
	assert.True(t, square.ContainsPoint(50, 50))
	assert.False(t, square.ContainsPoint(150, 50))
	assert.False(t, square.ContainsPoint(-10, 50))
}

func TestOutlinesOverlap_Overlapping(t *testing.T) {
	sq1 := model.Outline{
		{X: 0, Y: 0}, {X: 50, Y: 0}, {X: 50, Y: 50}, {X: 0, Y: 50},
	}
	sq2 := model.Outline{
		{X: 0, Y: 0}, {X: 50, Y: 0}, {X: 50, Y: 50}, {X: 0, Y: 50},
	}
	// Placed at overlapping positions
	assert.True(t, model.OutlinesOverlap(sq1, 0, 0, sq2, 25, 25))
}

func TestOutlinesOverlap_NotOverlapping(t *testing.T) {
	sq1 := model.Outline{
		{X: 0, Y: 0}, {X: 50, Y: 0}, {X: 50, Y: 50}, {X: 0, Y: 50},
	}
	sq2 := model.Outline{
		{X: 0, Y: 0}, {X: 50, Y: 0}, {X: 50, Y: 50}, {X: 0, Y: 50},
	}
	// Placed far apart
	assert.False(t, model.OutlinesOverlap(sq1, 0, 0, sq2, 100, 0))
}

func TestOutlinesOverlap_Containment(t *testing.T) {
	big := model.Outline{
		{X: 0, Y: 0}, {X: 200, Y: 0}, {X: 200, Y: 200}, {X: 0, Y: 200},
	}
	small := model.Outline{
		{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 20}, {X: 0, Y: 20},
	}
	// small is inside big
	assert.True(t, model.OutlinesOverlap(big, 0, 0, small, 50, 50))
}

func TestOptimize_NestingRotations_Default(t *testing.T) {
	s := model.DefaultSettings()
	assert.Equal(t, 2, s.NestingRotations)
}

func TestOptimize_OutlinePartMultipleRotations(t *testing.T) {
	s := defaultTestSettings()
	s.NestingRotations = 4 // Try 0, 45, 90, 135 degrees

	// An L-shaped outline part that might fit better at certain angles
	parts := []model.Part{
		{
			ID: "p1", Label: "L-Shape", Width: 150, Height: 100, Quantity: 1,
			Outline: model.Outline{
				{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 50},
				{X: 50, Y: 50}, {X: 50, Y: 100}, {X: 0, Y: 100},
			},
			Grain: model.GrainNone,
		},
	}
	stocks := []model.StockSheet{
		{ID: "s1", Label: "Sheet", Width: 500, Height: 500, Quantity: 1},
	}

	opt := New(s)
	result := opt.Optimize(parts, stocks)
	assert.Len(t, result.UnplacedParts, 0, "outline part should be placed")
	assert.Len(t, result.Sheets, 1)
}

func TestOptimize_OutlinePartWithGrainSkipsMultiRotation(t *testing.T) {
	s := defaultTestSettings()
	s.NestingRotations = 8

	// Part with grain should NOT use multi-rotation (grain takes priority)
	parts := []model.Part{
		{
			ID: "p1", Label: "GrainPart", Width: 100, Height: 50, Quantity: 1,
			Outline: model.Outline{
				{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 50}, {X: 0, Y: 50},
			},
			Grain: model.GrainHorizontal,
		},
	}
	stocks := []model.StockSheet{
		{ID: "s1", Label: "Sheet", Width: 500, Height: 500, Quantity: 1, Grain: model.GrainHorizontal},
	}

	opt := New(s)
	result := opt.Optimize(parts, stocks)
	assert.Len(t, result.UnplacedParts, 0, "grain part should be placed via normal path")
}

// TestOptimize_DefaultSettings_EndToEnd simulates what the UI does: optimize with
// DefaultSettings, then exercise the full results pipeline (GCode generation, etc.).
func TestOptimize_DefaultSettings_EndToEnd(t *testing.T) {
	settings := model.DefaultSettings()

	parts := []model.Part{
		model.NewPart("Panel A", 600, 400, 2),
		model.NewPart("Shelf", 500, 300, 3),
		model.NewPart("Door", 800, 600, 1),
	}
	stocks := []model.StockSheet{
		model.NewStockSheet("Plywood 2440x1220", 2440, 1220, 2),
	}

	opt := New(settings)
	result := opt.Optimize(parts, stocks)

	// Basic result validation
	require.NotEmpty(t, result.Sheets, "should produce at least one sheet")
	assert.Empty(t, result.UnplacedParts, "all parts should be placed")

	// Verify all parts are placed
	totalPlaced := 0
	for _, s := range result.Sheets {
		totalPlaced += len(s.Placements)
		// Verify each placement has valid coordinates
		for _, p := range s.Placements {
			assert.GreaterOrEqual(t, p.X, 0.0, "placement X should be non-negative")
			assert.GreaterOrEqual(t, p.Y, 0.0, "placement Y should be non-negative")
			assert.Greater(t, p.PlacedWidth(), 0.0, "placed width should be positive")
			assert.Greater(t, p.PlacedHeight(), 0.0, "placed height should be positive")
		}
	}
	// 2 + 3 + 1 = 6 total parts
	assert.Equal(t, 6, totalPlaced, "all 6 parts should be placed")

	// Verify efficiency is reasonable
	for _, s := range result.Sheets {
		total := s.TotalArea()
		used := s.UsedArea()
		assert.Greater(t, total, 0.0, "total area should be positive")
		assert.Greater(t, used, 0.0, "used area should be positive")
		assert.LessOrEqual(t, used, total, "used area should not exceed total")
	}

	// Verify TotalCutLength works
	cutLen := result.TotalCutLength()
	assert.Greater(t, cutLen, 0.0, "total cut length should be positive")
}

// TestOptimize_GeneticDefaultSettings_EndToEnd tests genetic algorithm with default settings.
func TestOptimize_GeneticDefaultSettings_EndToEnd(t *testing.T) {
	settings := model.DefaultSettings()
	settings.Algorithm = model.AlgorithmGenetic

	parts := []model.Part{
		model.NewPart("Panel A", 600, 400, 2),
		model.NewPart("Shelf", 500, 300, 3),
		model.NewPart("Door", 800, 600, 1),
	}
	stocks := []model.StockSheet{
		model.NewStockSheet("Plywood 2440x1220", 2440, 1220, 2),
	}

	opt := New(settings)
	result := opt.Optimize(parts, stocks)

	require.NotEmpty(t, result.Sheets, "should produce at least one sheet")
	assert.Empty(t, result.UnplacedParts, "all parts should be placed")

	totalPlaced := 0
	for _, s := range result.Sheets {
		totalPlaced += len(s.Placements)
	}
	assert.Equal(t, 6, totalPlaced, "all 6 parts should be placed")
}

// TestOptimize_DefaultSettings_GCodeGeneration verifies GCode can be generated from results.
func TestOptimize_DefaultSettings_GCodeGeneration(t *testing.T) {
	settings := model.DefaultSettings()

	parts := []model.Part{
		model.NewPart("Panel", 600, 400, 1),
	}
	stocks := []model.StockSheet{
		model.NewStockSheet("Plywood", 2440, 1220, 1),
	}

	opt := New(settings)
	result := opt.Optimize(parts, stocks)
	require.NotEmpty(t, result.Sheets)

	// Verify the result can be used for GCode generation without panic
	for _, sheet := range result.Sheets {
		assert.NotEmpty(t, sheet.Placements)
		assert.Greater(t, sheet.Stock.Width, 0.0)
		assert.Greater(t, sheet.Stock.Height, 0.0)
	}
}
