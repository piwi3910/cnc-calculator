package widgets

import (
	"fmt"
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/piwi3910/SlabCut/internal/gcode"
	"github.com/piwi3910/SlabCut/internal/model"
)

// Toolpath colors for different move types.
var (
	colorRapid   = color.NRGBA{R: 255, G: 60, B: 60, A: 200}   // Red for rapid moves
	colorFeed    = color.NRGBA{R: 30, G: 120, B: 255, A: 230}  // Blue for cutting moves
	colorPlunge  = color.NRGBA{R: 50, G: 200, B: 50, A: 220}   // Green for plunge
	colorRetract = color.NRGBA{R: 180, G: 180, B: 0, A: 180}   // Yellow for retract
	colorSheet   = color.NRGBA{R: 230, G: 210, B: 175, A: 255} // Light wood for stock
	colorPart    = color.NRGBA{R: 200, G: 220, B: 255, A: 120} // Light blue for part outlines
	colorTab     = color.NRGBA{R: 255, G: 165, B: 0, A: 220}   // Orange for tab markers
	colorDimFeed = color.NRGBA{R: 30, G: 120, B: 255, A: 60}   // Dim blue for remaining feed moves
	colorDimRap  = color.NRGBA{R: 255, G: 60, B: 60, A: 50}    // Dim red for remaining rapid moves
	colorToolPos = color.NRGBA{R: 255, G: 0, B: 0, A: 255}     // Bright red for tool position
	colorDoneFd  = color.NRGBA{R: 0, G: 200, B: 80, A: 230}    // Green for completed feed
	colorDoneRap = color.NRGBA{R: 200, G: 100, B: 100, A: 130} // Dim completed rapid
)

// GCodePreview is a custom Fyne widget that renders a visual preview
// of GCode toolpath movements overlaid on a stock sheet with part outlines.
// It supports simulation mode where only a subset of moves are shown as "completed".
type GCodePreview struct {
	widget.BaseWidget
	moves      []gcode.GCodeMove
	placements []model.Placement
	settings   model.CutSettings
	sheetW     float64
	sheetH     float64
	maxWidth   float32
	maxHeight  float32

	// Simulation state: how many moves to show as completed.
	// -1 means show all moves (no simulation mode / show everything).
	mu           sync.Mutex
	visibleMoves int
}

// NewGCodePreview creates a new GCode preview widget.
func NewGCodePreview(moves []gcode.GCodeMove, placements []model.Placement, settings model.CutSettings, sheetW, sheetH float64, maxW, maxH float32) *GCodePreview {
	gp := &GCodePreview{
		moves:        moves,
		placements:   placements,
		settings:     settings,
		sheetW:       sheetW,
		sheetH:       sheetH,
		maxWidth:     maxW,
		maxHeight:    maxH,
		visibleMoves: -1, // show all by default
	}
	gp.ExtendBaseWidget(gp)
	return gp
}

// SetVisibleMoves sets how many moves to show as "completed" in simulation mode.
// Pass -1 to show all moves (no simulation). Pass 0 to show none completed.
func (gp *GCodePreview) SetVisibleMoves(n int) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	if n >= len(gp.moves) {
		gp.visibleMoves = -1
	} else {
		gp.visibleMoves = n
	}
	gp.Refresh()
}

// MoveCount returns the total number of GCode moves.
func (gp *GCodePreview) MoveCount() int {
	return len(gp.moves)
}

// MoveInfo holds display information about a single GCode move.
type MoveInfo struct {
	Index    int
	Type     string
	ToX      float64
	ToY      float64
	ToZ      float64
	FeedRate float64
}

// GetMoveInfo returns information about the move at the given index.
// Returns nil if the index is out of range.
func (gp *GCodePreview) GetMoveInfo(idx int) *MoveInfo {
	if idx < 0 || idx >= len(gp.moves) {
		return nil
	}
	m := gp.moves[idx]
	var typeName string
	switch m.Type {
	case gcode.MoveRapid:
		typeName = "Rapid"
	case gcode.MoveFeed:
		typeName = "Feed"
	case gcode.MovePlunge:
		typeName = "Plunge"
	case gcode.MoveRetract:
		typeName = "Retract"
	default:
		typeName = "Unknown"
	}
	return &MoveInfo{
		Index:    idx,
		Type:     typeName,
		ToX:      m.ToX,
		ToY:      m.ToY,
		ToZ:      m.ToZ,
		FeedRate: m.FeedRate,
	}
}

// CreateRenderer implements fyne.Widget.
func (gp *GCodePreview) CreateRenderer() fyne.WidgetRenderer {
	return newGCodePreviewRenderer(gp)
}

type gcodePreviewRenderer struct {
	gp               *GCodePreview
	objects          []fyne.CanvasObject
	lastVisibleMoves int
	built            bool
}

func newGCodePreviewRenderer(gp *GCodePreview) *gcodePreviewRenderer {
	r := &gcodePreviewRenderer{gp: gp}
	r.rebuild()
	return r
}

func (r *gcodePreviewRenderer) rebuild() {
	r.objects = nil

	gp := r.gp
	gp.mu.Lock()
	r.lastVisibleMoves = gp.visibleMoves
	r.built = true
	gp.mu.Unlock()
	stockW := float32(gp.sheetW)
	stockH := float32(gp.sheetH)

	if stockW <= 0 || stockH <= 0 {
		return
	}

	// Calculate scale to fit within max bounds, with margin for tool offset
	margin := float32(gp.settings.ToolDiameter) + 10
	scaleX := (gp.maxWidth - margin*2) / stockW
	scaleY := (gp.maxHeight - margin*2) / stockH
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	if scale <= 0 {
		scale = 1
	}

	offsetX := margin
	offsetY := margin

	canvasW := stockW * scale
	canvasH := stockH * scale

	// Stock sheet background
	bg := canvas.NewRectangle(colorSheet)
	bg.Resize(fyne.NewSize(canvasW, canvasH))
	bg.Move(fyne.NewPos(offsetX, offsetY))
	r.objects = append(r.objects, bg)

	// Stock border — use theme foreground with alpha for theme awareness
	border := canvas.NewRectangle(color.Transparent)
	fgColor := theme.ForegroundColor()
	fgR, fgG, fgB, _ := fgColor.RGBA()
	border.StrokeColor = color.NRGBA{R: uint8(fgR >> 8), G: uint8(fgG >> 8), B: uint8(fgB >> 8), A: 180}
	border.StrokeWidth = 2
	border.Resize(fyne.NewSize(canvasW, canvasH))
	border.Move(fyne.NewPos(offsetX, offsetY))
	r.objects = append(r.objects, border)

	// Draw part outlines
	for _, p := range gp.placements {
		pw := float32(p.PlacedWidth()) * scale
		ph := float32(p.PlacedHeight()) * scale
		px := float32(p.X)*scale + offsetX
		py := float32(p.Y)*scale + offsetY

		partRect := canvas.NewRectangle(colorPart)
		partRect.Resize(fyne.NewSize(pw, ph))
		partRect.Move(fyne.NewPos(px, py))
		r.objects = append(r.objects, partRect)

		partBorder := canvas.NewRectangle(color.Transparent)
		partBorder.StrokeColor = color.NRGBA{R: 100, G: 130, B: 180, A: 200}
		partBorder.StrokeWidth = 1.5
		partBorder.Resize(fyne.NewSize(pw, ph))
		partBorder.Move(fyne.NewPos(px, py))
		r.objects = append(r.objects, partBorder)

		// Part label — use adaptive text color based on colorPart luminance
		if pw > 40 && ph > 18 {
			labelColor := contrastTextColor(colorPart)
			label := canvas.NewText(p.Part.Label, labelColor)
			label.TextSize = 10
			label.Move(fyne.NewPos(px+3, py+2))
			r.objects = append(r.objects, label)
		}
	}

	// Draw tab markers if tabs are configured
	if gp.settings.PartTabsPerSide > 0 {
		r.drawTabMarkers(scale, offsetX, offsetY)
	}

	gp.mu.Lock()
	visibleMoves := gp.visibleMoves
	gp.mu.Unlock()

	simulating := visibleMoves >= 0

	// Track tool position for simulation marker
	var toolX, toolY float32
	toolVisible := false

	// Draw toolpath lines
	for i, m := range gp.moves {
		fromX := float32(m.FromX)*scale + offsetX
		fromY := float32(m.FromY)*scale + offsetY
		toX := float32(m.ToX)*scale + offsetX
		toY := float32(m.ToY)*scale + offsetY

		dx := m.ToX - m.FromX
		dy := m.ToY - m.FromY
		xyDist := math.Sqrt(dx*dx + dy*dy)

		// Determine if this move is completed, current, or remaining
		isCompleted := !simulating || i < visibleMoves
		isCurrent := simulating && i == visibleMoves

		if isCurrent {
			toolX = toX
			toolY = toY
			toolVisible = true
		}

		switch m.Type {
		case gcode.MoveRapid:
			if xyDist < 0.01 {
				continue
			}
			var lineColor color.NRGBA
			if isCompleted {
				lineColor = colorDoneRap
			} else if simulating {
				lineColor = colorDimRap
			} else {
				lineColor = colorRapid
			}
			line := canvas.NewLine(lineColor)
			line.StrokeWidth = 1
			line.Position1 = fyne.NewPos(fromX, fromY)
			line.Position2 = fyne.NewPos(toX, toY)
			r.objects = append(r.objects, line)

			if !simulating {
				r.drawDashedOverlay(fromX, fromY, toX, toY, colorRapid)
			}

		case gcode.MoveFeed:
			if xyDist < 0.01 {
				continue
			}
			var lineColor color.NRGBA
			if isCompleted {
				lineColor = colorDoneFd
			} else if simulating {
				lineColor = colorDimFeed
			} else {
				lineColor = colorFeed
			}
			strokeW := float32(2)
			if isCompleted && simulating {
				strokeW = 2.5
			}
			line := canvas.NewLine(lineColor)
			line.StrokeWidth = strokeW
			line.Position1 = fyne.NewPos(fromX, fromY)
			line.Position2 = fyne.NewPos(toX, toY)
			r.objects = append(r.objects, line)

		case gcode.MovePlunge:
			markerColor := colorPlunge
			if simulating && !isCompleted {
				markerColor = color.NRGBA{R: 50, G: 200, B: 50, A: 60}
			}
			marker := canvas.NewCircle(markerColor)
			markerSize := float32(4)
			marker.Resize(fyne.NewSize(markerSize, markerSize))
			marker.Move(fyne.NewPos(fromX-markerSize/2, fromY-markerSize/2))
			r.objects = append(r.objects, marker)

		case gcode.MoveRetract:
			if xyDist < 0.01 {
				markerColor := colorRetract
				if simulating && !isCompleted {
					markerColor = color.NRGBA{R: 180, G: 180, B: 0, A: 50}
				}
				marker := canvas.NewCircle(markerColor)
				markerSize := float32(3)
				marker.Resize(fyne.NewSize(markerSize, markerSize))
				marker.Move(fyne.NewPos(fromX-markerSize/2, fromY-markerSize/2))
				r.objects = append(r.objects, marker)
			} else {
				lineColor := colorRetract
				if simulating && !isCompleted {
					lineColor = color.NRGBA{R: 180, G: 180, B: 0, A: 50}
				}
				line := canvas.NewLine(lineColor)
				line.StrokeWidth = 1
				line.Position1 = fyne.NewPos(fromX, fromY)
				line.Position2 = fyne.NewPos(toX, toY)
				r.objects = append(r.objects, line)
			}
		}
	}

	// Draw tool position marker on top of everything
	if toolVisible {
		// Outer ring
		outerSize := float32(12)
		outer := canvas.NewCircle(color.Transparent)
		outer.StrokeColor = colorToolPos
		outer.StrokeWidth = 2
		outer.Resize(fyne.NewSize(outerSize, outerSize))
		outer.Move(fyne.NewPos(toolX-outerSize/2, toolY-outerSize/2))
		r.objects = append(r.objects, outer)

		// Inner dot
		innerSize := float32(4)
		inner := canvas.NewCircle(colorToolPos)
		inner.Resize(fyne.NewSize(innerSize, innerSize))
		inner.Move(fyne.NewPos(toolX-innerSize/2, toolY-innerSize/2))
		r.objects = append(r.objects, inner)
	}
}

// drawDashedOverlay adds alternating gaps along a rapid move line for dashed appearance.
func (r *gcodePreviewRenderer) drawDashedOverlay(x1, y1, x2, y2 float32, col color.NRGBA) {
	dx := x2 - x1
	dy := y2 - y1
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 8 {
		return
	}

	dashLen := float32(6)
	gapLen := float32(4)
	nx := dx / length
	ny := dy / length

	bgColor := colorSheet
	cursor := dashLen
	for cursor+gapLen < length {
		gx1 := x1 + nx*cursor
		gy1 := y1 + ny*cursor
		gx2 := x1 + nx*(cursor+gapLen)
		gy2 := y1 + ny*(cursor+gapLen)

		gap := canvas.NewLine(bgColor)
		gap.StrokeWidth = 2.5
		gap.Position1 = fyne.NewPos(gx1, gy1)
		gap.Position2 = fyne.NewPos(gx2, gy2)
		r.objects = append(r.objects, gap)

		cursor += dashLen + gapLen
	}
	_ = col
}

// drawTabMarkers draws small orange rectangles where holding tabs are positioned.
func (r *gcodePreviewRenderer) drawTabMarkers(scale, offsetX, offsetY float32) {
	settings := r.gp.settings
	tabW := float32(settings.PartTabWidth) * scale
	tabMarkerH := float32(3)

	for _, p := range r.gp.placements {
		toolR := settings.ToolDiameter / 2.0
		pw := p.PlacedWidth() + settings.ToolDiameter
		ph := p.PlacedHeight() + settings.ToolDiameter
		x0 := float32(p.X-toolR)*scale + offsetX
		y0 := float32(p.Y-toolR)*scale + offsetY

		for side := 0; side < 4; side++ {
			var sideLen float64
			if side == 0 || side == 2 {
				sideLen = pw
			} else {
				sideLen = ph
			}
			spacing := sideLen / float64(settings.PartTabsPerSide+1)

			for t := 1; t <= settings.PartTabsPerSide; t++ {
				pos := float32(spacing*float64(t)) * scale
				var tx, ty, tw, th float32

				switch side {
				case 0:
					tx = x0 + pos - tabW/2
					ty = y0 - tabMarkerH/2
					tw = tabW
					th = tabMarkerH
				case 1:
					tx = x0 + float32(pw)*scale - tabMarkerH/2
					ty = y0 + pos - tabW/2
					tw = tabMarkerH
					th = tabW
				case 2:
					tx = x0 + float32(pw)*scale - pos - tabW/2
					ty = y0 + float32(ph)*scale - tabMarkerH/2
					tw = tabW
					th = tabMarkerH
				case 3:
					tx = x0 - tabMarkerH/2
					ty = y0 + float32(ph)*scale - pos - tabW/2
					tw = tabMarkerH
					th = tabW
				}

				tabRect := canvas.NewRectangle(colorTab)
				tabRect.Resize(fyne.NewSize(tw, th))
				tabRect.Move(fyne.NewPos(tx, ty))
				r.objects = append(r.objects, tabRect)
			}
		}
	}
}

func (r *gcodePreviewRenderer) Layout(size fyne.Size) {}
func (r *gcodePreviewRenderer) Refresh() {
	r.gp.mu.Lock()
	vm := r.gp.visibleMoves
	r.gp.mu.Unlock()
	if r.built && vm == r.lastVisibleMoves {
		return
	}
	r.rebuild()
}
func (r *gcodePreviewRenderer) Destroy()                     {}
func (r *gcodePreviewRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *gcodePreviewRenderer) MinSize() fyne.Size {
	gp := r.gp
	stockW := float32(gp.sheetW)
	stockH := float32(gp.sheetH)
	if stockW <= 0 || stockH <= 0 {
		return fyne.NewSize(100, 100)
	}

	margin := float32(gp.settings.ToolDiameter) + 10
	scaleX := (gp.maxWidth - margin*2) / stockW
	scaleY := (gp.maxHeight - margin*2) / stockH
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	if scale <= 0 {
		scale = 1
	}

	return fyne.NewSize(stockW*scale+margin*2, stockH*scale+margin*2)
}

// RenderGCodePreview creates a complete preview panel for a sheet's GCode output.
func RenderGCodePreview(sheet model.SheetResult, settings model.CutSettings, gcodeStr string) fyne.CanvasObject {
	moves := gcode.ParseGCode(gcodeStr)

	preview := NewGCodePreview(
		moves,
		sheet.Placements,
		settings,
		sheet.Stock.Width,
		sheet.Stock.Height,
		700, 450,
	)

	return preview
}

// SimulationSpeed defines playback speed multipliers.
var simulationSpeeds = []struct {
	Label      string
	Multiplier float64
}{
	{"0.25x", 0.25},
	{"0.5x", 0.5},
	{"1x", 1.0},
	{"2x", 2.0},
	{"4x", 4.0},
	{"8x", 8.0},
	{"16x", 16.0},
}

// RenderGCodeSimulation creates a GCode preview panel with full simulation controls:
// a progress slider, play/pause button, step forward/backward, speed control,
// loop toggle, coordinate display, and move counter. Completed toolpath is shown
// in green, remaining in dim colors, with a red crosshair showing current tool position.
func RenderGCodeSimulation(sheet model.SheetResult, settings model.CutSettings, gcodeStr string) fyne.CanvasObject {
	moves := gcode.ParseGCode(gcodeStr)
	if len(moves) == 0 {
		return widget.NewLabel("No toolpath moves found in GCode.")
	}

	preview := NewGCodePreview(
		moves,
		sheet.Placements,
		settings,
		sheet.Stock.Width,
		sheet.Stock.Height,
		700, 450,
	)

	totalMoves := preview.MoveCount()

	// Start in "show all" mode (non-simulation)
	preview.SetVisibleMoves(-1)

	// State for playback
	var playMu sync.Mutex
	playing := false
	loopEnabled := false
	var stopChan chan struct{}
	speedIdx := 2 // default 1x

	// UI elements
	moveLabel := widget.NewLabel(fmt.Sprintf("Move: %d / %d", totalMoves, totalMoves))
	moveLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Coordinate display showing current tool position and move info
	coordLabel := widget.NewLabel("X: --  Y: --  Z: --  F: --  Type: --")
	coordLabel.TextStyle = fyne.TextStyle{Monospace: true}

	slider := widget.NewSlider(0, float64(totalMoves))
	slider.Value = float64(totalMoves)
	slider.Step = 1

	// Speed selector
	speedNames := make([]string, len(simulationSpeeds))
	for i, s := range simulationSpeeds {
		speedNames[i] = s.Label
	}
	speedSelect := widget.NewSelect(speedNames, func(selected string) {
		for i, s := range simulationSpeeds {
			if s.Label == selected {
				playMu.Lock()
				speedIdx = i
				playMu.Unlock()
				break
			}
		}
	})
	speedSelect.SetSelected("1x")

	var playBtn *widget.Button

	updateCoordDisplay := func(pos int) {
		if pos <= 0 || pos > totalMoves {
			coordLabel.SetText("X: --  Y: --  Z: --  F: --  Type: --")
			return
		}
		info := preview.GetMoveInfo(pos - 1)
		if info != nil {
			coordLabel.SetText(fmt.Sprintf("X: %.2f  Y: %.2f  Z: %.2f  F: %.0f  Type: %s",
				info.ToX, info.ToY, info.ToZ, info.FeedRate, info.Type))
		}
	}

	updateDisplay := func(pos int) {
		if pos >= totalMoves {
			preview.SetVisibleMoves(-1) // show all
			moveLabel.SetText(fmt.Sprintf("Move: %d / %d", totalMoves, totalMoves))
		} else {
			preview.SetVisibleMoves(pos)
			moveLabel.SetText(fmt.Sprintf("Move: %d / %d", pos, totalMoves))
		}
		updateCoordDisplay(pos)
	}

	stopPlayback := func() {
		playMu.Lock()
		defer playMu.Unlock()
		if playing {
			playing = false
			close(stopChan)
		}
		if playBtn != nil {
			playBtn.SetIcon(theme.MediaPlayIcon())
			playBtn.SetText("Play")
		}
	}

	startPlayback := func() {
		playMu.Lock()
		if playing {
			playMu.Unlock()
			return
		}
		playing = true
		stopChan = make(chan struct{})
		ch := stopChan
		playMu.Unlock()

		if playBtn != nil {
			playBtn.SetIcon(theme.MediaPauseIcon())
			playBtn.SetText("Pause")
		}

		go func() {
			pos := int(slider.Value)
			if pos >= totalMoves {
				pos = 0
			}

			for {
				for pos < totalMoves {
					select {
					case <-ch:
						return
					default:
					}

					pos++
					slider.SetValue(float64(pos))
					updateDisplay(pos)

					playMu.Lock()
					spd := simulationSpeeds[speedIdx].Multiplier
					playMu.Unlock()

					// Base interval: 50ms at 1x speed
					interval := time.Duration(float64(50*time.Millisecond) / spd)
					if interval < time.Millisecond {
						interval = time.Millisecond
					}
					time.Sleep(interval)
				}

				// Check if looping
				playMu.Lock()
				shouldLoop := loopEnabled
				playMu.Unlock()

				if !shouldLoop {
					break
				}

				// Reset to beginning for next loop iteration
				pos = 0
				slider.SetValue(0)
				updateDisplay(0)
				time.Sleep(500 * time.Millisecond) // Brief pause between loops
			}

			// Reached end (no loop)
			playMu.Lock()
			playing = false
			playMu.Unlock()
			if playBtn != nil {
				playBtn.SetIcon(theme.MediaPlayIcon())
				playBtn.SetText("Play")
			}
		}()
	}

	slider.OnChanged = func(val float64) {
		updateDisplay(int(val))
	}

	playBtn = widget.NewButtonWithIcon("Play", theme.MediaPlayIcon(), func() {
		playMu.Lock()
		isPlaying := playing
		playMu.Unlock()

		if isPlaying {
			stopPlayback()
		} else {
			startPlayback()
		}
	})

	stopBtn := widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() {
		stopPlayback()
		slider.SetValue(0)
		updateDisplay(0)
	})

	stepBackBtn := widget.NewButtonWithIcon("", theme.MediaSkipPreviousIcon(), func() {
		stopPlayback()
		pos := int(slider.Value) - 1
		if pos < 0 {
			pos = 0
		}
		slider.SetValue(float64(pos))
		updateDisplay(pos)
	})

	stepFwdBtn := widget.NewButtonWithIcon("", theme.MediaSkipNextIcon(), func() {
		stopPlayback()
		pos := int(slider.Value) + 1
		if pos > totalMoves {
			pos = totalMoves
		}
		slider.SetValue(float64(pos))
		updateDisplay(pos)
	})

	resetBtn := widget.NewButtonWithIcon("Show All", theme.ViewRestoreIcon(), func() {
		stopPlayback()
		slider.SetValue(float64(totalMoves))
		updateDisplay(totalMoves)
	})

	// Loop toggle
	loopCheck := widget.NewCheck("Loop", func(checked bool) {
		playMu.Lock()
		loopEnabled = checked
		playMu.Unlock()
	})

	// Layout: controls below the preview
	controls := container.NewVBox(
		slider,
		container.NewHBox(
			stepBackBtn,
			playBtn,
			stopBtn,
			stepFwdBtn,
			widget.NewSeparator(),
			widget.NewLabel("Speed:"),
			speedSelect,
			loopCheck,
			layout.NewSpacer(),
			moveLabel,
			resetBtn,
		),
		coordLabel,
	)

	return container.NewBorder(nil, controls, nil, nil, preview)
}
