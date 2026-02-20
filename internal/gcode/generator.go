package gcode

import (
	"fmt"
	"math"
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

	for i, placement := range sheet.Placements {
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

		b.WriteString(g.comment(fmt.Sprintf("Pass %d/%d, depth=%.2fmm", pass, numPasses, depth)))

		// Rapid to first point
		b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove,
			g.format(translated[0].X), g.format(translated[0].Y)))
		// Plunge
		b.WriteString(fmt.Sprintf("%s Z%s F%s\n", g.profile.FeedMove,
			g.format(-depth), g.format(g.Settings.PlungeRate)))

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

		b.WriteString(g.comment(fmt.Sprintf("Pass %d/%d, depth=%.2fmm", pass, numPasses, depth)))

		if hasLeadIn {
			// Lead-in arc: rapid to arc start, plunge, then arc onto the perimeter
			g.writeLeadIn(b, x0, y0, depth)
		} else {
			// Rapid to start (top-left corner, slightly outside)
			b.WriteString(fmt.Sprintf("%s X%s Y%s\n", g.profile.RapidMove, g.format(x0), g.format(y0)))
			b.WriteString(fmt.Sprintf("%s Z%s F%s ; Plunge\n", g.profile.FeedMove, g.format(-depth), g.format(g.Settings.PlungeRate)))
		}

		// Cut rectangle perimeter (clockwise for climb milling)
		if isFinalPass && g.Settings.PartTabsPerSide > 0 {
			g.writePerimeterWithTabs(b, x0, y0, x1, y1, depth, tabs)
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
	b.WriteString(fmt.Sprintf("%s X%s Y%s F%s\n", p.FeedMove, g.format(x1), g.format(y0), g.format(g.Settings.FeedRate)))
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", p.FeedMove, g.format(x1), g.format(y1)))
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", p.FeedMove, g.format(x0), g.format(y1)))
	b.WriteString(fmt.Sprintf("%s X%s Y%s\n", p.FeedMove, g.format(x0), g.format(y0)))
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
