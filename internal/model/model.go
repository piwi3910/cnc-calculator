package model

import "github.com/google/uuid"

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

// Part represents a required piece to be cut.
type Part struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Width    float64 `json:"width"`  // mm (bounding box width for non-rectangular parts)
	Height   float64 `json:"height"` // mm (bounding box height for non-rectangular parts)
	Quantity int     `json:"quantity"`
	Grain    Grain   `json:"grain"`
	Outline  Outline `json:"outline,omitempty"` // Non-rectangular part outline; nil for rectangular parts
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
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Width    float64        `json:"width"`  // mm
	Height   float64        `json:"height"` // mm
	Quantity int            `json:"quantity"`
	Tabs     StockTabConfig `json:"tabs"` // Override default tab config for this sheet
}

func NewStockSheet(label string, w, h float64, qty int) StockSheet {
	return StockSheet{
		ID:       uuid.New().String()[:8],
		Label:    label,
		Width:    w,
		Height:   h,
		Quantity: qty,
		Tabs:     StockTabConfig{Enabled: false}, // Use defaults by default
	}
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

	// Stock sheet holding tabs (for securing sheet to CNC bed)
	StockTabs StockTabConfig `json:"stock_tabs"` // Stock sheet tab configuration

	// GCode post-processor profile
	GCodeProfile string `json:"gcode_profile"` // Name of the GCode profile to use
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

// GCodeProfile defines a post-processor configuration for different CNC controllers.
type GCodeProfile struct {
	Name        string `json:"name"`        // Profile name
	Description string `json:"description"` // Profile description
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

// GetProfile returns a GCode profile by name, or the Generic profile if not found.
func GetProfile(name string) GCodeProfile {
	for _, p := range GCodeProfiles {
		if p.Name == name {
			return p
		}
	}
	return GCodeProfiles[len(GCodeProfiles)-1] // Return Generic (last one)
}

// GetProfileNames returns a list of all available profile names.
func GetProfileNames() []string {
	var names []string
	for _, p := range GCodeProfiles {
		names = append(names, p.Name)
	}
	return names
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
		StockTabs: StockTabConfig{
			Enabled:       true, // Enabled by default
			AdvancedMode:  false,
			TopPadding:    25.0,
			BottomPadding: 25.0,
			LeftPadding:   25.0,
			RightPadding:  25.0,
			CustomZones:   nil,
		},
		GCodeProfile: "Generic", // Default GCode profile
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

// Project ties everything together for save/load.
type Project struct {
	Name     string          `json:"name"`
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
