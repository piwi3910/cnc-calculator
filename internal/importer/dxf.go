package importer

import (
	"fmt"
	"math"
	"sort"

	"github.com/piwi3910/cnc-calculator/internal/model"
	"github.com/yofu/dxf"
	"github.com/yofu/dxf/entity"
)

// segment represents a line segment between two 2D points, used for
// chaining disconnected LINE entities into closed outlines.
type segment struct {
	start model.Point2D
	end   model.Point2D
}

// ImportDXF imports parts from a DXF file. Each closed shape (LWPOLYLINE,
// CIRCLE, or chain of connected LINEs/ARCs) becomes a separate Part with
// an Outline and bounding-box Width/Height.
func ImportDXF(path string) ImportResult {
	result := ImportResult{}

	drawing, err := dxf.Open(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Cannot open DXF file: %v", err))
		return result
	}

	entities := drawing.Entities()
	if len(entities) == 0 {
		result.Errors = append(result.Errors, "DXF file contains no entities")
		return result
	}

	var outlines []model.Outline
	var segments []segment

	partNum := 0
	for _, ent := range entities {
		switch e := ent.(type) {
		case *entity.LwPolyline:
			outline := lwPolylineToOutline(e)
			if len(outline) >= 3 {
				outlines = append(outlines, outline)
			} else {
				result.Warnings = append(result.Warnings,
					"Skipped LWPOLYLINE with fewer than 3 vertices")
			}

		case *entity.Circle:
			outline := circleToOutline(e, 64)
			outlines = append(outlines, outline)

		case *entity.Arc:
			pts := arcToPoints(e, 32)
			if len(pts) >= 2 {
				segments = append(segments, pointsToSegments(pts)...)
			}

		case *entity.Line:
			seg := segment{
				start: model.Point2D{X: e.Start[0], Y: e.Start[1]},
				end:   model.Point2D{X: e.End[0], Y: e.End[1]},
			}
			segments = append(segments, seg)

		default:
			// Unsupported entity types are silently skipped
		}
	}

	// Chain loose segments (LINEs and ARCs) into closed outlines
	chainedOutlines := chainSegments(segments, 0.01)
	for _, co := range chainedOutlines {
		if len(co) >= 3 {
			outlines = append(outlines, co)
		}
	}

	if len(outlines) == 0 {
		result.Errors = append(result.Errors, "No closed shapes found in DXF file")
		return result
	}

	for _, outline := range outlines {
		partNum++
		normalized := normalizeOutline(outline)
		min, max := normalized.BoundingBox()
		width := max.X - min.X
		height := max.Y - min.Y

		if width < 0.01 || height < 0.01 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Skipped degenerate shape (%.2f x %.2f mm)", width, height))
			continue
		}

		part := model.NewPart(fmt.Sprintf("DXF Part %d", partNum), width, height, 1)
		part.Outline = normalized
		result.Parts = append(result.Parts, part)
	}

	return result
}

// lwPolylineToOutline converts a DXF LWPOLYLINE entity to an Outline.
// Bulge values on vertices produce interpolated arc segments.
func lwPolylineToOutline(lw *entity.LwPolyline) model.Outline {
	var outline model.Outline

	for i := 0; i < len(lw.Vertices); i++ {
		v := lw.Vertices[i]
		current := model.Point2D{X: v[0], Y: v[1]}

		bulge := 0.0
		if i < len(lw.Bulges) {
			bulge = lw.Bulges[i]
		}

		if math.Abs(bulge) > 1e-9 {
			// This vertex has a bulge: interpolate an arc to the next vertex
			nextIdx := (i + 1) % len(lw.Vertices)
			next := model.Point2D{X: lw.Vertices[nextIdx][0], Y: lw.Vertices[nextIdx][1]}
			arcPts := bulgeArcPoints(current, next, bulge, 32)
			// Add all but the last point (next vertex will be added naturally)
			outline = append(outline, arcPts[:len(arcPts)-1]...)
		} else {
			outline = append(outline, current)
		}
	}

	return outline
}

// bulgeArcPoints generates points along an arc defined by two endpoints and a
// DXF bulge factor. The bulge is the tangent of 1/4 the included angle.
func bulgeArcPoints(p1, p2 model.Point2D, bulge float64, numSegments int) model.Outline {
	// Chord midpoint and length
	mx := (p1.X + p2.X) / 2
	my := (p1.Y + p2.Y) / 2
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	chordLen := math.Sqrt(dx*dx + dy*dy)
	if chordLen < 1e-9 {
		return model.Outline{p1, p2}
	}

	// Sagitta and radius
	sagitta := math.Abs(bulge) * chordLen / 2
	radius := (chordLen*chordLen/(4*sagitta) + sagitta) / 2

	// Center of the arc
	// perpendicular direction from chord midpoint
	perpX := -dy / chordLen
	perpY := dx / chordLen
	dist := radius - sagitta
	if bulge > 0 {
		perpX, perpY = -perpX, -perpY
	}
	cx := mx + perpX*dist
	cy := my + perpY*dist

	// Start and end angles
	startAngle := math.Atan2(p1.Y-cy, p1.X-cx)
	endAngle := math.Atan2(p2.Y-cy, p2.X-cx)

	// Determine sweep direction
	includedAngle := 4 * math.Atan(math.Abs(bulge))
	if bulge < 0 {
		// Clockwise arc
		if endAngle > startAngle {
			endAngle -= 2 * math.Pi
		}
	} else {
		// Counter-clockwise arc
		if endAngle < startAngle {
			endAngle += 2 * math.Pi
		}
	}

	_ = includedAngle // used for reference, actual sweep from angles

	var pts model.Outline
	for i := 0; i <= numSegments; i++ {
		t := float64(i) / float64(numSegments)
		angle := startAngle + t*(endAngle-startAngle)
		pts = append(pts, model.Point2D{
			X: cx + radius*math.Cos(angle),
			Y: cy + radius*math.Sin(angle),
		})
	}
	return pts
}

// circleToOutline approximates a circle as a regular polygon.
func circleToOutline(c *entity.Circle, numSegments int) model.Outline {
	outline := make(model.Outline, numSegments)
	cx, cy, r := c.Center[0], c.Center[1], c.Radius
	for i := 0; i < numSegments; i++ {
		angle := 2 * math.Pi * float64(i) / float64(numSegments)
		outline[i] = model.Point2D{
			X: cx + r*math.Cos(angle),
			Y: cy + r*math.Sin(angle),
		}
	}
	return outline
}

// arcToPoints converts a DXF ARC entity to a series of line points.
func arcToPoints(a *entity.Arc, numSegments int) []model.Point2D {
	cx, cy := a.Circle.Center[0], a.Circle.Center[1]
	r := a.Circle.Radius
	startDeg := a.Angle[0]
	endDeg := a.Angle[1]

	startRad := startDeg * math.Pi / 180
	endRad := endDeg * math.Pi / 180
	if endRad <= startRad {
		endRad += 2 * math.Pi
	}

	pts := make([]model.Point2D, numSegments+1)
	for i := 0; i <= numSegments; i++ {
		t := float64(i) / float64(numSegments)
		angle := startRad + t*(endRad-startRad)
		pts[i] = model.Point2D{
			X: cx + r*math.Cos(angle),
			Y: cy + r*math.Sin(angle),
		}
	}
	return pts
}

// pointsToSegments converts a point sequence to a slice of connected segments.
func pointsToSegments(pts []model.Point2D) []segment {
	segs := make([]segment, 0, len(pts)-1)
	for i := 0; i < len(pts)-1; i++ {
		segs = append(segs, segment{start: pts[i], end: pts[i+1]})
	}
	return segs
}

// chainSegments connects individual segments into closed outlines.
// tolerance is the maximum distance between endpoints to consider them connected.
func chainSegments(segs []segment, tolerance float64) []model.Outline {
	if len(segs) == 0 {
		return nil
	}

	used := make([]bool, len(segs))
	var outlines []model.Outline

	for {
		// Find the first unused segment
		startIdx := -1
		for i, u := range used {
			if !u {
				startIdx = i
				break
			}
		}
		if startIdx == -1 {
			break
		}

		chain := []model.Point2D{segs[startIdx].start, segs[startIdx].end}
		used[startIdx] = true

		// Try to extend the chain
		changed := true
		for changed {
			changed = false
			tail := chain[len(chain)-1]

			for i, seg := range segs {
				if used[i] {
					continue
				}
				if pointsClose(tail, seg.start, tolerance) {
					chain = append(chain, seg.end)
					used[i] = true
					changed = true
					break
				}
				if pointsClose(tail, seg.end, tolerance) {
					chain = append(chain, seg.start)
					used[i] = true
					changed = true
					break
				}
			}
		}

		// Check if the chain is closed
		if len(chain) >= 3 && pointsClose(chain[0], chain[len(chain)-1], tolerance) {
			// Remove the duplicate closing point
			chain = chain[:len(chain)-1]
		}

		if len(chain) >= 3 {
			outlines = append(outlines, model.Outline(chain))
		}
	}

	// Sort outlines by area (largest first) for consistent ordering
	sort.Slice(outlines, func(i, j int) bool {
		return outlineArea(outlines[i]) > outlineArea(outlines[j])
	})

	return outlines
}

// pointsClose checks whether two points are within the given tolerance.
func pointsClose(a, b model.Point2D, tolerance float64) bool {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx+dy*dy) <= tolerance
}

// outlineArea computes the absolute area of a polygon using the shoelace formula.
func outlineArea(o model.Outline) float64 {
	n := len(o)
	if n < 3 {
		return 0
	}
	var area float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += o[i].X * o[j].Y
		area -= o[j].X * o[i].Y
	}
	return math.Abs(area) / 2
}

// normalizeOutline translates the outline so its bounding box starts at (0, 0).
func normalizeOutline(o model.Outline) model.Outline {
	if len(o) == 0 {
		return o
	}
	min, _ := o.BoundingBox()
	return o.Translate(-min.X, -min.Y)
}
