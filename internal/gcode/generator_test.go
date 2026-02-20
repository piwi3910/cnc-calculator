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
