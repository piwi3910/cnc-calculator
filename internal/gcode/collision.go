package gcode

import (
	"fmt"
	"math"

	"github.com/piwi3910/SlabCut/internal/model"
)

// CheckDustShoeCollisions analyzes the optimization result and detects potential
// collisions between the dust shoe and clamp/fixture zones. The dust shoe is
// modeled as a circle centered on the tool with diameter DustShoeWidth. A collision
// occurs when the dust shoe circle (plus clearance) overlaps any clamp zone while
// the tool is at safe Z height during rapid moves or at cutting depth during cuts.
//
// Collision checking examines:
// 1. Cutting positions around each part perimeter (tool center + tool radius offset)
// 2. Rapid move positions (approach to each part from safe Z)
//
// A collision is reported when the distance from the tool center to the nearest
// clamp zone edge is less than dustShoeRadius + clearance.
func CheckDustShoeCollisions(result model.OptimizeResult, settings model.CutSettings) []model.DustShoeCollision {
	if !settings.DustShoeEnabled || len(settings.ClampZones) == 0 {
		return nil
	}

	dustShoeRadius := settings.DustShoeWidth / 2.0
	clearance := settings.DustShoeClearance
	effectiveRadius := dustShoeRadius + clearance
	toolRadius := settings.ToolDiameter / 2.0

	var collisions []model.DustShoeCollision

	for sheetIdx, sheet := range result.Sheets {
		for partIdx, placement := range sheet.Placements {
			// Get the tool path positions for this part's perimeter cut
			positions := partCutPositions(placement, toolRadius)

			for _, pos := range positions {
				for _, cz := range settings.ClampZones {
					dist := distanceToClampZone(pos.x, pos.y, cz)
					if dist < effectiveRadius {
						collisions = append(collisions, model.DustShoeCollision{
							SheetIndex:  sheetIdx,
							SheetLabel:  sheet.Stock.Label,
							ClampLabel:  cz.Label,
							PartLabel:   placement.Part.Label,
							PartIndex:   partIdx,
							ToolX:       pos.x,
							ToolY:       pos.y,
							Distance:    dist - dustShoeRadius,
							IsDuringCut: pos.isCut,
						})
						// Only report one collision per clamp per part to avoid flood
						break
					}
				}
			}
		}
	}

	return deduplicateCollisions(collisions)
}

// toolPosition represents a position the tool center visits during machining.
type toolPosition struct {
	x, y  float64
	isCut bool // true = during cutting, false = during rapid move
}

// partCutPositions returns the key positions the tool center will visit
// when cutting around a rectangular part's perimeter. The tool cuts outside
// the part boundary, offset by the tool radius.
func partCutPositions(p model.Placement, toolRadius float64) []toolPosition {
	pw := p.PlacedWidth()
	ph := p.PlacedHeight()

	// Tool center positions around the part perimeter (outside cut)
	x0 := p.X - toolRadius
	y0 := p.Y - toolRadius
	x1 := p.X + pw + toolRadius
	y1 := p.Y + ph + toolRadius

	// Sample corner positions and midpoints of each side
	positions := []toolPosition{
		// Corners (most likely to collide with clamps at sheet edges)
		{x0, y0, true},
		{x1, y0, true},
		{x1, y1, true},
		{x0, y1, true},
		// Side midpoints
		{(x0 + x1) / 2, y0, true},
		{x1, (y0 + y1) / 2, true},
		{(x0 + x1) / 2, y1, true},
		{x0, (y0 + y1) / 2, true},
		// Part center (rapid approach position)
		{p.X + pw/2, p.Y + ph/2, false},
	}

	return positions
}

// distanceToClampZone computes the minimum distance from a point (px, py)
// to the boundary of a clamp zone rectangle. Returns 0 if the point is
// inside the zone, positive if outside.
func distanceToClampZone(px, py float64, cz model.ClampZone) float64 {
	// Find the nearest point on the clamp zone rectangle to (px, py)
	nearestX := math.Max(cz.X, math.Min(px, cz.X+cz.Width))
	nearestY := math.Max(cz.Y, math.Min(py, cz.Y+cz.Height))

	dx := px - nearestX
	dy := py - nearestY

	return math.Sqrt(dx*dx + dy*dy)
}

// deduplicateCollisions keeps at most one collision per (sheet, part, clamp) triple.
func deduplicateCollisions(collisions []model.DustShoeCollision) []model.DustShoeCollision {
	type key struct {
		sheet int
		part  int
		clamp string
	}
	seen := make(map[key]bool)
	var result []model.DustShoeCollision

	for _, c := range collisions {
		k := key{c.SheetIndex, c.PartIndex, c.ClampLabel}
		if !seen[k] {
			seen[k] = true
			result = append(result, c)
		}
	}
	return result
}

// FormatCollisionWarnings produces human-readable warning messages from collision data.
func FormatCollisionWarnings(collisions []model.DustShoeCollision) []string {
	var warnings []string
	for _, c := range collisions {
		moveType := "cutting"
		if !c.IsDuringCut {
			moveType = "rapid"
		}
		msg := fmt.Sprintf(
			"Sheet %d (%s): Dust shoe may collide with clamp %q while %s part %q at (%.0f, %.0f) — clearance: %.1f mm",
			c.SheetIndex+1, c.SheetLabel, c.ClampLabel, moveType,
			c.PartLabel, c.ToolX, c.ToolY, c.Distance,
		)
		warnings = append(warnings, msg)
	}
	return warnings
}
