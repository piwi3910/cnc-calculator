package widgets

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
)

// GCodePreview is a custom Fyne widget that renders a visual preview
// of GCode toolpath movements overlaid on a stock sheet with part outlines.
type GCodePreview struct {
	widget.BaseWidget
	moves      []gcode.GCodeMove
	placements []model.Placement
	settings   model.CutSettings
	sheetW     float64
	sheetH     float64
	maxWidth   float32
	maxHeight  float32
}

// NewGCodePreview creates a new GCode preview widget.
func NewGCodePreview(moves []gcode.GCodeMove, placements []model.Placement, settings model.CutSettings, sheetW, sheetH float64, maxW, maxH float32) *GCodePreview {
	gp := &GCodePreview{
		moves:      moves,
		placements: placements,
		settings:   settings,
		sheetW:     sheetW,
		sheetH:     sheetH,
		maxWidth:   maxW,
		maxHeight:  maxH,
	}
	gp.ExtendBaseWidget(gp)
	return gp
}

// CreateRenderer implements fyne.Widget.
func (gp *GCodePreview) CreateRenderer() fyne.WidgetRenderer {
	return newGCodePreviewRenderer(gp)
}

type gcodePreviewRenderer struct {
	gp      *GCodePreview
	objects []fyne.CanvasObject
}

func newGCodePreviewRenderer(gp *GCodePreview) *gcodePreviewRenderer {
	r := &gcodePreviewRenderer{gp: gp}
	r.rebuild()
	return r
}

func (r *gcodePreviewRenderer) rebuild() {
	r.objects = nil

	gp := r.gp
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

	// Stock border
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.NRGBA{R: 80, G: 80, B: 80, A: 255}
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

		// Part label
		if pw > 40 && ph > 18 {
			label := canvas.NewText(p.Part.Label, color.NRGBA{R: 50, G: 70, B: 120, A: 200})
			label.TextSize = 10
			label.Move(fyne.NewPos(px+3, py+2))
			r.objects = append(r.objects, label)
		}
	}

	// Draw tab markers if tabs are configured
	if gp.settings.PartTabsPerSide > 0 {
		r.drawTabMarkers(scale, offsetX, offsetY)
	}

	// Draw toolpath lines
	for _, m := range gp.moves {
		fromX := float32(m.FromX)*scale + offsetX
		fromY := float32(m.FromY)*scale + offsetY
		toX := float32(m.ToX)*scale + offsetX
		toY := float32(m.ToY)*scale + offsetY

		// Skip zero-length moves and pure Z-only moves (plunge/retract shown as markers)
		dx := m.ToX - m.FromX
		dy := m.ToY - m.FromY
		xyDist := math.Sqrt(dx*dx + dy*dy)

		switch m.Type {
		case gcode.MoveRapid:
			if xyDist < 0.01 {
				continue
			}
			line := canvas.NewLine(colorRapid)
			line.StrokeWidth = 1
			line.Position1 = fyne.NewPos(fromX, fromY)
			line.Position2 = fyne.NewPos(toX, toY)
			r.objects = append(r.objects, line)

			// Draw dashes along rapid moves for visual distinction
			r.drawDashedOverlay(fromX, fromY, toX, toY, colorRapid)

		case gcode.MoveFeed:
			if xyDist < 0.01 {
				continue
			}
			line := canvas.NewLine(colorFeed)
			line.StrokeWidth = 2
			line.Position1 = fyne.NewPos(fromX, fromY)
			line.Position2 = fyne.NewPos(toX, toY)
			r.objects = append(r.objects, line)

		case gcode.MovePlunge:
			// Draw a small downward arrow marker at plunge position
			marker := canvas.NewCircle(colorPlunge)
			markerSize := float32(4)
			marker.Resize(fyne.NewSize(markerSize, markerSize))
			marker.Move(fyne.NewPos(fromX-markerSize/2, fromY-markerSize/2))
			r.objects = append(r.objects, marker)

		case gcode.MoveRetract:
			if xyDist < 0.01 {
				// Pure Z retract: show small upward marker
				marker := canvas.NewCircle(colorRetract)
				markerSize := float32(3)
				marker.Resize(fyne.NewSize(markerSize, markerSize))
				marker.Move(fyne.NewPos(fromX-markerSize/2, fromY-markerSize/2))
				r.objects = append(r.objects, marker)
			} else {
				line := canvas.NewLine(colorRetract)
				line.StrokeWidth = 1
				line.Position1 = fyne.NewPos(fromX, fromY)
				line.Position2 = fyne.NewPos(toX, toY)
				r.objects = append(r.objects, line)
			}
		}
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

	// Overlay background-colored segments for gaps
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
	_ = col // color parameter reserved for potential future use
}

// drawTabMarkers draws small orange rectangles where holding tabs are positioned.
func (r *gcodePreviewRenderer) drawTabMarkers(scale, offsetX, offsetY float32) {
	settings := r.gp.settings
	tabW := float32(settings.PartTabWidth) * scale
	tabMarkerH := float32(3) // Fixed visual height for tab markers

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
				case 0: // bottom: along X at y0
					tx = x0 + pos - tabW/2
					ty = y0 - tabMarkerH/2
					tw = tabW
					th = tabMarkerH
				case 1: // right: along Y at x0+pw*scale
					tx = x0 + float32(pw)*scale - tabMarkerH/2
					ty = y0 + pos - tabW/2
					tw = tabMarkerH
					th = tabW
				case 2: // top: along X at y0+ph*scale
					tx = x0 + float32(pw)*scale - pos - tabW/2
					ty = y0 + float32(ph)*scale - tabMarkerH/2
					tw = tabW
					th = tabMarkerH
				case 3: // left: along Y at x0
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

func (r *gcodePreviewRenderer) Layout(size fyne.Size)        {}
func (r *gcodePreviewRenderer) Refresh()                     { r.rebuild() }
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

// RenderGCodePreview creates a complete preview panel for a sheet's GCode output,
// including the toolpath visualization and a color legend.
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
