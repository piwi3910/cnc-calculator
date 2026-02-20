package gcode

import (
	"testing"
)

func TestParseGCode_Empty(t *testing.T) {
	moves := ParseGCode("")
	if len(moves) != 0 {
		t.Errorf("expected 0 moves for empty input, got %d", len(moves))
	}
}

func TestParseGCode_CommentsOnly(t *testing.T) {
	code := `; This is a comment
; Another comment
(parenthetical comment)
`
	moves := ParseGCode(code)
	if len(moves) != 0 {
		t.Errorf("expected 0 moves for comments-only input, got %d", len(moves))
	}
}

func TestParseGCode_RapidMove(t *testing.T) {
	code := "G0 X10.000 Y20.000\n"
	moves := ParseGCode(code)
	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	m := moves[0]
	if m.Type != MoveRapid {
		t.Errorf("expected MoveRapid, got %d", m.Type)
	}
	if m.FromX != 0 || m.FromY != 0 {
		t.Errorf("expected from (0,0), got (%.3f, %.3f)", m.FromX, m.FromY)
	}
	if m.ToX != 10 || m.ToY != 20 {
		t.Errorf("expected to (10,20), got (%.3f, %.3f)", m.ToX, m.ToY)
	}
}

func TestParseGCode_FeedMove(t *testing.T) {
	code := "G0 X0.000 Y0.000\nG1 X100.000 Y0.000 F1500.0\n"
	moves := ParseGCode(code)
	if len(moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(moves))
	}
	m := moves[1]
	if m.Type != MoveFeed {
		t.Errorf("expected MoveFeed, got %d", m.Type)
	}
	if m.ToX != 100 || m.ToY != 0 {
		t.Errorf("expected to (100,0), got (%.3f, %.3f)", m.ToX, m.ToY)
	}
	if m.FeedRate != 1500 {
		t.Errorf("expected feed rate 1500, got %.1f", m.FeedRate)
	}
}

func TestParseGCode_PlungeMove(t *testing.T) {
	code := "G0 X10.000 Y10.000\nG0 Z5.000\nG1 Z-6.000 F500.0\n"
	moves := ParseGCode(code)
	if len(moves) != 3 {
		t.Fatalf("expected 3 moves, got %d", len(moves))
	}
	m := moves[2]
	if m.Type != MovePlunge {
		t.Errorf("expected MovePlunge, got %d", m.Type)
	}
	if m.FromZ != 5 || m.ToZ != -6 {
		t.Errorf("expected Z from 5 to -6, got %.3f to %.3f", m.FromZ, m.ToZ)
	}
}

func TestParseGCode_RetractMove(t *testing.T) {
	code := "G0 X10.000 Y10.000\nG1 Z-6.000 F500.0\nG0 Z5.000\n"
	moves := ParseGCode(code)
	if len(moves) != 3 {
		t.Fatalf("expected 3 moves, got %d", len(moves))
	}
	m := moves[2]
	if m.Type != MoveRetract {
		t.Errorf("expected MoveRetract, got %d", m.Type)
	}
	if m.ToZ != 5 {
		t.Errorf("expected retract to Z=5, got Z=%.3f", m.ToZ)
	}
}

func TestParseGCode_InlineComment(t *testing.T) {
	code := "G1 X50.000 Y50.000 F1500.0 ; cutting move\n"
	moves := ParseGCode(code)
	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	if moves[0].ToX != 50 || moves[0].ToY != 50 {
		t.Errorf("expected to (50,50), got (%.3f, %.3f)", moves[0].ToX, moves[0].ToY)
	}
}

func TestParseGCode_NonMovementLines(t *testing.T) {
	code := `G90
G21
G17
M3 S18000
G0 X0.000 Y0.000
G0 Z5.000
`
	moves := ParseGCode(code)
	if len(moves) != 2 {
		t.Errorf("expected 2 moves (only G0 lines), got %d", len(moves))
	}
}

func TestParseGCode_StateTracking(t *testing.T) {
	code := `G0 X10.000 Y20.000
G0 Z5.000
G1 Z-6.000 F500.0
G1 X100.000 Y20.000 F1500.0
G1 X100.000 Y80.000
G0 Z5.000
`
	moves := ParseGCode(code)
	if len(moves) != 6 {
		t.Fatalf("expected 6 moves, got %d", len(moves))
	}

	// Verify position state is tracked across moves
	// Move 3 (index 2): plunge at X=10, Y=20
	if moves[2].FromX != 10 || moves[2].FromY != 20 {
		t.Errorf("move 2: expected from (10,20), got (%.3f, %.3f)", moves[2].FromX, moves[2].FromY)
	}
	// Move 4 (index 3): feed from (10,20) to (100,20)
	if moves[3].FromX != 10 || moves[3].ToX != 100 {
		t.Errorf("move 3: expected X from 10 to 100, got %.3f to %.3f", moves[3].FromX, moves[3].ToX)
	}
	// Move 5 (index 4): feed from (100,20) to (100,80)
	if moves[4].FromX != 100 || moves[4].FromY != 20 || moves[4].ToY != 80 {
		t.Errorf("move 4: expected from (100,20) to (100,80), got (%.3f,%.3f) to (%.3f,%.3f)",
			moves[4].FromX, moves[4].FromY, moves[4].ToX, moves[4].ToY)
	}
}

func TestParseGCode_FeedRateSticky(t *testing.T) {
	code := `G1 X10.000 Y10.000 F1500.0
G1 X20.000 Y20.000
`
	moves := ParseGCode(code)
	if len(moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(moves))
	}
	// Feed rate should persist from previous command
	if moves[1].FeedRate != 1500 {
		t.Errorf("expected sticky feed rate 1500, got %.1f", moves[1].FeedRate)
	}
}

func TestParseGCode_FullCutSequence(t *testing.T) {
	// Simulate a realistic single-part GCode sequence
	code := `; CNCCalculator GCode - Sheet 1
G90
G21
G17
M3 S18000
G0 X0.000 Y0.000
G0 Z5.000

; --- Part 1: Shelf (600.0 x 300.0) ---
; Pass 1/3, depth=6.00mm
G0 X-3.000 Y-3.000
G1 Z-6.000 F500.0 ; Plunge
G1 X603.000 Y-3.000 F1500.0
G1 X603.000 Y303.000
G1 X-3.000 Y303.000
G1 X-3.000 Y-3.000
G0 Z5.000

; === Job complete ===
G0 Z5.000
G0 X0 Y0
M5
M2
`
	moves := ParseGCode(code)

	// Count move types
	counts := map[MoveType]int{}
	for _, m := range moves {
		counts[m.Type]++
	}

	if counts[MoveRapid] < 2 {
		t.Errorf("expected at least 2 rapid moves, got %d", counts[MoveRapid])
	}
	if counts[MoveFeed] < 4 {
		t.Errorf("expected at least 4 feed moves (rectangle perimeter), got %d", counts[MoveFeed])
	}
	if counts[MovePlunge] < 1 {
		t.Errorf("expected at least 1 plunge move, got %d", counts[MovePlunge])
	}
	if counts[MoveRetract] < 1 {
		t.Errorf("expected at least 1 retract move, got %d", counts[MoveRetract])
	}
}

func TestClassifyMove(t *testing.T) {
	tests := []struct {
		name    string
		isRapid bool
		fromZ   float64
		toZ     float64
		fromX   float64
		fromY   float64
		toX     float64
		toY     float64
		want    MoveType
	}{
		{"rapid XY", true, 5, 5, 0, 0, 10, 20, MoveRapid},
		{"rapid retract", true, -6, 5, 10, 20, 10, 20, MoveRetract},
		{"rapid with Z up", true, 0, 5, 0, 0, 0, 0, MoveRetract},
		{"feed XY", false, -6, -6, 0, 0, 100, 0, MoveFeed},
		{"plunge", false, 5, -6, 10, 20, 10, 20, MovePlunge},
		{"retract feed", false, -6, 0, 10, 20, 10, 20, MoveRetract},
		{"feed with slight Z", false, -6, -6.0001, 0, 0, 100, 0, MoveFeed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyMove(tt.isRapid, tt.fromZ, tt.toZ, tt.fromX, tt.fromY, tt.toX, tt.toY)
			if got != tt.want {
				t.Errorf("classifyMove() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseGCode_NegativeCoordinates(t *testing.T) {
	code := "G0 X-3.000 Y-3.000\n"
	moves := ParseGCode(code)
	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	if moves[0].ToX != -3 || moves[0].ToY != -3 {
		t.Errorf("expected to (-3,-3), got (%.3f, %.3f)", moves[0].ToX, moves[0].ToY)
	}
}
