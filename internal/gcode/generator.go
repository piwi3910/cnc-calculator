package gcode

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/piwi3910/SlabCut/internal/model"
)

// Generator produces GCode from an optimized sheet layout.
type Generator struct {
	Settings model.CutSettings
	profile  model.GCodeProfile
}

func New(settings model.CutSettings) *Generator {
	return &Generator{
		Settings: settings,
		profile:  model.GetProfile(settings.GCodeProfile),
	}
}

// GenerateSheet produces GCode for a single sheet's placements.
func (g *Generator) GenerateSheet(sheet model.SheetResult, sheetIndex int) string {
	var b strings.Builder

	g.writeHeader(&b, sheet, sheetIndex)

	placements := sheet.Placements
	if g.Settings.StructuralOrdering && len(placements) > 1 {
		placements = g.structuralOrderPlacements(placements, sheet.Stock.Width, sheet.Stock.Height)
		b.WriteString(g.comment("Cut ordering: structural integrity (center-out)"))
	} else if g.Settings.OptimizeToolpath && len(placements) > 1 {
		placements = g.orderPlacements(placements)
		b.WriteString(g.comment("Toolpath ordering: nearest-neighbor optimization"))
	}

	for i, placement := range placements {
		g.writePart(&b, placement, i+1)
	}

	g.writeFooter(&b)
	return b.String()
}

// GenerateAll produces one GCode string per sheet.
func (g *Generator) GenerateAll(result model.OptimizeResult) []string {
	var codes []string
	for i, sheet := range result.Sheets {
		codes = append(codes, g.GenerateSheet(sheet, i+1))
	}
	return codes
}

// orderPlacements reorders placements using a nearest-neighbor heuristic to
// minimize total rapid travel distance. Starting from the origin (0,0), each
// subsequent placement is chosen as the one closest to the previous placement's
// center. This reduces machine idle time and total cycle time.
func (g *Generator) orderPlacements(placements []model.Placement) []model.Placement {
	n := len(placements)
	if n <= 1 {
		return placements
	}

	// Work on a copy to avoid modifying the original
	remaining := make([]model.Placement, n)
	copy(remaining, placements)
	ordered := make([]model.Placement, 0, n)

	// Start from origin (0, 0)
	curX, curY := 0.0, 0.0

	for len(remaining) > 0 {
		bestIdx := 0
		bestDist := math.MaxFloat64

		for i, p := range remaining {
			// Use the center of the placement as the reference point
			cx := p.X + p.PlacedWidth()/2.0
			cy := p.Y + p.PlacedHeight()/2.0
			dx := cx - curX
			dy := cy - curY
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < bestDist {
				bestDist = dist
				bestIdx = i
			}
		}

		chosen := remaining[bestIdx]
		ordered = append(ordered, chosen)
		curX = chosen.X + chosen.PlacedWidth()/2.0
		curY = chosen.Y + chosen.PlacedHeight()/2.0

		// Remove chosen from remaining
		remaining[bestIdx] = remaining[len(remaining)-1]
		remaining = remaining[:len(remaining)-1]
	}

	return ordered
}

// structuralOrderPlacements reorders placements to maintain structural integrity
// of the stock sheet during machining. Interior parts (those furthest from all
// sheet edges) are cut first, and parts near the edges are cut last.
//
// This preserves the rigidity of the surrounding material while interior cuts
// are being made, reducing vibration, chatter, and part movement.
//
// The algorithm computes a "minimum edge distance" for each placement — the
// shortest distance from the placement's bounding box to any sheet edge. Parts
// with larger minimum edge distances are more interior and are cut first.
// Ties are broken by distance from the sheet center (closer to center = first).
func (g *Generator) structuralOrderPlacements(placements []model.Placement, sheetW, sheetH float64) []model.Placement {
	n := len(placements)
	if n <= 1 {
		return placements
	}

	// Work on a copy
	ordered := make([]model.Placement, n)
	copy(ordered, placements)

	sheetCX := sheetW / 2.0
	sheetCY := sheetH / 2.0

	sort.SliceStable(ordered, func(i, j int) bool {
		di := minEdgeDistance(ordered[i], sheetW, sheetH)
		dj := minEdgeDistance(ordered[j], sheetW, sheetH)

		// Higher min-edge-distance = more interior = should be cut first (sort earlier)
		if math.Abs(di-dj) > 0.01 {
			return di > dj
		}

		// Tie-break: closer to center of sheet = cut first
		ci := centerDistance(ordered[i], sheetCX, sheetCY)
		cj := centerDistance(ordered[j], sheetCX, sheetCY)
		return ci < cj
	})

	return ordered
}

// minEdgeDistance returns the minimum distance from a placement's bounding box
// to any edge of the stock sheet. A larger value means the part is more interior.
func minEdgeDistance(p model.Placement, sheetW, sheetH float64) float64 {
	pw := p.PlacedWidth()
	ph := p.PlacedHeight()

	distLeft := p.X
	distRight := sheetW - (p.X + pw)
	distTop := p.Y
	distBottom := sheetH - (p.Y + ph)

	min := distLeft
	if distRight < min {
		min = distRight
	}
	if distTop < min {
		min = distTop
	}
	if distBottom < min {
		min = distBottom
	}
	return min
}

// centerDistance returns the Euclidean distance from a placement's center
// to the given center point.
func centerDistance(p model.Placement, cx, cy float64) float64 {
	px := p.X + p.PlacedWidth()/2.0
	py := p.Y + p.PlacedHeight()/2.0
	dx := px - cx
	dy := py - cy
	return math.Sqrt(dx*dx + dy*dy)
}

// placementDistance returns the Euclidean distance between the centers of two placements.
func placementDistance(a, b model.Placement) float64 {
	ax := a.X + a.PlacedWidth()/2.0
	ay := a.Y + a.PlacedHeight()/2.0
	bx := b.X + b.PlacedWidth()/2.0
	by := b.Y + b.PlacedHeight()/2.0
	dx := ax - bx
	dy := ay - by
	return math.Sqrt(dx*dx + dy*dy)
}

// TotalRapidDistance calculates the total rapid travel distance for a sequence
// of placements, starting from the origin (0,0). This is useful for comparing
// ordered vs unordered toolpaths.
func TotalRapidDistance(placements []model.Placement) float64 {
	if len(placements) == 0 {
		return 0
	}
	total := 0.0
	curX, curY := 0.0, 0.0
	for _, p := range placements {
		cx := p.X + p.PlacedWidth()/2.0
		cy := p.Y + p.PlacedHeight()/2.0
		dx := cx - curX
		dy := cy - curY
		total += math.Sqrt(dx*dx + dy*dy)
		curX = cx
		curY = cy
	}
	return total
}

func (g *Generator) writeHeader(b *strings.Builder, sheet model.SheetResult, idx int) {
	p := g.profile

	// Write file header comment
	b.WriteString(p.CommentPrefix)
	b.WriteString(fmt.Sprintf(" CNCCalculator GCode — Sheet %d (%s)\n", idx, sheet.Stock.Label))
	b.WriteString(p.CommentPrefix)
	b.WriteString(fmt.Sprintf(" Stock: %.1f x %.1f mm\n", sheet.Stock.Width, sheet.Stock.Height))
	b.WriteString(p.CommentPrefix)
	b.WriteString(fmt.Sprintf(" Parts: %d, Efficiency: %.1f%%\n", len(sheet.Placements), sheet.Efficiency()))
	b.WriteString(p.CommentPrefix)
	b.WriteString(fmt.Sprintf(" Tool: %.1fmm, Feed: %.0f mm/min, Plunge: %.0f mm/min\n",
		g.Settings.ToolDiameter, g.Settings.FeedRate, g.Settings.PlungeRate))
	b.WriteString(p.CommentPrefix)
	b.WriteString(fmt.Sprintf(" Depth: %.1fmm in %.1fmm passes\n", g.Settings.CutDepth, g.Settings.PassDepth))
	b.WriteString(p.CommentPrefix)
	b.WriteString(fmt.Sprintf(" Profile: %s\n", p.Name))
	b.WriteString("\n")

	// Write startup codes
	for _, code := range p.StartCode {
		b.WriteString(code + "\n")
	}

	// Spindle start
	if p.SpindleStart != "" {
		b.WriteString(fmt.Sprintf(p.SpindleStart+"\n", g.Settings.SpindleSpeed))
	}

	// Initial safe Z retract
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", p.RapidMove, g.format(0), g.format(0)))
	b.WriteString(fmt.Sprintf("%s Z%s\n", p.RapidMove, g.format(g.Settings.SafeZ)))

	b.WriteString("\n")
}

func (g *Generator) writeFooter(b *strings.Builder) {
	p := g.profile

	b.WriteString("\n")
	b.WriteString(p.CommentPrefix + " === Job complete ===\n")

	// Write end codes
	for _, code := range p.EndCode {
		// Replace [SafeZ] placeholder
		code = strings.ReplaceAll(code, "[SafeZ]", g.format(g.Settings.SafeZ))
		b.WriteString(code + "\n")
	}

	// Spindle stop
	if p.SpindleStop != "" {
		b.WriteString(p.SpindleStop + "\n")
	}
}

// writePlunge generates the plunge entry at position (x, y) to the given depth
// using the configured plunge strategy (direct, ramp, or helix).
// The tool is assumed to already be at (x, y) at safe Z height.
func (g *Generator) writePlunge(b *strings.Builder, x, y, depth float64) {
	switch g.Settings.PlungeType {
	case model.PlungeRamp:
		g.writeRampPlunge(b, x, y, depth)
	case model.PlungeHelix:
		g.writeHelixPlunge(b, x, y, depth)
	default:
		g.writeDirectPlunge(b, depth)
	}
}

// writeDirectPlunge performs a standard straight-down plunge.
func (g *Generator) writeDirectPlunge(b *strings.Builder, depth float64) {
	b.WriteString(fmt.Sprintf("%s Z%s F%s\n", g.profile.FeedMove,
		g.format(-depth), g.format(g.Settings.PlungeRate)))
}

// writeRampPlunge generates a linear ramp entry. The tool moves forward along X
// while simultaneously descending to the target depth, creating an angled entry
// that reduces axial load on the tool. The ramp length is calculated from the
// configured ramp angle and the depth to plunge.
func (g *Generator) writeRampPlunge(b *strings.Builder, x, y, depth float64) {
	angle := g.Settings.RampAngle
	if angle <= 0 {
		angle = 3.0 // Default 3 degrees
	}
	if angle > 45.0 {
		angle = 45.0 // Cap at 45 degrees
	}

	// Calculate ramp length from angle: length = depth / tan(angle)
	rampLength := depth / math.Tan(angle*math.Pi/180.0)

	b.WriteString(g.comment(fmt.Sprintf("Ramp plunge entry (%.1f deg, length=%.2fmm)", angle, rampLength)))

	// Rapid to safe Z at current position
	b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.RapidMove, g.format(0)))

	// Ramp down: move forward along X while descending
	rampEndX := x + rampLength
	b.WriteString(fmt.Sprintf("%s X%s Y%s Z%s F%s\n", g.profile.FeedMove,
		g.format(rampEndX), g.format(y), g.format(-depth), g.format(g.Settings.PlungeRate)))

	// Move back to original X at cut depth
	b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove,
		g.format(x), g.format(y), g.format(g.Settings.FeedRate)))
}

// writeHelixPlunge generates a helical plunge entry. The tool descends in a
// circular helix pattern, distributing the cutting force over a larger area
// and reducing heat buildup. The helix diameter and depth per revolution are
// configurable.
func (g *Generator) writeHelixPlunge(b *strings.Builder, x, y, depth float64) {
	diameter := g.Settings.HelixDiameter
	if diameter <= 0 {
		diameter = g.Settings.ToolDiameter // Default to tool diameter
	}
	radius := diameter / 2.0

	// Depth per revolution as a percentage of pass depth
	revPercent := g.Settings.HelixRevPercent
	if revPercent <= 0 {
		revPercent = 50.0 // Default 50%
	}
	depthPerRev := g.Settings.PassDepth * revPercent / 100.0
	if depthPerRev <= 0 {
		depthPerRev = 1.0 // Minimum 1mm per revolution
	}

	numRevolutions := math.Ceil(depth / depthPerRev)
	if numRevolutions < 1 {
		numRevolutions = 1
	}

	b.WriteString(g.comment(fmt.Sprintf("Helix plunge entry (dia=%.1fmm, %.1f rev)", diameter, numRevolutions)))

	// Move to helix start position (center + radius offset in X)
	helixStartX := x + radius
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove,
		g.format(helixStartX), g.format(y)))
	b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.RapidMove, g.format(0)))

	// Generate helix revolutions using G2/G3 arcs with Z descent
	// I offset = distance from current position to center in X
	// J offset = distance from current position to center in Y
	iOffset := -radius // Center is to the left of start position
	jOffset := 0.0

	arcCmd := "G2" // Clockwise helix
	if g.Settings.UseClimb {
		arcCmd = "G3" // Counter-clockwise for climb milling
	}

	currentDepth := 0.0
	for rev := 0; rev < int(numRevolutions); rev++ {
		currentDepth += depthPerRev
		if currentDepth > depth {
			currentDepth = depth
		}
		// Full circle arc back to the same XY position, but lower in Z
		b.WriteString(fmt.Sprintf("%s X%s Y%s Z%s I%s J%s F%s\n",
			arcCmd,
			g.format(helixStartX), g.format(y), g.format(-currentDepth),
			g.format(iOffset), g.format(jOffset),
			g.format(g.Settings.PlungeRate)))
	}

	// Move back to the original position at cut depth
	b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove,
		g.format(x), g.format(y), g.format(g.Settings.FeedRate)))
}

func (g *Generator) writePart(b *strings.Builder, p model.Placement, partNum int) {
	if len(p.Part.Outline) > 0 {
		g.writeOutlinePart(b, p, partNum)
	} else {
		g.writeRectPart(b, p, partNum)
	}
}

// writeOutlinePart generates GCode that follows the actual part outline
// instead of a rectangular perimeter.
func (g *Generator) writeOutlinePart(b *strings.Builder, p model.Placement, partNum int) {
	toolR := g.Settings.ToolDiameter / 2.0

	b.WriteString(g.comment(fmt.Sprintf("--- Part %d: %s (%.1f x %.1f, outline)%s ---",
		partNum, p.Part.Label, p.Part.Width, p.Part.Height,
		rotatedStr(p.Rotated))))

	// Build the toolpath: offset each outline point outward by tool radius
	outline := g.offsetOutline(p.Part.Outline, toolR)

	// Translate outline to placement position on the stock sheet
	translated := make(model.Outline, len(outline))
	for i, pt := range outline {
		translated[i] = model.Point2D{X: pt.X + p.X, Y: pt.Y + p.Y}
	}

	if len(translated) < 3 {
		b.WriteString(g.comment("WARNING: outline has fewer than 3 points, skipping"))
		return
	}

	numPasses := int(math.Ceil(g.Settings.CutDepth / g.Settings.PassDepth))

	for pass := 1; pass <= numPasses; pass++ {
		depth := float64(pass) * g.Settings.PassDepth
		if depth > g.Settings.CutDepth {
			depth = g.Settings.CutDepth
		}
		isFinalPass := pass == numPasses

		// Apply onion skin on final pass
		effectiveDepth, skinApplied := g.applyOnionSkin(depth, isFinalPass)
		if skinApplied {
			b.WriteString(g.comment(fmt.Sprintf("Onion skin: leaving %.2fmm skin", g.Settings.OnionSkinDepth)))
		}

		b.WriteString(g.comment(fmt.Sprintf("Pass %d/%d, depth=%.2fmm", pass, numPasses, effectiveDepth)))

		// Rapid to first point
		b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove,
			g.format(translated[0].X), g.format(translated[0].Y)))
		// Plunge using configured strategy
		g.writePlunge(b, translated[0].X, translated[0].Y, effectiveDepth)

		// Follow outline
		for i := 1; i < len(translated); i++ {
			b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove,
				g.format(translated[i].X), g.format(translated[i].Y),
				g.format(g.Settings.FeedRate)))
		}
		// Close the loop back to the first point
		b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove,
			g.format(translated[0].X), g.format(translated[0].Y),
			g.format(g.Settings.FeedRate)))

		// Retract
		b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.RapidMove, g.format(g.Settings.SafeZ)))
	}

	// Onion skin cleanup pass for outline parts
	if g.onionSkinActive() && g.Settings.OnionSkinCleanup {
		fullDepth := g.Settings.CutDepth
		b.WriteString(g.comment("Onion skin cleanup pass"))
		b.WriteString(g.comment(fmt.Sprintf("Cleanup depth=%.2fmm (removing %.2fmm skin)",
			fullDepth, g.Settings.OnionSkinDepth)))

		b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove,
			g.format(translated[0].X), g.format(translated[0].Y)))
		g.writePlunge(b, translated[0].X, translated[0].Y, fullDepth)

		for i := 1; i < len(translated); i++ {
			b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove,
				g.format(translated[i].X), g.format(translated[i].Y),
				g.format(g.Settings.FeedRate)))
		}
		b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove,
			g.format(translated[0].X), g.format(translated[0].Y),
			g.format(g.Settings.FeedRate)))
		b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.RapidMove, g.format(g.Settings.SafeZ)))
	}

	b.WriteString("\n")
}

// offsetOutline creates a simple outward offset of the outline by the given
// distance. For each vertex, it computes the average outward normal of the
// two adjacent edges and shifts the vertex along that normal.
func (g *Generator) offsetOutline(outline model.Outline, dist float64) model.Outline {
	n := len(outline)
	if n < 3 {
		return outline
	}

	result := make(model.Outline, n)
	for i := 0; i < n; i++ {
		prev := outline[(i-1+n)%n]
		curr := outline[i]
		next := outline[(i+1)%n]

		// Edge vectors
		e1x := curr.X - prev.X
		e1y := curr.Y - prev.Y
		e2x := next.X - curr.X
		e2y := next.Y - curr.Y

		// Outward normals (perpendicular, pointing left of travel direction)
		n1x, n1y := normalize(-e1y, e1x)
		n2x, n2y := normalize(-e2y, e2x)

		// Average normal
		nx := (n1x + n2x) / 2
		ny := (n1y + n2y) / 2
		nLen := math.Sqrt(nx*nx + ny*ny)
		if nLen > 1e-9 {
			nx /= nLen
			ny /= nLen
		}

		result[i] = model.Point2D{
			X: curr.X + nx*dist,
			Y: curr.Y + ny*dist,
		}
	}
	return result
}

// onionSkinActive returns true if onion skinning is enabled and the skin depth is valid.
func (g *Generator) onionSkinActive() bool {
	return g.Settings.OnionSkinEnabled && g.Settings.OnionSkinDepth > 0
}

// applyOnionSkin adjusts the depth for the final pass if onion skinning is active.
// It returns the adjusted depth and whether the onion skin was applied.
func (g *Generator) applyOnionSkin(depth float64, isFinalPass bool) (float64, bool) {
	if !isFinalPass || !g.onionSkinActive() {
		return depth, false
	}
	skinDepth := depth - g.Settings.OnionSkinDepth
	if skinDepth < 0 {
		skinDepth = 0
	}
	return skinDepth, true
}

func (g *Generator) writeRectPart(b *strings.Builder, p model.Placement, partNum int) {
	toolR := g.Settings.ToolDiameter / 2.0

	// The part rectangle in stock coordinates
	pw := p.PlacedWidth()
	ph := p.PlacedHeight()

	// Offset for tool radius (cut outside the part perimeter)
	x0 := p.X - toolR
	y0 := p.Y - toolR
	x1 := p.X + pw + toolR
	y1 := p.Y + ph + toolR

	b.WriteString(g.comment(fmt.Sprintf("--- Part %d: %s (%.1f x %.1f)%s ---",
		partNum, p.Part.Label, p.Part.Width, p.Part.Height,
		rotatedStr(p.Rotated))))

	numPasses := int(math.Ceil(g.Settings.CutDepth / g.Settings.PassDepth))

	// Generate tabs info
	tabs := g.calculateTabs(p)

	hasLeadIn := g.Settings.LeadInRadius > 0
	hasLeadOut := g.Settings.LeadOutRadius > 0

	for pass := 1; pass <= numPasses; pass++ {
		depth := float64(pass) * g.Settings.PassDepth
		if depth > g.Settings.CutDepth {
			depth = g.Settings.CutDepth
		}
		isFinalPass := pass == numPasses

		// Apply onion skin on final pass
		effectiveDepth, skinApplied := g.applyOnionSkin(depth, isFinalPass)
		if skinApplied {
			b.WriteString(g.comment(fmt.Sprintf("Onion skin: leaving %.2fmm skin", g.Settings.OnionSkinDepth)))
		}

		b.WriteString(g.comment(fmt.Sprintf("Pass %d/%d, depth=%.2fmm", pass, numPasses, effectiveDepth)))

		if hasLeadIn {
			// Lead-in arc: rapid to arc start, plunge, then arc onto the perimeter
			g.writeLeadIn(b, x0, y0, effectiveDepth)
		} else {
			// Rapid to start (top-left corner, slightly outside)
			b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove, g.format(x0), g.format(y0)))
			g.writePlunge(b, x0, y0, effectiveDepth)
		}

		// Cut rectangle perimeter (clockwise for climb milling)
		if isFinalPass && g.Settings.PartTabsPerSide > 0 {
			g.writePerimeterWithTabs(b, x0, y0, x1, y1, effectiveDepth, tabs)
		} else {
			g.writePerimeter(b, x0, y0, x1, y1)
		}

		if hasLeadOut {
			// Lead-out arc: arc away from perimeter, then retract
			g.writeLeadOut(b, x0, y0)
		}

		// Retract between passes
		b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.RapidMove, g.format(g.Settings.SafeZ)))
	}

	// Onion skin cleanup pass: cut through the remaining skin at full depth
	if g.onionSkinActive() && g.Settings.OnionSkinCleanup {
		fullDepth := g.Settings.CutDepth
		b.WriteString(g.comment("Onion skin cleanup pass"))
		b.WriteString(g.comment(fmt.Sprintf("Cleanup depth=%.2fmm (removing %.2fmm skin)",
			fullDepth, g.Settings.OnionSkinDepth)))

		if hasLeadIn {
			g.writeLeadIn(b, x0, y0, fullDepth)
		} else {
			b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove, g.format(x0), g.format(y0)))
			g.writePlunge(b, x0, y0, fullDepth)
		}

		g.writePerimeter(b, x0, y0, x1, y1)

		if hasLeadOut {
			g.writeLeadOut(b, x0, y0)
		}
		b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.RapidMove, g.format(g.Settings.SafeZ)))
	}

	b.WriteString("\n")
}

// writeLeadIn generates an arc approach to the perimeter start point (x0, y0).
// The arc starts from an offset position and curves onto the cut path, providing
// a smooth entry that reduces tool deflection and improves surface finish.
//
// For climb milling (clockwise perimeter), the perimeter starts at (x0,y0) and
// moves right along the bottom edge. The lead-in arc approaches from below-left,
// curving up to meet the start point. We use G3 (counter-clockwise arc) so the
// tool sweeps into the cut direction smoothly.
//
// For conventional milling (counter-clockwise perimeter), we use G2 (clockwise arc).
func (g *Generator) writeLeadIn(b *strings.Builder, x0, y0, depth float64) {
	r := g.Settings.LeadInRadius
	angle := g.Settings.LeadInAngle * math.Pi / 180.0

	// Arc center is at the perimeter start point offset inward.
	// For a clockwise (climb) perimeter starting at (x0,y0) moving right,
	// the arc center is directly below the start point (negative Y direction).
	// The arc start is offset from the center by the radius at the approach angle.
	centerX := x0
	centerY := y0 - r

	// Arc start position: offset from center by radius at the approach angle
	// Angle is measured from the line connecting center to the perimeter point.
	arcStartX := centerX - r*math.Sin(angle)
	arcStartY := centerY + r - r*math.Cos(angle)

	// I, J are relative offsets from arc start to arc center
	iOffset := centerX - arcStartX
	jOffset := centerY - arcStartY

	b.WriteString(g.comment("Lead-in arc"))
	// Rapid to arc start position
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove, g.format(arcStartX), g.format(arcStartY)))
	// Plunge to cut depth
	b.WriteString(fmt.Sprintf("%s Z%s F%s\n", g.profile.FeedMove, g.format(-depth), g.format(g.Settings.PlungeRate)))

	// Arc to perimeter start point
	arcCmd := "G3" // Counter-clockwise for climb milling lead-in
	if !g.Settings.UseClimb {
		arcCmd = "G2" // Clockwise for conventional milling lead-in
	}
	b.WriteString(fmt.Sprintf("%s X%s Y%s I%s J%s F%s\n",
		arcCmd, g.format(x0), g.format(y0),
		g.format(iOffset), g.format(jOffset),
		g.format(g.Settings.FeedRate)))
}

// writeLeadOut generates an arc exit from the perimeter end point (x0, y0).
// After the perimeter cut returns to the start point, the lead-out arc continues
// the motion away from the cut, providing a smooth exit that prevents dwell marks.
//
// The arc mirrors the lead-in geometry, curving away from the perimeter.
func (g *Generator) writeLeadOut(b *strings.Builder, x0, y0 float64) {
	r := g.Settings.LeadOutRadius
	angle := g.Settings.LeadInAngle * math.Pi / 180.0

	// Arc center is at the perimeter end point (same as start) offset inward.
	centerX := x0
	centerY := y0 - r

	// Arc end position: mirror of lead-in start
	arcEndX := centerX - r*math.Sin(angle)
	arcEndY := centerY + r - r*math.Cos(angle)

	// I, J are relative offsets from current position (x0, y0) to arc center
	iOffset := centerX - x0
	jOffset := centerY - y0

	b.WriteString(g.comment("Lead-out arc"))
	arcCmd := "G3" // Counter-clockwise for climb milling lead-out
	if !g.Settings.UseClimb {
		arcCmd = "G2" // Clockwise for conventional milling lead-out
	}
	b.WriteString(fmt.Sprintf("%s X%s Y%s I%s J%s F%s\n",
		arcCmd, g.format(arcEndX), g.format(arcEndY),
		g.format(iOffset), g.format(jOffset),
		g.format(g.Settings.FeedRate)))
}

func (g *Generator) writePerimeter(b *strings.Builder, x0, y0, x1, y1 float64) {
	p := g.profile
	toolR := g.Settings.ToolDiameter / 2.0

	// Corner points in clockwise order: bottom-left, bottom-right, top-right, top-left
	corners := [4][2]float64{
		{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1},
	}
	// Previous corner directions (coming from) - used for T-bone calculation
	// Corner 0 (x0,y0): coming from left side (x0,y1)->(x0,y0), going to bottom (x0,y0)->(x1,y0)
	// Corner 1 (x1,y0): coming from bottom, going to right side
	// Corner 2 (x1,y1): coming from right side, going to top
	// Corner 3 (x0,y1): coming from top, going to left side

	b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", p.FeedMove, g.format(x1), g.format(y0), g.format(g.Settings.FeedRate)))
	g.writeCornerOvercut(b, corners[1][0], corners[1][1], toolR, 1)
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", p.FeedMove, g.format(x1), g.format(y1)))
	g.writeCornerOvercut(b, corners[2][0], corners[2][1], toolR, 2)
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", p.FeedMove, g.format(x0), g.format(y1)))
	g.writeCornerOvercut(b, corners[3][0], corners[3][1], toolR, 3)
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", p.FeedMove, g.format(x0), g.format(y0)))
	g.writeCornerOvercut(b, corners[0][0], corners[0][1], toolR, 0)
}

// writeCornerOvercut generates a corner relief cut at the given corner position.
// For dogbone: cuts a small circle at 45 degrees into the corner diagonal.
// For T-bone: cuts perpendicular to the longer adjacent edge.
// The cornerIndex (0-3) indicates which corner of the rectangle (CW from bottom-left):
//
//	0 = bottom-left, 1 = bottom-right, 2 = top-right, 3 = top-left
func (g *Generator) writeCornerOvercut(b *strings.Builder, cx, cy, toolR float64, cornerIndex int) {
	overcutType := g.Settings.CornerOvercut
	if overcutType == model.CornerOvercutNone || overcutType == "" {
		return
	}

	// The overcut distance is the tool radius - this extends the cut into the
	// corner so that after the tool passes, the corner is actually square.
	dist := toolR
	sqrt2inv := 1.0 / math.Sqrt(2.0)

	var dx, dy float64

	switch overcutType {
	case model.CornerOvercutDogbone:
		// Dogbone: cut diagonally into the corner at 45 degrees
		switch cornerIndex {
		case 0: // bottom-left corner - overcut toward bottom-left
			dx = -dist * sqrt2inv
			dy = -dist * sqrt2inv
		case 1: // bottom-right corner - overcut toward bottom-right
			dx = dist * sqrt2inv
			dy = -dist * sqrt2inv
		case 2: // top-right corner - overcut toward top-right
			dx = dist * sqrt2inv
			dy = dist * sqrt2inv
		case 3: // top-left corner - overcut toward top-left
			dx = -dist * sqrt2inv
			dy = dist * sqrt2inv
		}
	case model.CornerOvercutTbone:
		// T-bone: cut perpendicular to the longer edge. For a rectangle perimeter
		// traversed clockwise, the T-bone goes along the axis of the edge
		// we just arrived from. Bottom/top edges are horizontal, left/right vertical.
		switch cornerIndex {
		case 0: // bottom-left: arrived from left edge (vertical), overcut down
			dx = 0
			dy = -dist
		case 1: // bottom-right: arrived from bottom edge (horizontal), overcut right
			dx = dist
			dy = 0
		case 2: // top-right: arrived from right edge (vertical), overcut up
			dx = 0
			dy = dist
		case 3: // top-left: arrived from top edge (horizontal), overcut left
			dx = -dist
			dy = 0
		}
	}

	// Move into the overcut position and back
	b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove,
		g.format(cx+dx), g.format(cy+dy), g.format(g.Settings.FeedRate)))
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.FeedMove,
		g.format(cx), g.format(cy)))
}

// comment wraps text in the profile's comment syntax.
func (g *Generator) comment(text string) string {
	return g.profile.CommentPrefix + " " + text + g.profile.CommentSuffix + "\n"
}

// format formats a coordinate according to the profile's decimal places.
func (g *Generator) format(v float64) string {
	format := fmt.Sprintf("%%.%df", g.profile.DecimalPlaces)
	return fmt.Sprintf(format, v)
}

// Tab represents a holding tab position along the perimeter.
type Tab struct {
	side     int     // 0=bottom, 1=right, 2=top, 3=left
	startPos float64 // distance along that side
}

func (g *Generator) calculateTabs(p model.Placement) []Tab {
	if g.Settings.PartTabsPerSide <= 0 {
		return nil
	}

	pw := p.PlacedWidth() + g.Settings.ToolDiameter
	ph := p.PlacedHeight() + g.Settings.ToolDiameter

	var tabs []Tab
	for side := 0; side < 4; side++ {
		var length float64
		if side == 0 || side == 2 {
			length = pw
		} else {
			length = ph
		}
		spacing := length / float64(g.Settings.PartTabsPerSide+1)
		for t := 1; t <= g.Settings.PartTabsPerSide; t++ {
			tabs = append(tabs, Tab{
				side:     side,
				startPos: spacing * float64(t),
			})
		}
	}
	return tabs
}

func (g *Generator) writePerimeterWithTabs(b *strings.Builder, x0, y0, x1, y1, depth float64, tabs []Tab) {
	tabDepth := depth - g.Settings.PartTabHeight
	if tabDepth < 0 {
		tabDepth = 0
	}
	tw := g.Settings.PartTabWidth

	// Side 0: bottom (x0,y0) -> (x1,y0)
	g.writeSideWithTabs(b, x0, y0, x1, y0, true, depth, tabDepth, tw, g.tabsForSide(tabs, 0))
	// Side 1: right (x1,y0) -> (x1,y1)
	g.writeSideWithTabs(b, x1, y0, x1, y1, false, depth, tabDepth, tw, g.tabsForSide(tabs, 1))
	// Side 2: top (x1,y1) -> (x0,y1)
	g.writeSideWithTabs(b, x1, y1, x0, y1, true, depth, tabDepth, tw, g.tabsForSide(tabs, 2))
	// Side 3: left (x0,y1) -> (x0,y0)
	g.writeSideWithTabs(b, x0, y1, x0, y0, false, depth, tabDepth, tw, g.tabsForSide(tabs, 3))
}

func (g *Generator) tabsForSide(tabs []Tab, side int) []Tab {
	var result []Tab
	for _, t := range tabs {
		if t.side == side {
			result = append(result, t)
		}
	}
	return result
}

func (g *Generator) writeSideWithTabs(b *strings.Builder, x0, y0, x1, y1 float64, isHoriz bool,
	cutDepth, tabDepth, tabWidth float64, tabs []Tab) {

	if len(tabs) == 0 {
		b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove, g.format(x1), g.format(y1), g.format(g.Settings.FeedRate)))
		return
	}

	dx := x1 - x0
	dy := y1 - y0
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 0.001 {
		return
	}
	nx := dx / length
	ny := dy / length

	// Walk along the side, raising Z for tabs
	cursor := 0.0
	for _, tab := range tabs {
		tabStart := tab.startPos - tabWidth/2
		tabEnd := tab.startPos + tabWidth/2

		// Cut to tab start
		if tabStart > cursor {
			px := x0 + nx*tabStart
			py := y0 + ny*tabStart
			b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove, g.format(px), g.format(py), g.format(g.Settings.FeedRate)))
		}

		// Raise to tab height
		b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.FeedMove, g.format(-tabDepth)))
		// Traverse tab
		px := x0 + nx*tabEnd
		py := y0 + ny*tabEnd
		b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.FeedMove, g.format(px), g.format(py)))
		// Plunge back down
		b.WriteString(fmt.Sprintf("%s Z%s\n", g.profile.FeedMove, g.format(-cutDepth)))

		cursor = tabEnd
	}

	// Finish to end of side
	b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", g.profile.FeedMove, g.format(x1), g.format(y1), g.format(g.Settings.FeedRate)))
}

func rotatedStr(r bool) string {
	if r {
		return " [rotated]"
	}
	return ""
}

// normalize returns a unit vector in the given direction.
func normalize(x, y float64) (float64, float64) {
	length := math.Sqrt(x*x + y*y)
	if length < 1e-9 {
		return 0, 0
	}
	return x / length, y / length
}
