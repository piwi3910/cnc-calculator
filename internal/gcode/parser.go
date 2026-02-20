package gcode

import (
	"regexp"
	"strconv"
	"strings"
)

// MoveType represents the type of CNC toolpath movement.
type MoveType int

const (
	MoveRapid   MoveType = iota // G0: rapid positioning (no cutting)
	MoveFeed                    // G1: linear feed (cutting move in XY plane)
	MovePlunge                  // G1 with Z decreasing: plunging into material
	MoveRetract                 // G0/G1 with Z increasing: retracting from material
)

// GCodeMove represents a single parsed movement from GCode.
type GCodeMove struct {
	Type     MoveType
	FromX    float64
	FromY    float64
	FromZ    float64
	ToX      float64
	ToY      float64
	ToZ      float64
	FeedRate float64
}

// ParseGCode parses a GCode string into a slice of structured moves.
// It tracks absolute position state and classifies each G0/G1 command
// by its movement characteristics (rapid, feed, plunge, retract).
func ParseGCode(code string) []GCodeMove {
	var moves []GCodeMove

	// Current machine state
	curX, curY, curZ := 0.0, 0.0, 0.0
	curFeed := 0.0

	lines := strings.Split(code, "\n")

	coordRe := regexp.MustCompile(`([XYZF])([-]?\d+\.?\d*)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Strip inline comments (semicolon or parenthetical)
		if idx := strings.Index(line, ";"); idx >= 0 {
			line = line[:idx]
		}
		if idx := strings.Index(line, "("); idx >= 0 {
			if end := strings.Index(line, ")"); end > idx {
				line = line[:idx] + line[end+1:]
			}
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Determine command type
		isRapid := false
		isFeed := false
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "G0 ") || strings.HasPrefix(upper, "G00 ") || upper == "G0" || upper == "G00" {
			isRapid = true
		} else if strings.HasPrefix(upper, "G1 ") || strings.HasPrefix(upper, "G01 ") || upper == "G1" || upper == "G01" {
			isFeed = true
		}

		if !isRapid && !isFeed {
			continue
		}

		// Parse coordinates from this line
		newX, newY, newZ, newFeed := curX, curY, curZ, curFeed
		matches := coordRe.FindAllStringSubmatch(upper, -1)
		for _, m := range matches {
			val, err := strconv.ParseFloat(m[2], 64)
			if err != nil {
				continue
			}
			switch m[1] {
			case "X":
				newX = val
			case "Y":
				newY = val
			case "Z":
				newZ = val
			case "F":
				newFeed = val
			}
		}

		// Classify the move
		moveType := classifyMove(isRapid, curZ, newZ, curX, curY, newX, newY)

		moves = append(moves, GCodeMove{
			Type:     moveType,
			FromX:    curX,
			FromY:    curY,
			FromZ:    curZ,
			ToX:      newX,
			ToY:      newY,
			ToZ:      newZ,
			FeedRate: newFeed,
		})

		curX, curY, curZ, curFeed = newX, newY, newZ, newFeed
	}

	return moves
}

// classifyMove determines the MoveType based on movement characteristics.
func classifyMove(isRapid bool, fromZ, toZ, fromX, fromY, toX, toY float64) MoveType {
	zDelta := toZ - fromZ
	hasXY := fromX != toX || fromY != toY

	switch {
	case isRapid:
		if zDelta > 0 {
			return MoveRetract
		}
		return MoveRapid
	case zDelta < -0.001 && !hasXY:
		// Z going down (more negative) without XY movement = plunge
		return MovePlunge
	case zDelta > 0.001 && !hasXY:
		// Z going up without XY movement = retract
		return MoveRetract
	default:
		return MoveFeed
	}
}
