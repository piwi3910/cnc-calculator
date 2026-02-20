package model

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

// PlungeType represents the plunge entry strategy for CNC operations.
type PlungeType string

const (
	PlungeDirect PlungeType = "direct" // Straight plunge into material
	PlungeRamp   PlungeType = "ramp"   // Ramped entry at an angle
	PlungeHelix  PlungeType = "helix"  // Helical plunge entry
)

// PlungeTypeOptions returns the available plunge type choices for UI display.
func PlungeTypeOptions() []string {
	return []string{"Direct", "Ramp", "Helix"}
}

// PlungeTypeFromString converts a display string to a PlungeType.
func PlungeTypeFromString(s string) PlungeType {
	switch s {
	case "Ramp":
		return PlungeRamp
	case "Helix":
		return PlungeHelix
	default:
		return PlungeDirect
	}
}

// String returns the display name for a PlungeType.
func (p PlungeType) String() string {
	switch p {
	case PlungeRamp:
		return "Ramp"
	case PlungeHelix:
		return "Helix"
	default:
		return "Direct"
	}
}

// CornerOvercut represents the corner relief cut type for CNC routing.
type CornerOvercut string

const (
	CornerOvercutNone    CornerOvercut = "none"    // No corner overcut
	CornerOvercutDogbone CornerOvercut = "dogbone" // Circular overcut into diagonal
	CornerOvercutTbone   CornerOvercut = "tbone"   // Perpendicular overcut along longest edge
)

// CornerOvercutOptions returns available corner overcut choices for UI display.
func CornerOvercutOptions() []string {
	return []string{"None", "Dogbone", "T-Bone"}
}

// CornerOvercutFromString converts a display string to a CornerOvercut.
func CornerOvercutFromString(s string) CornerOvercut {
	switch s {
	case "Dogbone":
		return CornerOvercutDogbone
	case "T-Bone":
		return CornerOvercutTbone
	default:
		return CornerOvercutNone
	}
}

// String returns the display name for a CornerOvercut.
func (c CornerOvercut) String() string {
	switch c {
	case CornerOvercutDogbone:
		return "Dogbone"
	case CornerOvercutTbone:
		return "T-Bone"
	default:
		return "None"
	}
}

// Grain represents the grain direction constraint for a part.
type Grain int

const (
	GrainNone       Grain = iota // No grain constraint, can rotate freely
	GrainHorizontal              // Grain runs along the width
	GrainVertical                // Grain runs along the height
)

func (g Grain) String() string {
	switch g {
	case GrainHorizontal:
		return "Horizontal"
	case GrainVertical:
		return "Vertical"
	default:
		return "None"
	}
}

// Point2D represents a 2D coordinate in mm.
type Point2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Outline represents a closed polygon as a sequence of 2D points.
// The outline is implicitly closed: the last point connects back to the first.
type Outline []Point2D

// BoundingBox returns the min and max corners of the outline.
func (o Outline) BoundingBox() (min, max Point2D) {
	if len(o) == 0 {
		return Point2D{}, Point2D{}
	}
	min = Point2D{X: o[0].X, Y: o[0].Y}
	max = Point2D{X: o[0].X, Y: o[0].Y}
	for _, p := range o[1:] {
		if p.X < min.X {
			min.X = p.X
		}
		if p.Y < min.Y {
			min.Y = p.Y
		}
		if p.X > max.X {
			max.X = p.X
		}
		if p.Y > max.Y {
			max.Y = p.Y
		}
	}
	return min, max
}

// Translate shifts all points by dx, dy.
func (o Outline) Translate(dx, dy float64) Outline {
	result := make(Outline, len(o))
	for i, p := range o {
		result[i] = Point2D{X: p.X + dx, Y: p.Y + dy}
	}
	return result
}

// Perimeter returns the total perimeter length of the outline polygon.
func (o Outline) Perimeter() float64 {
	if len(o) < 2 {
		return 0
	}
	var total float64
	for i := 0; i < len(o); i++ {
		j := (i + 1) % len(o)
		dx := o[j].X - o[i].X
		dy := o[j].Y - o[i].Y
		total += math.Sqrt(dx*dx + dy*dy)
	}
	return total
}

// Rotate returns a new outline rotated by the given angle (in radians) around
// the centroid. The result is then translated so the bounding box origin is at (0,0).
func (o Outline) Rotate(radians float64) Outline {
	if len(o) == 0 || radians == 0 {
		return o
	}

	// Find centroid
	var cx, cy float64
	for _, p := range o {
		cx += p.X
		cy += p.Y
	}
	cx /= float64(len(o))
	cy /= float64(len(o))

	cos := math.Cos(radians)
	sin := math.Sin(radians)

	rotated := make(Outline, len(o))
	for i, p := range o {
		dx := p.X - cx
		dy := p.Y - cy
		rotated[i] = Point2D{
			X: cx + dx*cos - dy*sin,
			Y: cy + dx*sin + dy*cos,
		}
	}

	// Translate so bounding box starts at (0,0)
	min, _ := rotated.BoundingBox()
	for i := range rotated {
		rotated[i].X -= min.X
		rotated[i].Y -= min.Y
	}

	return rotated
}

// Area returns the area of the polygon using the shoelace formula.
// Returns the absolute area (always positive).
func (o Outline) Area() float64 {
	n := len(o)
	if n < 3 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		sum += o[i].X*o[j].Y - o[j].X*o[i].Y
	}
	return math.Abs(sum) / 2.0
}

// ContainsPoint returns true if point (px,py) is inside the polygon
// using the ray-casting algorithm.
func (o Outline) ContainsPoint(px, py float64) bool {
	n := len(o)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		yi := o[i].Y
		yj := o[j].Y
		if (yi > py) != (yj > py) {
			xi := o[i].X
			xj := o[j].X
			xIntersect := xi + (py-yi)/(yj-yi)*(xj-xi)
			if px < xIntersect {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

// segmentsIntersect returns true if line segment (a1,a2)-(b1,b2) intersects (c1,c2)-(d1,d2).
func segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	d1 := crossProduct(cx, cy, dx, dy, ax, ay)
	d2 := crossProduct(cx, cy, dx, dy, bx, by)
	d3 := crossProduct(ax, ay, bx, by, cx, cy)
	d4 := crossProduct(ax, ay, bx, by, dx, dy)

	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}

	// Check collinear cases
	if d1 == 0 && onSegment(cx, cy, dx, dy, ax, ay) {
		return true
	}
	if d2 == 0 && onSegment(cx, cy, dx, dy, bx, by) {
		return true
	}
	if d3 == 0 && onSegment(ax, ay, bx, by, cx, cy) {
		return true
	}
	if d4 == 0 && onSegment(ax, ay, bx, by, dx, dy) {
		return true
	}
	return false
}

func crossProduct(ox, oy, ax, ay, bx, by float64) float64 {
	return (ax-ox)*(by-oy) - (ay-oy)*(bx-ox)
}

func onSegment(px, py, qx, qy, rx, ry float64) bool {
	return math.Min(px, qx) <= rx && rx <= math.Max(px, qx) &&
		math.Min(py, qy) <= ry && ry <= math.Max(py, qy)
}

// OutlinesOverlap checks whether two outlines (translated to their placement
// positions) overlap. It uses edge intersection and point-in-polygon tests.
func OutlinesOverlap(a Outline, ax, ay float64, b Outline, bx, by float64) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}

	// Translate outlines to absolute positions
	absA := a.Translate(ax, ay)
	absB := b.Translate(bx, by)

	// Check if any edges intersect
	na := len(absA)
	nb := len(absB)
	for i := 0; i < na; i++ {
		ni := (i + 1) % na
		for j := 0; j < nb; j++ {
			nj := (j + 1) % nb
			if segmentsIntersect(
				absA[i].X, absA[i].Y, absA[ni].X, absA[ni].Y,
				absB[j].X, absB[j].Y, absB[nj].X, absB[nj].Y,
			) {
				return true
			}
		}
	}

	// Check containment: a point of A inside B or vice versa
	if absB.ContainsPoint(absA[0].X, absA[0].Y) {
		return true
	}
	if absA.ContainsPoint(absB[0].X, absB[0].Y) {
		return true
	}

	return false
}

// EdgeBanding flags which edges of a part need edge banding applied.
type EdgeBanding struct {
	Top    bool `json:"top"`    // Banding on the top edge (width side)
	Bottom bool `json:"bottom"` // Banding on the bottom edge (width side)
	Left   bool `json:"left"`   // Banding on the left edge (height side)
	Right  bool `json:"right"`  // Banding on the right edge (height side)
}

// HasAny returns true if any edge needs banding.
func (eb EdgeBanding) HasAny() bool {
	return eb.Top || eb.Bottom || eb.Left || eb.Right
}

// EdgeCount returns the number of edges that need banding.
func (eb EdgeBanding) EdgeCount() int {
	count := 0
	if eb.Top {
		count++
	}
	if eb.Bottom {
		count++
	}
	if eb.Left {
		count++
	}
	if eb.Right {
		count++
	}
	return count
}

// LinearLength returns the total linear length of edge banding needed for one piece
// of the given dimensions (in mm).
func (eb EdgeBanding) LinearLength(width, height float64) float64 {
	var total float64
	if eb.Top {
		total += width
	}
	if eb.Bottom {
		total += width
	}
	if eb.Left {
		total += height
	}
	if eb.Right {
		total += height
	}
	return total
}

// String returns a human-readable summary of banded edges.
func (eb EdgeBanding) String() string {
	if !eb.HasAny() {
		return "None"
	}
	var edges []string
	if eb.Top {
		edges = append(edges, "T")
	}
	if eb.Bottom {
		edges = append(edges, "B")
	}
	if eb.Left {
		edges = append(edges, "L")
	}
	if eb.Right {
		edges = append(edges, "R")
	}
	result := ""
	for i, e := range edges {
		if i > 0 {
			result += "+"
		}
		result += e
	}
	return result
}

// Part represents a required piece to be cut.
type Part struct {
	ID          string      `json:"id"`
	Label       string      `json:"label"`
	Width       float64     `json:"width"`  // mm (bounding box width for non-rectangular parts)
	Height      float64     `json:"height"` // mm (bounding box height for non-rectangular parts)
	Quantity    int         `json:"quantity"`
	Grain       Grain       `json:"grain"`
	Material    string      `json:"material,omitempty"`     // Material type (e.g., "Plywood", "MDF"); empty means unspecified
	Outline     Outline     `json:"outline,omitempty"`      // Non-rectangular part outline; nil for rectangular parts
	Cutouts     []Outline   `json:"cutouts,omitempty"`      // Interior cutout holes where smaller parts can be nested
	EdgeBanding EdgeBanding `json:"edge_banding,omitempty"` // Which edges need banding
}

// CutoutBounds returns the bounding rectangles of all cutouts in the part.
// Each returned rect is relative to the part origin (0,0).
func (p Part) CutoutBounds() []CutoutRect {
	var rects []CutoutRect
	for _, c := range p.Cutouts {
		if len(c) < 3 {
			continue // Not a valid polygon
		}
		min, max := c.BoundingBox()
		w := max.X - min.X
		h := max.Y - min.Y
		if w > 0 && h > 0 {
			rects = append(rects, CutoutRect{
				X:      min.X,
				Y:      min.Y,
				Width:  w,
				Height: h,
			})
		}
	}
	return rects
}

// CutoutRect represents the bounding rectangle of an interior cutout,
// relative to the part origin.
type CutoutRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func NewPart(label string, w, h float64, qty int) Part {
	return Part{
		ID:       uuid.New().String()[:8],
		Label:    label,
		Width:    w,
		Height:   h,
		Quantity: qty,
		Grain:    GrainNone,
	}
}

// StockSheet represents an available sheet of material to cut from.
type StockSheet struct {
	ID            string         `json:"id"`
	Label         string         `json:"label"`
	Width         float64        `json:"width"`  // mm
	Height        float64        `json:"height"` // mm
	Quantity      int            `json:"quantity"`
	Grain         Grain          `json:"grain"`              // Sheet grain direction (None, Horizontal, Vertical)
	Material      string         `json:"material,omitempty"` // Material type (e.g., "Plywood", "MDF"); empty means unspecified
	Tabs          StockTabConfig `json:"tabs"`               // Override default tab config for this sheet
	PricePerSheet float64        `json:"price_per_sheet"`    // Cost per sheet in user's currency (0 = not set)
}

func NewStockSheet(label string, w, h float64, qty int) StockSheet {
	return StockSheet{
		ID:       uuid.New().String()[:8],
		Label:    label,
		Width:    w,
		Height:   h,
		Quantity: qty,
		Grain:    GrainNone,
		Tabs:     StockTabConfig{Enabled: false}, // Use defaults by default
	}
}

// CanPlaceWithGrain checks whether a part with the given grain constraint can be
// placed on a stock sheet with the given grain, optionally rotated 90 degrees.
// Returns (canPlaceNormal, canPlaceRotated).
//
// Rules:
//   - If the part grain is None, it can always be placed in either orientation.
//   - If the stock grain is None, any part grain is acceptable; rotation is blocked
//     only because the part has grain (rotating would flip the grain direction).
//   - If both part and stock have a grain, the part grain must match the stock grain.
//     Rotation would flip the grain so it is not allowed when both have grain.
func CanPlaceWithGrain(partGrain, stockGrain Grain) (canNormal, canRotated bool) {
	if partGrain == GrainNone {
		return true, true
	}
	if stockGrain == GrainNone {
		return true, false
	}
	if partGrain == stockGrain {
		return true, false
	}
	return false, false
}

// Algorithm represents the optimizer algorithm to use.
type Algorithm string

const (
	AlgorithmGuillotine Algorithm = "guillotine" // Greedy guillotine best-area-fit (fast)
	AlgorithmGenetic    Algorithm = "genetic"    // Genetic algorithm meta-heuristic (slower, often better)
)

// CutSettings holds optimizer and CNC configuration.
type CutSettings struct {
	// Optimizer settings
	Algorithm      Algorithm `json:"algorithm"`       // Optimizer algorithm: "guillotine" or "genetic"
	KerfWidth      float64   `json:"kerf_width"`      // Blade/bit width in mm
	EdgeTrim       float64   `json:"edge_trim"`       // Trim around sheet edges in mm
	GuillotineOnly bool      `json:"guillotine_only"` // Restrict to guillotine cuts

	// CNC / GCode settings
	ToolDiameter float64 `json:"tool_diameter"` // End mill diameter in mm
	FeedRate     float64 `json:"feed_rate"`     // Cutting feed rate mm/min
	PlungeRate   float64 `json:"plunge_rate"`   // Plunge feed rate mm/min
	SpindleSpeed int     `json:"spindle_speed"` // RPM
	SafeZ        float64 `json:"safe_z"`        // Safe retract height mm
	CutDepth     float64 `json:"cut_depth"`     // Total material thickness mm
	PassDepth    float64 `json:"pass_depth"`    // Depth per pass mm

	// Part holding tabs (for keeping parts connected during cut)
	PartTabWidth    float64 `json:"part_tab_width"`     // Part tab width mm
	PartTabHeight   float64 `json:"part_tab_height"`    // Part tab height mm
	PartTabsPerSide int     `json:"part_tabs_per_side"` // Number of tabs per part side
	UseClimb        bool    `json:"use_climb"`          // Climb vs conventional milling

	// Lead-in/lead-out arcs (for smoother CNC entry and exit)
	LeadInRadius  float64 `json:"lead_in_radius"`  // Arc radius for approach to cut (0 = disabled)
	LeadOutRadius float64 `json:"lead_out_radius"` // Arc radius for exit from cut (0 = disabled)
	LeadInAngle   float64 `json:"lead_in_angle"`   // Approach angle in degrees (default 90)

	// Stock sheet holding tabs (for securing sheet to CNC bed)
	StockTabs StockTabConfig `json:"stock_tabs"` // Stock sheet tab configuration

	// GCode post-processor profile
	GCodeProfile string `json:"gcode_profile"` // Name of the GCode profile to use

	// Toolpath ordering (minimize rapid travel distance)
	OptimizeToolpath bool `json:"optimize_toolpath"` // Enable nearest-neighbor toolpath ordering

	// Plunge entry strategy
	PlungeType      PlungeType `json:"plunge_type"`       // Plunge strategy: direct, ramp, or helix
	RampAngle       float64    `json:"ramp_angle"`        // Ramp entry angle in degrees (for ramp plunge)
	HelixDiameter   float64    `json:"helix_diameter"`    // Helix diameter in mm (for helix plunge)
	HelixRevPercent float64    `json:"helix_rev_percent"` // Helix depth per revolution as % of pass depth

	// Corner overcuts for interior corners
	CornerOvercut CornerOvercut `json:"corner_overcut"` // Corner relief type: none, dogbone, or tbone

	// Onion skinning (leave thin layer on final pass to prevent part movement)
	OnionSkinEnabled bool    `json:"onion_skin_enabled"` // Enable onion skin on final pass
	OnionSkinDepth   float64 `json:"onion_skin_depth"`   // Thickness of skin to leave (mm)
	OnionSkinCleanup bool    `json:"onion_skin_cleanup"` // Generate a separate cleanup pass to remove the skin

	// Structural integrity cut ordering (interior cuts first, perimeter last)
	StructuralOrdering bool `json:"structural_ordering"` // Order cuts from center outward for structural integrity

	// Non-rectangular nesting: number of rotation angles to try for outline parts
	// 2 = 0° and 90° (default), 4 = every 45°, 8 = every 22.5°, etc.
	NestingRotations int `json:"nesting_rotations"` // Number of rotation angles for outline parts (default 2)

	// Fixture/clamp exclusion zones
	ClampZones []ClampZone `json:"clamp_zones,omitempty"` // Clamp/fixture zones to exclude from optimization

	// Dust shoe collision detection
	DustShoeEnabled   bool    `json:"dust_shoe_enabled"`   // Enable dust shoe collision checking
	DustShoeWidth     float64 `json:"dust_shoe_width"`     // Dust shoe diameter/width in mm
	DustShoeClearance float64 `json:"dust_shoe_clearance"` // Minimum clearance between dust shoe edge and clamp (mm)

	// Multi-objective optimization weights (all values 0-1, normalized internally)
	OptimizeWeights OptimizeWeights `json:"optimize_weights"` // Weights for multi-objective fitness
}

// OptimizeWeights controls the priority of different optimization objectives.
// Each weight is in the range [0,1]. They are normalized internally so their
// relative proportions determine priority. A weight of 0 disables that objective.
type OptimizeWeights struct {
	MinimizeWaste   float64 `json:"minimize_waste"`    // Weight for minimizing material waste (default 1.0)
	MinimizeSheets  float64 `json:"minimize_sheets"`   // Weight for minimizing number of sheets used (default 0.5)
	MinimizeCutLen  float64 `json:"minimize_cut_len"`  // Weight for minimizing total cut length (default 0.0)
	MinimizeJobTime float64 `json:"minimize_job_time"` // Weight for minimizing estimated job time (default 0.0)
}

// DefaultOptimizeWeights returns the default optimization weights.
func DefaultOptimizeWeights() OptimizeWeights {
	return OptimizeWeights{
		MinimizeWaste:   1.0,
		MinimizeSheets:  0.5,
		MinimizeCutLen:  0.0,
		MinimizeJobTime: 0.0,
	}
}

// Normalize returns a copy of the weights with values normalized to sum to 1.
// If all weights are zero, returns equal weights for waste and sheets.
func (w OptimizeWeights) Normalize() OptimizeWeights {
	total := w.MinimizeWaste + w.MinimizeSheets + w.MinimizeCutLen + w.MinimizeJobTime
	if total <= 0 {
		return OptimizeWeights{MinimizeWaste: 0.5, MinimizeSheets: 0.5}
	}
	return OptimizeWeights{
		MinimizeWaste:   w.MinimizeWaste / total,
		MinimizeSheets:  w.MinimizeSheets / total,
		MinimizeCutLen:  w.MinimizeCutLen / total,
		MinimizeJobTime: w.MinimizeJobTime / total,
	}
}

// StockTabConfig defines holding tabs for the stock sheet edges.
// These keep the sheet secured to the CNC bed while cutting.
type StockTabConfig struct {
	Enabled      bool `json:"enabled"`       // Whether stock tabs are enabled
	AdvancedMode bool `json:"advanced_mode"` // true = custom positions, false = edge padding

	// Simple mode: uniform padding on edges
	TopPadding    float64 `json:"top_padding"`    // mm from top edge to keep as tab
	BottomPadding float64 `json:"bottom_padding"` // mm from bottom edge to keep as tab
	LeftPadding   float64 `json:"left_padding"`   // mm from left edge to keep as tab
	RightPadding  float64 `json:"right_padding"`  // mm from right edge to keep as tab

	// Advanced mode: specific tab zones
	// Each zone is defined by: x, y, width, height (all in mm from stock origin)
	CustomZones []TabZone `json:"custom_zones"`
}

// TabZone defines a rectangular tab zone on the stock sheet.
type TabZone struct {
	X      float64 `json:"x"`      // Distance from left edge (mm)
	Y      float64 `json:"y"`      // Distance from top edge (mm)
	Width  float64 `json:"width"`  // Zone width (mm)
	Height float64 `json:"height"` // Zone height (mm)
}

// ClampZone defines a rectangular exclusion zone on the stock sheet where
// a clamp or fixture is placed. The optimizer will avoid placing parts in
// these zones, and the GCode generator can check for dust shoe collisions.
type ClampZone struct {
	Label   string  `json:"label"`    // Descriptive label (e.g., "Front-left clamp")
	X       float64 `json:"x"`        // Distance from left edge (mm)
	Y       float64 `json:"y"`        // Distance from top edge (mm)
	Width   float64 `json:"width"`    // Zone width (mm)
	Height  float64 `json:"height"`   // Zone height (mm)
	ZHeight float64 `json:"z_height"` // Height above stock surface (mm), used for collision detection
}

// Overlaps returns true if this clamp zone overlaps with the given rectangle.
func (cz ClampZone) Overlaps(x, y, w, h float64) bool {
	return cz.X < x+w && cz.X+cz.Width > x &&
		cz.Y < y+h && cz.Y+cz.Height > y
}

// ToTabZone converts a ClampZone to a TabZone for use in exclusion logic.
func (cz ClampZone) ToTabZone() TabZone {
	return TabZone{
		X:      cz.X,
		Y:      cz.Y,
		Width:  cz.Width,
		Height: cz.Height,
	}
}

// DustShoeCollision describes a potential collision between the dust shoe and a clamp/fixture.
type DustShoeCollision struct {
	SheetIndex  int     `json:"sheet_index"`   // 0-based index of the sheet
	SheetLabel  string  `json:"sheet_label"`   // Label of the stock sheet
	ClampLabel  string  `json:"clamp_label"`   // Label of the clamp zone
	PartLabel   string  `json:"part_label"`    // Label of the part being cut near the clamp
	PartIndex   int     `json:"part_index"`    // Index of the placement on the sheet
	ToolX       float64 `json:"tool_x"`        // Tool center X position where collision occurs
	ToolY       float64 `json:"tool_y"`        // Tool center Y position where collision occurs
	Distance    float64 `json:"distance"`      // Distance from dust shoe edge to clamp edge (negative = overlap)
	IsDuringCut bool    `json:"is_during_cut"` // true if during cutting move, false if during rapid
}

// GCodeProfile defines a post-processor configuration for different CNC controllers.
type GCodeProfile struct {
	Name        string `json:"name"`        // Profile name
	Description string `json:"description"` // Profile description
	IsBuiltIn   bool   `json:"is_built_in"` // Whether this is a built-in profile (cannot be deleted)
	Units       string `json:"units"`       // "mm" or "inches"

	// Startup codes
	StartCode    []string `json:"start_code"`    // Commands at start of file
	SpindleStart string   `json:"spindle_start"` // Spindle on command (e.g., "M3 S%d")
	SpindleStop  string   `json:"spindle_stop"`  // Spindle off command
	HomeAll      string   `json:"home_all"`      // Home all axes command
	HomeXY       string   `json:"home_xy"`       // Home XY only command

	// Motion settings
	AbsoluteMode string `json:"absolute_mode"` // G90 or equivalent
	FeedMode     string `json:"feed_mode"`     // Feed rate mode
	RapidMove    string `json:"rapid_move"`    // G0 or equivalent
	FeedMove     string `json:"feed_move"`     // G1 or equivalent

	// End codes
	EndCode []string `json:"end_code"` // Commands at end of file

	// Comment style
	CommentPrefix string `json:"comment_prefix"` // Comment start (e.g., ";")
	CommentSuffix string `json:"comment_suffix"` // Comment end (if needed, e.g., ")" for Fanuc)

	// Number formatting
	DecimalPlaces int  `json:"decimal_places"` // Number of decimal places for coordinates
	LeadingZeros  bool `json:"leading_zeros"`  // Whether to pad with leading zeros
}

// Built-in GCode profiles
var GCodeProfiles = []GCodeProfile{
	{
		Name:          "Grbl",
		Description:   "Standard Grbl configuration (Arduino CNC shields)",
		IsBuiltIn:     true,
		Units:         "mm",
		StartCode:     []string{"G90", "G21", "G17"},
		SpindleStart:  "M3 S%d",
		SpindleStop:   "M5",
		HomeAll:       "$H",
		HomeXY:        "$H",
		AbsoluteMode:  "G90",
		FeedMode:      "G94",
		RapidMove:     "G0",
		FeedMove:      "G1",
		EndCode:       []string{"G0 Z[SafeZ]", "G0 X0 Y0", "M5", "M2"},
		CommentPrefix: ";",
		CommentSuffix: "",
		DecimalPlaces: 3,
		LeadingZeros:  false,
	},
	{
		Name:          "Mach3",
		Description:   "Mach3 CNC control software",
		IsBuiltIn:     true,
		Units:         "mm",
		StartCode:     []string{"G90", "G21", "G17", "G94"},
		SpindleStart:  "M3 S%d",
		SpindleStop:   "M5",
		HomeAll:       "G28 X0 Y0 Z0",
		HomeXY:        "G28 X0 Y0",
		AbsoluteMode:  "G90",
		FeedMode:      "G94",
		RapidMove:     "G0",
		FeedMove:      "G1",
		EndCode:       []string{"G0 Z[SafeZ]", "G28 X0 Y0", "M5", "M30"},
		CommentPrefix: ";",
		CommentSuffix: "",
		DecimalPlaces: 4,
		LeadingZeros:  false,
	},
	{
		Name:          "LinuxCNC",
		Description:   "LinuxCNC (formerly EMC2)",
		IsBuiltIn:     true,
		Units:         "mm",
		StartCode:     []string{"G90", "G21", "G17", "G94"},
		SpindleStart:  "M3 S%d",
		SpindleStop:   "M5",
		HomeAll:       "G28 X0 Y0 Z0",
		HomeXY:        "G28 X0 Y0",
		AbsoluteMode:  "G90",
		FeedMode:      "G94",
		RapidMove:     "G0",
		FeedMove:      "G1",
		EndCode:       []string{"G0 Z[SafeZ]", "G0 X0 Y0", "M5", "M2"},
		CommentPrefix: ";",
		CommentSuffix: "",
		DecimalPlaces: 4,
		LeadingZeros:  false,
	},
	{
		Name:          "Generic",
		Description:   "Generic standard GCode",
		IsBuiltIn:     true,
		Units:         "mm",
		StartCode:     []string{"G90", "G21"},
		SpindleStart:  "M3 S%d",
		SpindleStop:   "M5",
		HomeAll:       "G28 X0 Y0 Z0",
		HomeXY:        "G28 X0 Y0",
		AbsoluteMode:  "G90",
		FeedMode:      "G94",
		RapidMove:     "G0",
		FeedMove:      "G1",
		EndCode:       []string{"G0 Z[SafeZ]", "G0 X0 Y0", "M5", "M2"},
		CommentPrefix: ";",
		CommentSuffix: "",
		DecimalPlaces: 3,
		LeadingZeros:  false,
	},
}

// CustomProfiles holds user-defined GCode profiles loaded from disk.
var CustomProfiles []GCodeProfile

// AllProfiles returns all profiles: built-in profiles followed by custom profiles.
func AllProfiles() []GCodeProfile {
	all := make([]GCodeProfile, 0, len(GCodeProfiles)+len(CustomProfiles))
	all = append(all, GCodeProfiles...)
	all = append(all, CustomProfiles...)
	return all
}

// GetProfile returns a GCode profile by name, or the Generic profile if not found.
// It searches both built-in and custom profiles.
func GetProfile(name string) GCodeProfile {
	for _, p := range AllProfiles() {
		if p.Name == name {
			return p
		}
	}
	return GCodeProfiles[len(GCodeProfiles)-1] // Return Generic (last one)
}

// GetProfileNames returns a list of all available profile names (built-in + custom).
func GetProfileNames() []string {
	var names []string
	for _, p := range AllProfiles() {
		names = append(names, p.Name)
	}
	return names
}

// AddCustomProfile adds a custom profile. Returns an error if a profile with the same name
// already exists among built-in profiles.
func AddCustomProfile(profile GCodeProfile) error {
	for _, p := range GCodeProfiles {
		if p.Name == profile.Name {
			return fmt.Errorf("cannot add custom profile: name %q conflicts with built-in profile", profile.Name)
		}
	}
	// Replace existing custom profile with same name
	for i, p := range CustomProfiles {
		if p.Name == profile.Name {
			CustomProfiles[i] = profile
			return nil
		}
	}
	CustomProfiles = append(CustomProfiles, profile)
	return nil
}

// RemoveCustomProfile removes a custom profile by name. Returns an error if the profile
// is built-in or does not exist.
func RemoveCustomProfile(name string) error {
	for _, p := range GCodeProfiles {
		if p.Name == name {
			return fmt.Errorf("cannot remove built-in profile %q", name)
		}
	}
	for i, p := range CustomProfiles {
		if p.Name == name {
			CustomProfiles = append(CustomProfiles[:i], CustomProfiles[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("custom profile %q not found", name)
}

// NewCustomProfile creates a new custom profile with sensible defaults based on the Generic profile.
func NewCustomProfile(name string) GCodeProfile {
	generic := GetProfile("Generic")
	generic.Name = name
	generic.Description = "Custom profile"
	generic.IsBuiltIn = false
	return generic
}

func DefaultSettings() CutSettings {
	return CutSettings{
		Algorithm:       AlgorithmGuillotine,
		KerfWidth:       3.2,
		EdgeTrim:        10.0,
		GuillotineOnly:  false,
		ToolDiameter:    6.0,
		FeedRate:        1500.0,
		PlungeRate:      500.0,
		SpindleSpeed:    18000,
		SafeZ:           5.0,
		CutDepth:        18.0,
		PassDepth:       6.0,
		PartTabWidth:    8.0,
		PartTabHeight:   2.0,
		PartTabsPerSide: 0, // Disabled by default
		UseClimb:        true,
		LeadInRadius:    0.0,  // Disabled by default
		LeadOutRadius:   0.0,  // Disabled by default
		LeadInAngle:     90.0, // 90 degree approach angle
		StockTabs: StockTabConfig{
			Enabled:       true, // Enabled by default
			AdvancedMode:  false,
			TopPadding:    25.0,
			BottomPadding: 25.0,
			LeftPadding:   25.0,
			RightPadding:  25.0,
			CustomZones:   nil,
		},
		GCodeProfile:     "Generic", // Default GCode profile
		OptimizeToolpath: false,     // Disabled by default

		PlungeType:        PlungeDirect,      // Direct plunge by default
		RampAngle:         3.0,               // 3 degree ramp angle
		HelixDiameter:     5.0,               // 5mm helix diameter
		HelixRevPercent:   50.0,              // 50% of pass depth per revolution
		CornerOvercut:     CornerOvercutNone, // No corner overcuts by default
		OnionSkinEnabled:  false,             // Onion skinning disabled by default
		OnionSkinDepth:    0.2,               // 0.2mm thin skin
		OnionSkinCleanup:  false,             // No cleanup pass by default
		DustShoeEnabled:   false,             // Dust shoe collision detection disabled by default
		DustShoeWidth:     80.0,              // 80mm default dust shoe diameter
		DustShoeClearance: 5.0,               // 5mm minimum clearance
		OptimizeWeights:   DefaultOptimizeWeights(),
		NestingRotations:  2, // Default: 0° and 90° (standard rectangular behavior)
	}
}

// Placement represents a single part placed on a stock sheet.
type Placement struct {
	Part    Part    `json:"part"`
	X       float64 `json:"x"`       // Position from left edge (mm)
	Y       float64 `json:"y"`       // Position from top edge (mm)
	Rotated bool    `json:"rotated"` // Whether part was rotated 90°
}

// PlacedWidth returns the effective width considering rotation.
func (p Placement) PlacedWidth() float64 {
	if p.Rotated {
		return p.Part.Height
	}
	return p.Part.Width
}

// PlacedHeight returns the effective height considering rotation.
func (p Placement) PlacedHeight() float64 {
	if p.Rotated {
		return p.Part.Width
	}
	return p.Part.Height
}

// SheetResult represents one stock sheet with its placed parts.
type SheetResult struct {
	Stock      StockSheet  `json:"stock"`
	Placements []Placement `json:"placements"`
}

// UsedArea returns the total area used by placed parts.
func (sr SheetResult) UsedArea() float64 {
	var total float64
	for _, p := range sr.Placements {
		total += p.PlacedWidth() * p.PlacedHeight()
	}
	return total
}

// TotalArea returns the stock sheet area.
func (sr SheetResult) TotalArea() float64 {
	return sr.Stock.Width * sr.Stock.Height
}

// Efficiency returns the usage percentage.
func (sr SheetResult) Efficiency() float64 {
	ta := sr.TotalArea()
	if ta == 0 {
		return 0
	}
	return (sr.UsedArea() / ta) * 100.0
}

// OptimizeResult holds the full solution.
type OptimizeResult struct {
	Sheets        []SheetResult `json:"sheets"`
	UnplacedParts []Part        `json:"unplaced_parts"`
}

// TotalEfficiency returns overall material usage percentage.
func (or OptimizeResult) TotalEfficiency() float64 {
	var usedArea, totalArea float64
	for _, s := range or.Sheets {
		usedArea += s.UsedArea()
		totalArea += s.TotalArea()
	}
	if totalArea == 0 {
		return 0
	}
	return (usedArea / totalArea) * 100.0
}

// TotalCutLength returns the total perimeter of all placed parts (mm).
// This approximates the total cut length needed. For outline parts, it uses
// the outline perimeter; for rectangular parts, it uses 2*(W+H).
func (or OptimizeResult) TotalCutLength() float64 {
	var total float64
	for _, s := range or.Sheets {
		for _, p := range s.Placements {
			if len(p.Part.Outline) > 0 {
				total += p.Part.Outline.Perimeter()
			} else {
				total += 2 * (p.PlacedWidth() + p.PlacedHeight())
			}
		}
	}
	return total
}

// EstimatedJobTime estimates the total machining time (minutes) based on
// cut length, feed rate, and number of sheets (setup time per sheet).
// setupTimePerSheet is an additional constant overhead per sheet (e.g. 2 min).
func (or OptimizeResult) EstimatedJobTime(feedRate float64, passDepth, cutDepth float64, setupTimePerSheet float64) float64 {
	if feedRate <= 0 {
		return 0
	}
	cutLen := or.TotalCutLength()
	numPasses := 1.0
	if passDepth > 0 && cutDepth > 0 {
		numPasses = math.Ceil(cutDepth / passDepth)
	}
	cuttingTime := (cutLen * numPasses) / feedRate // minutes
	setupTime := float64(len(or.Sheets)) * setupTimePerSheet
	return cuttingTime + setupTime
}

// TotalCost returns the total material cost across all used sheets.
// Returns 0 if no sheets have pricing set.
func (or OptimizeResult) TotalCost() float64 {
	var total float64
	for _, s := range or.Sheets {
		total += s.Stock.PricePerSheet
	}
	return total
}

// HasPricing returns true if any sheet in the result has a price set.
func (or OptimizeResult) HasPricing() bool {
	for _, s := range or.Sheets {
		if s.Stock.PricePerSheet > 0 {
			return true
		}
	}
	return false
}

// Project ties everything together for save/load.
// ProjectMetadata holds sharing and collaboration metadata for a project.
type ProjectMetadata struct {
	Author      string `json:"author,omitempty"`
	Email       string `json:"email,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Version     string `json:"version,omitempty"`
	SharedFrom  string `json:"shared_from,omitempty"`
	Description string `json:"description,omitempty"`
}

// Project ties everything together for save/load.
type Project struct {
	Name     string          `json:"name"`
	Metadata ProjectMetadata `json:"metadata,omitempty"`
	Parts    []Part          `json:"parts"`
	Stocks   []StockSheet    `json:"stocks"`
	Settings CutSettings     `json:"settings"`
	Result   *OptimizeResult `json:"result,omitempty"`
}

func NewProject() Project {
	return Project{
		Name:     "Untitled",
		Parts:    []Part{},
		Stocks:   []StockSheet{},
		Settings: DefaultSettings(),
	}
}
