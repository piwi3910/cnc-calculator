package gcode

import (
	"math"
	"strings"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

// newTestSettings returns CutSettings suitable for testing with predictable output.
func newTestSettings() model.CutSettings {
	s := model.DefaultSettings()
	s.ToolDiameter = 6.0
	s.FeedRate = 1000.0
	s.PlungeRate = 300.0
	s.SpindleSpeed = 12000
	s.SafeZ = 5.0
	s.CutDepth = 6.0
	s.PassDepth = 6.0
	s.GCodeProfile = "Generic"
	s.PartTabsPerSide = 0
	s.LeadInRadius = 0
	s.LeadOutRadius = 0
	return s
}

func newTestPlacement() model.Placement {
	return model.Placement{
		Part: model.Part{
			ID:       "test1",
			Label:    "TestPart",
			Width:    100,
			Height:   50,
			Quantity: 1,
		},
		X:       10,
		Y:       10,
		Rotated: false,
	}
}

func newTestSheet() model.SheetResult {
	return model.SheetResult{
		Stock: model.StockSheet{
			ID:       "stock1",
			Label:    "TestStock",
			Width:    500,
			Height:   300,
			Quantity: 1,
		},
		Placements: []model.Placement{newTestPlacement()},
	}
}

func TestGenerateSheet_NoLeadInOut(t *testing.T) {
	settings := newTestSettings()
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if strings.Contains(code, "Lead-in arc") {
		t.Error("expected no lead-in arc comment when LeadInRadius is 0")
	}
	if strings.Contains(code, "Lead-out arc") {
		t.Error("expected no lead-out arc comment when LeadOutRadius is 0")
	}
	// Check for arc commands (G2 or G3 followed by space, to avoid matching G21, G28, etc.)
	if strings.Contains(code, "G2 ") || strings.Contains(code, "G3 ") {
		t.Error("expected no arc commands (G2/G3) when lead-in/out disabled")
	}
}

func TestGenerateSheet_WithLeadIn(t *testing.T) {
	settings := newTestSettings()
	settings.LeadInRadius = 5.0
	settings.LeadInAngle = 90.0
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Lead-in arc") {
		t.Error("expected lead-in arc comment when LeadInRadius > 0")
	}
	if !strings.Contains(code, "G3") {
		t.Error("expected G3 (counter-clockwise) arc for climb milling lead-in")
	}
}

func TestGenerateSheet_WithLeadOut(t *testing.T) {
	settings := newTestSettings()
	settings.LeadOutRadius = 5.0
	settings.LeadInAngle = 90.0
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Lead-out arc") {
		t.Error("expected lead-out arc comment when LeadOutRadius > 0")
	}
	if !strings.Contains(code, "G3") {
		t.Error("expected G3 (counter-clockwise) arc for climb milling lead-out")
	}
}

func TestGenerateSheet_WithBothLeadInAndOut(t *testing.T) {
	settings := newTestSettings()
	settings.LeadInRadius = 5.0
	settings.LeadOutRadius = 3.0
	settings.LeadInAngle = 90.0
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Lead-in arc") {
		t.Error("expected lead-in arc comment")
	}
	if !strings.Contains(code, "Lead-out arc") {
		t.Error("expected lead-out arc comment")
	}

	// Count G3 commands (should have at least one for lead-in and one for lead-out)
	g3Count := strings.Count(code, "G3 ")
	if g3Count < 2 {
		t.Errorf("expected at least 2 G3 commands (lead-in + lead-out), got %d", g3Count)
	}
}

func TestGenerateSheet_ConventionalMilling(t *testing.T) {
	settings := newTestSettings()
	settings.LeadInRadius = 5.0
	settings.LeadOutRadius = 5.0
	settings.LeadInAngle = 90.0
	settings.UseClimb = false
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "G2") {
		t.Error("expected G2 (clockwise) arc for conventional milling lead-in/out")
	}
}

func TestLeadInArcGeometry(t *testing.T) {
	settings := newTestSettings()
	settings.LeadInRadius = 5.0
	settings.LeadInAngle = 90.0
	gen := New(settings)

	var b strings.Builder
	toolR := settings.ToolDiameter / 2.0
	p := newTestPlacement()
	x0 := p.X - toolR
	y0 := p.Y - toolR

	gen.writeLeadIn(&b, x0, y0, 6.0)
	output := b.String()

	// The arc start should be offset from the perimeter start
	// With radius=5 and angle=90, arcStartX = x0 - 5*sin(90) = x0 - 5
	// arcStartY = y0 - 5 + 5 - 5*cos(90) = y0 - 5 + 5 - 0 = y0
	expectedArcStartX := x0 - 5.0*math.Sin(math.Pi/2)
	expectedArcStartY := (y0 - 5.0) + 5.0 - 5.0*math.Cos(math.Pi/2)

	arcStartXStr := gen.format(expectedArcStartX)
	arcStartYStr := gen.format(expectedArcStartY)

	rapidLine := gen.profile.RapidMove + " X" + arcStartXStr + " Y" + arcStartYStr
	if !strings.Contains(output, rapidLine) {
		t.Errorf("expected rapid to arc start position %q in output:\n%s", rapidLine, output)
	}

	// Should contain arc to the perimeter start point
	perimXStr := gen.format(x0)
	perimYStr := gen.format(y0)
	if !strings.Contains(output, "X"+perimXStr+" Y"+perimYStr) {
		t.Errorf("expected arc endpoint at perimeter start (%s, %s) in output:\n%s", perimXStr, perimYStr, output)
	}
}

func TestLeadOutArcGeometry(t *testing.T) {
	settings := newTestSettings()
	settings.LeadOutRadius = 5.0
	settings.LeadInAngle = 90.0
	gen := New(settings)

	var b strings.Builder
	toolR := settings.ToolDiameter / 2.0
	p := newTestPlacement()
	x0 := p.X - toolR
	y0 := p.Y - toolR

	gen.writeLeadOut(&b, x0, y0)
	output := b.String()

	// Should contain a G3 arc command
	if !strings.Contains(output, "G3") {
		t.Errorf("expected G3 arc command in lead-out output:\n%s", output)
	}

	// The arc end should be offset from the perimeter point
	r := settings.LeadOutRadius
	angle := settings.LeadInAngle * math.Pi / 180.0
	centerX := x0
	centerY := y0 - r
	expectedEndX := centerX - r*math.Sin(angle)
	expectedEndY := centerY + r - r*math.Cos(angle)

	endXStr := gen.format(expectedEndX)
	endYStr := gen.format(expectedEndY)
	if !strings.Contains(output, "X"+endXStr+" Y"+endYStr) {
		t.Errorf("expected arc endpoint (%s, %s) in output:\n%s", endXStr, endYStr, output)
	}
}

func TestLeadIn_DisabledWhenZeroRadius(t *testing.T) {
	settings := newTestSettings()
	settings.LeadInRadius = 0
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	// Should use direct rapid + plunge, no arc
	if strings.Contains(code, "Lead-in arc") {
		t.Error("lead-in should be disabled when radius is 0")
	}
}

func TestMultiplePasses_LeadInOutOnEach(t *testing.T) {
	settings := newTestSettings()
	settings.LeadInRadius = 5.0
	settings.LeadOutRadius = 3.0
	settings.LeadInAngle = 90.0
	settings.CutDepth = 12.0
	settings.PassDepth = 6.0 // 2 passes
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	leadInCount := strings.Count(code, "Lead-in arc")
	leadOutCount := strings.Count(code, "Lead-out arc")

	if leadInCount != 2 {
		t.Errorf("expected 2 lead-in arcs (one per pass), got %d", leadInCount)
	}
	if leadOutCount != 2 {
		t.Errorf("expected 2 lead-out arcs (one per pass), got %d", leadOutCount)
	}
}

func TestDefaultSettings_LeadInOutDisabled(t *testing.T) {
	s := model.DefaultSettings()
	if s.LeadInRadius != 0 {
		t.Errorf("expected default LeadInRadius to be 0, got %f", s.LeadInRadius)
	}
	if s.LeadOutRadius != 0 {
		t.Errorf("expected default LeadOutRadius to be 0, got %f", s.LeadOutRadius)
	}
	if s.LeadInAngle != 90.0 {
		t.Errorf("expected default LeadInAngle to be 90, got %f", s.LeadInAngle)
	}
}

// ─── Toolpath Ordering Tests ────────────────────────────────

func TestToolpathOrdering_Disabled(t *testing.T) {
	settings := newTestSettings()
	settings.OptimizeToolpath = false
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if strings.Contains(code, "Toolpath ordering") {
		t.Error("expected no toolpath ordering comment when disabled")
	}
}

func TestToolpathOrdering_Enabled(t *testing.T) {
	settings := newTestSettings()
	settings.OptimizeToolpath = true
	gen := New(settings)

	sheet := model.SheetResult{
		Stock: model.StockSheet{
			ID: "stock1", Label: "TestStock",
			Width: 1000, Height: 500, Quantity: 1,
		},
		Placements: []model.Placement{
			{Part: model.Part{ID: "p1", Label: "A", Width: 100, Height: 50, Quantity: 1}, X: 800, Y: 400},
			{Part: model.Part{ID: "p2", Label: "B", Width: 100, Height: 50, Quantity: 1}, X: 10, Y: 10},
			{Part: model.Part{ID: "p3", Label: "C", Width: 100, Height: 50, Quantity: 1}, X: 400, Y: 200},
		},
	}

	code := gen.GenerateSheet(sheet, 1)

	if !strings.Contains(code, "Toolpath ordering") {
		t.Error("expected toolpath ordering comment when enabled")
	}

	// With nearest-neighbor from origin (0,0), the order should be: B (near origin), C (middle), A (far)
	idxB := strings.Index(code, "Part 1: B")
	idxC := strings.Index(code, "Part 2: C")
	idxA := strings.Index(code, "Part 3: A")

	if idxB < 0 || idxC < 0 || idxA < 0 {
		t.Fatalf("expected all parts in output, got:\n%s", code)
	}
	if !(idxB < idxC && idxC < idxA) {
		t.Errorf("expected nearest-neighbor order B,C,A but parts appeared in different order")
	}
}

func TestToolpathOrdering_SinglePart(t *testing.T) {
	settings := newTestSettings()
	settings.OptimizeToolpath = true
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	// Single part: ordering should not add comment since len(placements) <= 1
	if strings.Contains(code, "Toolpath ordering") {
		t.Error("expected no toolpath ordering comment for single part")
	}
}

func TestToolpathOrdering_ReducesDistance(t *testing.T) {
	// Placements arranged in a zigzag pattern that would benefit from reordering
	placements := []model.Placement{
		{Part: model.Part{ID: "p1", Label: "A", Width: 50, Height: 50, Quantity: 1}, X: 900, Y: 0},
		{Part: model.Part{ID: "p2", Label: "B", Width: 50, Height: 50, Quantity: 1}, X: 0, Y: 0},
		{Part: model.Part{ID: "p3", Label: "C", Width: 50, Height: 50, Quantity: 1}, X: 450, Y: 0},
	}

	originalDist := TotalRapidDistance(placements)

	settings := newTestSettings()
	gen := New(settings)
	ordered := gen.orderPlacements(placements)
	orderedDist := TotalRapidDistance(ordered)

	if orderedDist > originalDist {
		t.Errorf("ordered distance (%.2f) should not exceed original distance (%.2f)", orderedDist, originalDist)
	}
}

func TestTotalRapidDistance(t *testing.T) {
	placements := []model.Placement{
		{Part: model.Part{ID: "p1", Label: "A", Width: 0, Height: 0, Quantity: 1}, X: 100, Y: 0},
		{Part: model.Part{ID: "p2", Label: "B", Width: 0, Height: 0, Quantity: 1}, X: 200, Y: 0},
	}

	dist := TotalRapidDistance(placements)
	// From (0,0) to center of first (100,0) = 100, then to center of second (200,0) = 100
	expected := 200.0
	if math.Abs(dist-expected) > 0.01 {
		t.Errorf("expected total distance %.2f, got %.2f", expected, dist)
	}
}

func TestTotalRapidDistance_Empty(t *testing.T) {
	dist := TotalRapidDistance(nil)
	if dist != 0 {
		t.Errorf("expected 0 for empty placements, got %f", dist)
	}
}

func TestDefaultSettings_ToolpathOrderingDisabled(t *testing.T) {
	s := model.DefaultSettings()
	if s.OptimizeToolpath {
		t.Error("expected default OptimizeToolpath to be false")
	}
}

// ─── Plunge Entry Strategy Tests ────────────────────────────

func TestDirectPlunge_Default(t *testing.T) {
	settings := newTestSettings()
	// PlungeType defaults to "" which maps to direct
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	// Should not contain ramp or helix comments
	if strings.Contains(code, "Ramp plunge") {
		t.Error("expected no ramp plunge when PlungeType is direct")
	}
	if strings.Contains(code, "Helix plunge") {
		t.Error("expected no helix plunge when PlungeType is direct")
	}
}

func TestRampPlunge(t *testing.T) {
	settings := newTestSettings()
	settings.PlungeType = model.PlungeRamp
	settings.RampAngle = 5.0
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Ramp plunge entry") {
		t.Error("expected ramp plunge comment when PlungeType is ramp")
	}
	// Ramp involves simultaneous XYZ move
	if !strings.Contains(code, "Z-") {
		t.Error("expected Z descent in ramp plunge")
	}
}

func TestHelixPlunge(t *testing.T) {
	settings := newTestSettings()
	settings.PlungeType = model.PlungeHelix
	settings.HelixDiameter = 8.0
	settings.HelixRevPercent = 50.0
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Helix plunge entry") {
		t.Error("expected helix plunge comment when PlungeType is helix")
	}
	// Helix uses arc commands
	if !strings.Contains(code, "G3") && !strings.Contains(code, "G2") {
		t.Error("expected arc command (G2/G3) in helix plunge")
	}
}

func TestHelixPlunge_MultiplePasses(t *testing.T) {
	settings := newTestSettings()
	settings.PlungeType = model.PlungeHelix
	settings.HelixDiameter = 8.0
	settings.HelixRevPercent = 50.0
	settings.CutDepth = 12.0
	settings.PassDepth = 6.0 // 2 passes
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	helixCount := strings.Count(code, "Helix plunge entry")
	if helixCount != 2 {
		t.Errorf("expected 2 helix plunge entries (one per pass), got %d", helixCount)
	}
}

func TestRampPlunge_AngleClamping(t *testing.T) {
	settings := newTestSettings()
	settings.PlungeType = model.PlungeRamp
	settings.RampAngle = 0 // Should default to 3 degrees
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Ramp plunge entry (3.0 deg") {
		t.Errorf("expected default ramp angle of 3.0 degrees when set to 0, got:\n%s", code)
	}
}

func TestHelixPlunge_ClimbMilling(t *testing.T) {
	settings := newTestSettings()
	settings.PlungeType = model.PlungeHelix
	settings.HelixDiameter = 8.0
	settings.UseClimb = true
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "G3") {
		t.Error("expected G3 (counter-clockwise) for climb milling helix")
	}
}

func TestHelixPlunge_ConventionalMilling(t *testing.T) {
	settings := newTestSettings()
	settings.PlungeType = model.PlungeHelix
	settings.HelixDiameter = 8.0
	settings.UseClimb = false
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "G2 ") {
		t.Error("expected G2 (clockwise) for conventional milling helix")
	}
}

func TestDefaultSettings_PlungeType(t *testing.T) {
	s := model.DefaultSettings()
	if s.PlungeType != model.PlungeDirect {
		t.Errorf("expected default PlungeType to be direct, got %s", s.PlungeType)
	}
	if s.RampAngle != 3.0 {
		t.Errorf("expected default RampAngle to be 3.0, got %f", s.RampAngle)
	}
	if s.HelixDiameter != 5.0 {
		t.Errorf("expected default HelixDiameter to be 5.0, got %f", s.HelixDiameter)
	}
}

func TestPlungeTypeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected model.PlungeType
	}{
		{"Direct", model.PlungeDirect},
		{"Ramp", model.PlungeRamp},
		{"Helix", model.PlungeHelix},
		{"unknown", model.PlungeDirect},
	}
	for _, tt := range tests {
		got := model.PlungeTypeFromString(tt.input)
		if got != tt.expected {
			t.Errorf("PlungeTypeFromString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ─── Corner Overcut Tests ───────────────────────────────────

func TestCornerOvercut_None(t *testing.T) {
	settings := newTestSettings()
	settings.CornerOvercut = model.CornerOvercutNone
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	// Standard perimeter should have exactly 4 feed moves for the rectangle
	// (plus the return-to-start from lead-in or closing the rectangle)
	// No extra moves for overcuts
	lines := strings.Split(code, "\n")
	overcutMoves := 0
	for _, line := range lines {
		// Overcut moves go to intermediate positions; with no overcuts
		// there should be no such moves. We verify by checking that
		// the code does not contain diagonal overcut positions.
		if strings.Contains(line, "overcut") {
			overcutMoves++
		}
	}
	if overcutMoves > 0 {
		t.Error("expected no overcut-related content when corner overcut is none")
	}
}

func TestCornerOvercut_Dogbone(t *testing.T) {
	settings := newTestSettings()
	settings.CornerOvercut = model.CornerOvercutDogbone
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	// With dogbone overcuts, there should be additional G1 moves at each corner.
	// Each corner gets a move-out + move-back = 2 extra G1 moves.
	// 4 corners * 2 moves = 8 extra G1 feed moves beyond the standard 4 perimeter moves.
	feedMoveCount := strings.Count(code, gen.profile.FeedMove)

	// Standard rect: plunge(1) + 4 perimeter + retract = 5 G1 moves
	// With dogbone: plunge(1) + 4 perimeter + 8 overcut = 13 G1 moves (plus Z moves)
	// We just verify there are more G1 moves than without overcuts
	settingsNoOvercut := newTestSettings()
	genNoOvercut := New(settingsNoOvercut)
	codeNoOvercut := genNoOvercut.GenerateSheet(newTestSheet(), 1)
	feedMoveCountNoOvercut := strings.Count(codeNoOvercut, gen.profile.FeedMove)

	if feedMoveCount <= feedMoveCountNoOvercut {
		t.Errorf("expected more feed moves with dogbone overcuts (%d) than without (%d)",
			feedMoveCount, feedMoveCountNoOvercut)
	}
}

func TestCornerOvercut_Tbone(t *testing.T) {
	settings := newTestSettings()
	settings.CornerOvercut = model.CornerOvercutTbone
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	// T-bone should also add extra moves
	settingsNoOvercut := newTestSettings()
	genNoOvercut := New(settingsNoOvercut)
	codeNoOvercut := genNoOvercut.GenerateSheet(newTestSheet(), 1)

	feedMoveCount := strings.Count(code, gen.profile.FeedMove)
	feedMoveCountNoOvercut := strings.Count(codeNoOvercut, gen.profile.FeedMove)

	if feedMoveCount <= feedMoveCountNoOvercut {
		t.Errorf("expected more feed moves with T-bone overcuts (%d) than without (%d)",
			feedMoveCount, feedMoveCountNoOvercut)
	}
}

func TestCornerOvercut_DogboneDiagonalPositions(t *testing.T) {
	settings := newTestSettings()
	settings.CornerOvercut = model.CornerOvercutDogbone
	settings.ToolDiameter = 6.0
	gen := New(settings)

	toolR := settings.ToolDiameter / 2.0
	p := newTestPlacement()
	pw := p.PlacedWidth()
	ph := p.PlacedHeight()
	x0 := p.X - toolR
	y0 := p.Y - toolR
	x1 := p.X + pw + toolR
	y1 := p.Y + ph + toolR

	code := gen.GenerateSheet(newTestSheet(), 1)

	// Bottom-right corner (x1, y0) should have a diagonal overcut toward (x1+d, y0-d)
	sqrt2inv := 1.0 / math.Sqrt(2.0)
	overcutDist := toolR * sqrt2inv
	expectedX := x1 + overcutDist
	expectedY := y0 - overcutDist

	expectedStr := "X" + gen.format(expectedX) + " Y" + gen.format(expectedY)
	if !strings.Contains(code, expectedStr) {
		t.Errorf("expected dogbone overcut position %s in output for corner (%.1f, %.1f):\n%s",
			expectedStr, x1, y0, code)
	}
	_ = x0
	_ = y1
}

func TestCornerOvercut_TbonePerpendicularPositions(t *testing.T) {
	settings := newTestSettings()
	settings.CornerOvercut = model.CornerOvercutTbone
	settings.ToolDiameter = 6.0
	gen := New(settings)

	toolR := settings.ToolDiameter / 2.0
	p := newTestPlacement()
	pw := p.PlacedWidth()
	x1 := p.X + pw + toolR
	y0 := p.Y - toolR

	code := gen.GenerateSheet(newTestSheet(), 1)

	// Bottom-right corner (x1, y0): T-bone overcuts along X (right)
	expectedX := x1 + toolR
	expectedY := y0
	expectedStr := "X" + gen.format(expectedX) + " Y" + gen.format(expectedY)
	if !strings.Contains(code, expectedStr) {
		t.Errorf("expected T-bone overcut position %s for bottom-right corner:\n%s",
			expectedStr, code)
	}
}

func TestDefaultSettings_CornerOvercut(t *testing.T) {
	s := model.DefaultSettings()
	if s.CornerOvercut != model.CornerOvercutNone {
		t.Errorf("expected default CornerOvercut to be none, got %s", s.CornerOvercut)
	}
}

func TestCornerOvercutFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected model.CornerOvercut
	}{
		{"None", model.CornerOvercutNone},
		{"Dogbone", model.CornerOvercutDogbone},
		{"T-Bone", model.CornerOvercutTbone},
		{"unknown", model.CornerOvercutNone},
	}
	for _, tt := range tests {
		got := model.CornerOvercutFromString(tt.input)
		if got != tt.expected {
			t.Errorf("CornerOvercutFromString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ─── Onion Skinning Tests ───────────────────────────────────

func TestOnionSkin_Disabled(t *testing.T) {
	settings := newTestSettings()
	settings.OnionSkinEnabled = false
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if strings.Contains(code, "Onion skin") {
		t.Error("expected no onion skin comments when disabled")
	}
}

func TestOnionSkin_Enabled(t *testing.T) {
	settings := newTestSettings()
	settings.OnionSkinEnabled = true
	settings.OnionSkinDepth = 0.2
	settings.CutDepth = 6.0
	settings.PassDepth = 6.0 // 1 pass
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Onion skin: leaving 0.20mm skin") {
		t.Error("expected onion skin comment when enabled")
	}

	// The final pass depth should be 6.0 - 0.2 = 5.8
	if !strings.Contains(code, "depth=5.80mm") {
		t.Errorf("expected depth of 5.80mm with onion skin, got:\n%s", code)
	}
}

func TestOnionSkin_MultiplePassesOnlyAffectsFinal(t *testing.T) {
	settings := newTestSettings()
	settings.OnionSkinEnabled = true
	settings.OnionSkinDepth = 0.2
	settings.CutDepth = 12.0
	settings.PassDepth = 6.0 // 2 passes
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	// First pass should be at full 6.0mm depth
	if !strings.Contains(code, "depth=6.00mm") {
		t.Error("expected first pass at 6.00mm depth")
	}

	// Second (final) pass should be at 12.0 - 0.2 = 11.80mm
	if !strings.Contains(code, "depth=11.80mm") {
		t.Errorf("expected final pass at 11.80mm with onion skin, got:\n%s", code)
	}

	// Should have exactly one onion skin comment
	skinCount := strings.Count(code, "Onion skin: leaving")
	if skinCount != 1 {
		t.Errorf("expected 1 onion skin comment, got %d", skinCount)
	}
}

func TestOnionSkin_WithCleanupPass(t *testing.T) {
	settings := newTestSettings()
	settings.OnionSkinEnabled = true
	settings.OnionSkinDepth = 0.2
	settings.OnionSkinCleanup = true
	settings.CutDepth = 6.0
	settings.PassDepth = 6.0
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if !strings.Contains(code, "Onion skin cleanup pass") {
		t.Error("expected cleanup pass comment when OnionSkinCleanup is true")
	}

	// Cleanup should be at full depth
	if !strings.Contains(code, "Cleanup depth=6.00mm") {
		t.Errorf("expected cleanup at full depth 6.00mm, got:\n%s", code)
	}
}

func TestOnionSkin_NoCleanupWhenDisabled(t *testing.T) {
	settings := newTestSettings()
	settings.OnionSkinEnabled = true
	settings.OnionSkinDepth = 0.2
	settings.OnionSkinCleanup = false
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if strings.Contains(code, "Onion skin cleanup pass") {
		t.Error("expected no cleanup pass when OnionSkinCleanup is false")
	}
}

func TestOnionSkin_ZeroDepthIgnored(t *testing.T) {
	settings := newTestSettings()
	settings.OnionSkinEnabled = true
	settings.OnionSkinDepth = 0.0 // Zero depth = effectively disabled
	gen := New(settings)
	code := gen.GenerateSheet(newTestSheet(), 1)

	if strings.Contains(code, "Onion skin:") {
		t.Error("expected no onion skin when depth is 0")
	}
}

func TestDefaultSettings_OnionSkin(t *testing.T) {
	s := model.DefaultSettings()
	if s.OnionSkinEnabled {
		t.Error("expected default OnionSkinEnabled to be false")
	}
	if s.OnionSkinDepth != 0.2 {
		t.Errorf("expected default OnionSkinDepth to be 0.2, got %f", s.OnionSkinDepth)
	}
	if s.OnionSkinCleanup {
		t.Error("expected default OnionSkinCleanup to be false")
	}
}

// --- Structural Cut Ordering Tests ---

func TestStructuralOrder_InteriorFirst(t *testing.T) {
	// Place 3 parts on a 1000x1000 sheet:
	// - center part (most interior) should be cut first
	// - edge part (least interior) should be cut last
	settings := newTestSettings()
	settings.StructuralOrdering = true
	gen := New(settings)

	sheet := model.SheetResult{
		Stock: model.StockSheet{Width: 1000, Height: 1000},
		Placements: []model.Placement{
			{Part: model.Part{Label: "Edge", Width: 100, Height: 100}, X: 0, Y: 0},           // on corner, minEdgeDist=0
			{Part: model.Part{Label: "Middle", Width: 100, Height: 100}, X: 200, Y: 200},      // minEdgeDist=200
			{Part: model.Part{Label: "Center", Width: 100, Height: 100}, X: 450, Y: 450},      // minEdgeDist=450
		},
	}

	code := gen.GenerateSheet(sheet, 1)

	// Center should appear first, Edge last in the GCode
	centerIdx := strings.Index(code, "Center")
	middleIdx := strings.Index(code, "Middle")
	edgeIdx := strings.Index(code, "Edge")

	if centerIdx < 0 || middleIdx < 0 || edgeIdx < 0 {
		t.Fatal("expected all three part labels to appear in GCode")
	}
	if centerIdx > middleIdx {
		t.Errorf("Center (most interior) should be cut before Middle: center@%d, middle@%d", centerIdx, middleIdx)
	}
	if middleIdx > edgeIdx {
		t.Errorf("Middle should be cut before Edge: middle@%d, edge@%d", middleIdx, edgeIdx)
	}
}

func TestStructuralOrder_SinglePlacement(t *testing.T) {
	settings := newTestSettings()
	settings.StructuralOrdering = true
	gen := New(settings)

	sheet := model.SheetResult{
		Stock: model.StockSheet{Width: 500, Height: 300},
		Placements: []model.Placement{
			{Part: model.Part{Label: "Only", Width: 100, Height: 100}, X: 50, Y: 50},
		},
	}

	code := gen.GenerateSheet(sheet, 1)
	if !strings.Contains(code, "Only") {
		t.Error("expected single part label in GCode")
	}
}

func TestStructuralOrder_Disabled(t *testing.T) {
	// When structural ordering is disabled, order should remain unchanged
	settings := newTestSettings()
	settings.StructuralOrdering = false
	settings.OptimizeToolpath = false
	gen := New(settings)

	sheet := model.SheetResult{
		Stock: model.StockSheet{Width: 1000, Height: 1000},
		Placements: []model.Placement{
			{Part: model.Part{Label: "Edge", Width: 100, Height: 100}, X: 0, Y: 0},
			{Part: model.Part{Label: "Center", Width: 100, Height: 100}, X: 450, Y: 450},
		},
	}

	code := gen.GenerateSheet(sheet, 1)
	edgeIdx := strings.Index(code, "Edge")
	centerIdx := strings.Index(code, "Center")

	// Edge should appear first (original order preserved)
	if edgeIdx > centerIdx {
		t.Error("with structural ordering disabled, original order should be preserved")
	}
}

func TestStructuralOrder_OverridesToolpathOptimization(t *testing.T) {
	// When both are enabled, structural ordering takes priority
	settings := newTestSettings()
	settings.StructuralOrdering = true
	settings.OptimizeToolpath = true
	gen := New(settings)

	sheet := model.SheetResult{
		Stock: model.StockSheet{Width: 1000, Height: 1000},
		Placements: []model.Placement{
			{Part: model.Part{Label: "Edge", Width: 100, Height: 100}, X: 0, Y: 0},
			{Part: model.Part{Label: "Center", Width: 100, Height: 100}, X: 450, Y: 450},
		},
	}

	code := gen.GenerateSheet(sheet, 1)
	if !strings.Contains(code, "structural integrity") {
		t.Error("expected structural integrity comment when structural ordering enabled")
	}
	if strings.Contains(code, "nearest-neighbor") {
		t.Error("structural ordering should override nearest-neighbor when both enabled")
	}
}

func TestStructuralOrder_TiebreakByCenter(t *testing.T) {
	// Two parts with same min-edge-distance: one closer to center should be first
	settings := newTestSettings()
	settings.StructuralOrdering = true
	gen := New(settings)

	// Sheet 1000x1000, center at (500,500)
	// PartA at (100,100): minEdgeDist = 100, center distance ~= 565
	// PartB at (400,100): minEdgeDist = 100, center distance ~= 412
	sheet := model.SheetResult{
		Stock: model.StockSheet{Width: 1000, Height: 1000},
		Placements: []model.Placement{
			{Part: model.Part{Label: "FarCenter", Width: 100, Height: 100}, X: 100, Y: 100},
			{Part: model.Part{Label: "NearCenter", Width: 100, Height: 100}, X: 400, Y: 100},
		},
	}

	code := gen.GenerateSheet(sheet, 1)
	nearIdx := strings.Index(code, "NearCenter")
	farIdx := strings.Index(code, "FarCenter")

	if nearIdx < 0 || farIdx < 0 {
		t.Fatal("expected both part labels in GCode")
	}
	if nearIdx > farIdx {
		t.Error("NearCenter (closer to sheet center) should be cut before FarCenter on tiebreak")
	}
}

func TestMinEdgeDistance(t *testing.T) {
	tests := []struct {
		name     string
		p        model.Placement
		sheetW   float64
		sheetH   float64
		expected float64
	}{
		{
			name:     "corner part",
			p:        model.Placement{Part: model.Part{Width: 100, Height: 100}, X: 0, Y: 0},
			sheetW:   1000,
			sheetH:   1000,
			expected: 0,
		},
		{
			name:     "centered part",
			p:        model.Placement{Part: model.Part{Width: 100, Height: 100}, X: 450, Y: 450},
			sheetW:   1000,
			sheetH:   1000,
			expected: 450,
		},
		{
			name:     "edge part",
			p:        model.Placement{Part: model.Part{Width: 100, Height: 100}, X: 50, Y: 400},
			sheetW:   1000,
			sheetH:   1000,
			expected: 50, // closest to left edge
		},
		{
			name:     "rotated part",
			p:        model.Placement{Part: model.Part{Width: 200, Height: 100}, X: 100, Y: 100, Rotated: true},
			sheetW:   1000,
			sheetH:   1000,
			expected: 100, // uses PlacedWidth=100, PlacedHeight=200
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := minEdgeDistance(tc.p, tc.sheetW, tc.sheetH)
			if math.Abs(got-tc.expected) > 0.01 {
				t.Errorf("minEdgeDistance = %.2f, want %.2f", got, tc.expected)
			}
		})
	}
}

func TestCenterDistance(t *testing.T) {
	p := model.Placement{Part: model.Part{Width: 100, Height: 100}, X: 0, Y: 0}
	// Center of part is (50, 50), sheet center (500, 500)
	d := centerDistance(p, 500, 500)
	expected := math.Sqrt(450*450 + 450*450)
	if math.Abs(d-expected) > 0.01 {
		t.Errorf("centerDistance = %.2f, want %.2f", d, expected)
	}
}

func TestStructuralOrder_CommentInGCode(t *testing.T) {
	settings := newTestSettings()
	settings.StructuralOrdering = true
	gen := New(settings)

	sheet := model.SheetResult{
		Stock: model.StockSheet{Width: 500, Height: 300},
		Placements: []model.Placement{
			{Part: model.Part{Label: "A", Width: 50, Height: 50}, X: 10, Y: 10},
			{Part: model.Part{Label: "B", Width: 50, Height: 50}, X: 200, Y: 100},
		},
	}

	code := gen.GenerateSheet(sheet, 1)
	if !strings.Contains(code, "structural integrity") {
		t.Error("expected structural integrity comment in GCode output")
	}
}
