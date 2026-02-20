package gcode

import (
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistanceToClampZone_PointOutside(t *testing.T) {
	cz := model.ClampZone{X: 100, Y: 100, Width: 50, Height: 50}

	// Point to the left
	d := distanceToClampZone(80, 125, cz)
	assert.InDelta(t, 20.0, d, 0.001, "should be 20mm to the left")

	// Point above
	d = distanceToClampZone(125, 80, cz)
	assert.InDelta(t, 20.0, d, 0.001, "should be 20mm above")

	// Point at corner diagonal
	d = distanceToClampZone(80, 80, cz)
	expected := 28.284 // sqrt(20^2 + 20^2)
	assert.InDelta(t, expected, d, 0.01, "diagonal distance should be ~28.28mm")
}

func TestDistanceToClampZone_PointInside(t *testing.T) {
	cz := model.ClampZone{X: 100, Y: 100, Width: 50, Height: 50}

	d := distanceToClampZone(125, 125, cz)
	assert.InDelta(t, 0.0, d, 0.001, "point inside zone should have distance 0")
}

func TestDistanceToClampZone_PointOnEdge(t *testing.T) {
	cz := model.ClampZone{X: 100, Y: 100, Width: 50, Height: 50}

	d := distanceToClampZone(100, 125, cz)
	assert.InDelta(t, 0.0, d, 0.001, "point on edge should have distance 0")
}

func TestCheckDustShoeCollisions_NoClamps(t *testing.T) {
	settings := model.DefaultSettings()
	settings.DustShoeEnabled = true
	settings.ClampZones = nil

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock:      model.NewStockSheet("Sheet", 1000, 600, 1),
				Placements: []model.Placement{{Part: model.NewPart("A", 200, 100, 1), X: 50, Y: 50}},
			},
		},
	}

	collisions := CheckDustShoeCollisions(result, settings)
	assert.Empty(t, collisions, "no clamps means no collisions")
}

func TestCheckDustShoeCollisions_Disabled(t *testing.T) {
	settings := model.DefaultSettings()
	settings.DustShoeEnabled = false
	settings.ClampZones = []model.ClampZone{
		{Label: "Clamp", X: 0, Y: 0, Width: 50, Height: 50, ZHeight: 25},
	}

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock:      model.NewStockSheet("Sheet", 1000, 600, 1),
				Placements: []model.Placement{{Part: model.NewPart("A", 200, 100, 1), X: 10, Y: 10}},
			},
		},
	}

	collisions := CheckDustShoeCollisions(result, settings)
	assert.Empty(t, collisions, "disabled dust shoe means no collision checking")
}

func TestCheckDustShoeCollisions_DetectsNearbyClamp(t *testing.T) {
	settings := model.DefaultSettings()
	settings.DustShoeEnabled = true
	settings.DustShoeWidth = 80.0      // 40mm radius
	settings.DustShoeClearance = 5.0   // 5mm clearance
	settings.ToolDiameter = 6.0        // 3mm tool radius

	// Clamp at origin, part very close to it — tool perimeter will be near clamp
	settings.ClampZones = []model.ClampZone{
		{Label: "CornerClamp", X: 0, Y: 0, Width: 50, Height: 50, ZHeight: 25},
	}

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.NewStockSheet("Sheet", 1000, 600, 1),
				Placements: []model.Placement{
					{Part: model.NewPart("NearClamp", 200, 100, 1), X: 55, Y: 55, Rotated: false},
				},
			},
		},
	}

	collisions := CheckDustShoeCollisions(result, settings)
	require.NotEmpty(t, collisions, "should detect collision with nearby clamp")
	assert.Equal(t, "CornerClamp", collisions[0].ClampLabel)
	assert.Equal(t, "NearClamp", collisions[0].PartLabel)
}

func TestCheckDustShoeCollisions_FarPartNoColl(t *testing.T) {
	settings := model.DefaultSettings()
	settings.DustShoeEnabled = true
	settings.DustShoeWidth = 80.0
	settings.DustShoeClearance = 5.0
	settings.ToolDiameter = 6.0

	// Clamp at origin, part far away
	settings.ClampZones = []model.ClampZone{
		{Label: "Clamp", X: 0, Y: 0, Width: 50, Height: 50, ZHeight: 25},
	}

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.NewStockSheet("Sheet", 1000, 600, 1),
				Placements: []model.Placement{
					{Part: model.NewPart("FarPart", 200, 100, 1), X: 500, Y: 300, Rotated: false},
				},
			},
		},
	}

	collisions := CheckDustShoeCollisions(result, settings)
	assert.Empty(t, collisions, "far part should not collide with clamp")
}

func TestCheckDustShoeCollisions_MultipleClamps(t *testing.T) {
	settings := model.DefaultSettings()
	settings.DustShoeEnabled = true
	settings.DustShoeWidth = 80.0
	settings.DustShoeClearance = 5.0
	settings.ToolDiameter = 6.0

	settings.ClampZones = []model.ClampZone{
		{Label: "CL1", X: 0, Y: 0, Width: 50, Height: 50, ZHeight: 25},
		{Label: "CL2", X: 950, Y: 550, Width: 50, Height: 50, ZHeight: 25},
	}

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.NewStockSheet("Sheet", 1000, 600, 1),
				Placements: []model.Placement{
					// Near first clamp
					{Part: model.NewPart("A", 100, 80, 1), X: 55, Y: 55, Rotated: false},
					// Near second clamp
					{Part: model.NewPart("B", 100, 80, 1), X: 840, Y: 470, Rotated: false},
				},
			},
		},
	}

	collisions := CheckDustShoeCollisions(result, settings)
	require.GreaterOrEqual(t, len(collisions), 2, "should detect collisions with both clamps")
}

func TestFormatCollisionWarnings(t *testing.T) {
	collisions := []model.DustShoeCollision{
		{
			SheetIndex: 0, SheetLabel: "Sheet1",
			ClampLabel: "Clamp1", PartLabel: "Part A",
			ToolX: 100, ToolY: 200, Distance: -5.3,
			IsDuringCut: true,
		},
		{
			SheetIndex: 1, SheetLabel: "Sheet2",
			ClampLabel: "Clamp2", PartLabel: "Part B",
			ToolX: 50, ToolY: 75, Distance: 2.1,
			IsDuringCut: false,
		},
	}

	warnings := FormatCollisionWarnings(collisions)
	require.Len(t, warnings, 2)
	assert.Contains(t, warnings[0], "Sheet 1")
	assert.Contains(t, warnings[0], "Clamp1")
	assert.Contains(t, warnings[0], "cutting")
	assert.Contains(t, warnings[1], "rapid")
}

func TestDeduplicateCollisions(t *testing.T) {
	collisions := []model.DustShoeCollision{
		{SheetIndex: 0, PartIndex: 0, ClampLabel: "CL1"},
		{SheetIndex: 0, PartIndex: 0, ClampLabel: "CL1"}, // duplicate
		{SheetIndex: 0, PartIndex: 0, ClampLabel: "CL2"}, // different clamp
		{SheetIndex: 0, PartIndex: 1, ClampLabel: "CL1"}, // different part
	}

	result := deduplicateCollisions(collisions)
	assert.Len(t, result, 3, "should remove 1 duplicate")
}

func TestPartCutPositions(t *testing.T) {
	p := model.Placement{
		Part: model.NewPart("Test", 200, 100, 1),
		X:    50,
		Y:    50,
	}

	positions := partCutPositions(p, 3.0)
	require.Len(t, positions, 9, "should have 4 corners + 4 midpoints + 1 center")

	// First position should be tool offset from part origin
	assert.InDelta(t, 47.0, positions[0].x, 0.001) // 50 - 3.0
	assert.InDelta(t, 47.0, positions[0].y, 0.001) // 50 - 3.0
	assert.True(t, positions[0].isCut)

	// Last position should be center (rapid approach)
	assert.InDelta(t, 150.0, positions[8].x, 0.001) // 50 + 200/2
	assert.InDelta(t, 100.0, positions[8].y, 0.001) // 50 + 100/2
	assert.False(t, positions[8].isCut)
}
