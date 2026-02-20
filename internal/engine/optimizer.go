package engine

import (
	"sort"

	"github.com/piwi3910/cnc-calculator/internal/model"
)

// Optimizer runs the 2D bin-packing algorithm.
type Optimizer struct {
	Settings model.CutSettings
}

func New(settings model.CutSettings) *Optimizer {
	return &Optimizer{Settings: settings}
}

// Optimize takes parts and stock sheets, returns an optimized layout.
// Uses a guillotine-based shelf algorithm with best-fit decreasing heuristic.
func (o *Optimizer) Optimize(parts []model.Part, stocks []model.StockSheet) model.OptimizeResult {
	// Expand parts by quantity into individual placement candidates
	var expanded []model.Part
	for _, p := range parts {
		for i := 0; i < p.Quantity; i++ {
			cp := p
			cp.Quantity = 1
			expanded = append(expanded, cp)
		}
	}

	// Sort by area descending (largest first = better packing)
	sort.Slice(expanded, func(i, j int) bool {
		ai := expanded[i].Width * expanded[i].Height
		aj := expanded[j].Width * expanded[j].Height
		return ai > aj
	})

	// Build available stock pool
	var stockPool []model.StockSheet
	for _, s := range stocks {
		for i := 0; i < s.Quantity; i++ {
			cp := s
			cp.Quantity = 1
			stockPool = append(stockPool, cp)
		}
	}

	result := model.OptimizeResult{}
	remaining := expanded

	for len(remaining) > 0 && len(stockPool) > 0 {
		// Pick the largest available stock sheet
		bestStockIdx := o.selectBestStock(stockPool, remaining)
		if bestStockIdx < 0 {
			break
		}

		stock := stockPool[bestStockIdx]
		// Remove used stock
		stockPool = append(stockPool[:bestStockIdx], stockPool[bestStockIdx+1:]...)

		sheet := model.SheetResult{Stock: stock}
		var unplaced []model.Part

		// Get stock tab configuration
		tabConfig := stock.Tabs
		if !tabConfig.Enabled {
			tabConfig = o.Settings.StockTabs
		}

		// Calculate free rectangles (initial space minus edge trim and stock tabs)
		freeRects := o.calculateFreeRects(stock, tabConfig)

		packer := newGuillotinePackerWithRects(freeRects, o.Settings.KerfWidth)

		for _, part := range remaining {
			placed := false

			// Try original orientation
			if ok, x, y := packer.insert(part.Width, part.Height); ok {
				sheet.Placements = append(sheet.Placements, model.Placement{
					Part:    part,
					X:       x,
					Y:       y,
					Rotated: false,
				})
				placed = true
			}

			// Try rotated (if grain allows)
			if !placed && part.Grain == model.GrainNone {
				if ok, x, y := packer.insert(part.Height, part.Width); ok {
					sheet.Placements = append(sheet.Placements, model.Placement{
						Part:    part,
						X:       x,
						Y:       y,
						Rotated: true,
					})
					placed = true
				}
			}

			if !placed {
				unplaced = append(unplaced, part)
			}
		}

		if len(sheet.Placements) > 0 {
			result.Sheets = append(result.Sheets, sheet)
		}
		remaining = unplaced
	}

	result.UnplacedParts = remaining
	return result
}

// calculateFreeRects computes the initial free rectangles for packing,
// accounting for edge trim and stock tab exclusion zones.
func (o *Optimizer) calculateFreeRects(stock model.StockSheet, tabConfig model.StockTabConfig) []rect {
	// Start with full sheet minus edge trim
	baseRect := rect{
		x: o.Settings.EdgeTrim,
		y: o.Settings.EdgeTrim,
		w: stock.Width - 2*o.Settings.EdgeTrim,
		h: stock.Height - 2*o.Settings.EdgeTrim,
	}

	// If no tabs enabled, return single rect
	if !tabConfig.Enabled {
		return []rect{baseRect}
	}

	// Get exclusion zones
	var exclusions []model.TabZone
	if tabConfig.AdvancedMode {
		exclusions = tabConfig.CustomZones
	} else {
		// Simple mode: convert padding to exclusion zones
		if tabConfig.TopPadding > 0 {
			exclusions = append(exclusions, model.TabZone{
				X:      0,
				Y:      0,
				Width:  stock.Width,
				Height: tabConfig.TopPadding,
			})
		}
		if tabConfig.BottomPadding > 0 {
			exclusions = append(exclusions, model.TabZone{
				X:      0,
				Y:      stock.Height - tabConfig.BottomPadding,
				Width:  stock.Width,
				Height: tabConfig.BottomPadding,
			})
		}
		if tabConfig.LeftPadding > 0 {
			exclusions = append(exclusions, model.TabZone{
				X:      0,
				Y:      0,
				Width:  tabConfig.LeftPadding,
				Height: stock.Height,
			})
		}
		if tabConfig.RightPadding > 0 {
			exclusions = append(exclusions, model.TabZone{
				X:      stock.Width - tabConfig.RightPadding,
				Y:      0,
				Width:  tabConfig.RightPadding,
				Height: stock.Height,
			})
		}
	}

	// Subtract exclusions from base rect to get free rectangles
	return o.subtractExclusions(baseRect, exclusions)
}

// subtractExclusions subtracts exclusion zones from a base rectangle,
// returning a list of remaining free rectangles.
func (o *Optimizer) subtractExclusions(base rect, exclusions []model.TabZone) []rect {
	freeRects := []rect{base}

	for _, excl := range exclusions {
		var newFree []rect
		exclRect := rect{x: excl.X, y: excl.Y, w: excl.Width, h: excl.Height}

		for _, free := range freeRects {
			newFree = append(newFree, o.subtractRect(free, exclRect)...)
		}

		if len(newFree) > 0 {
			freeRects = newFree
		}
	}

	// Filter out tiny rects (smaller than minimum practical size)
	var result []rect
	for _, r := range freeRects {
		if r.w > 1 && r.h > 1 { // Minimum 1mm
			result = append(result, r)
		}
	}

	return result
}

// subtractRect subtracts one rectangle from another, returning up to 4 rectangles.
// This is a classic rectangle subtraction operation.
func (o *Optimizer) subtractRect(base, sub rect) []rect {
	// Check if rectangles intersect
	if !o.intersects(base, sub) {
		return []rect{base}
	}

	var result []rect

	// The intersection area
	intersect := rect{
		x: max(base.x, sub.x),
		y: max(base.y, sub.y),
		w: min(base.x+base.w, sub.x+sub.w) - max(base.x, sub.x),
		h: min(base.y+base.h, sub.y+sub.h) - max(base.y, sub.y),
	}

	if intersect.w <= 0 || intersect.h <= 0 {
		return []rect{base}
	}

	// Split base around intersection
	// Left portion
	if intersect.x > base.x {
		result = append(result, rect{
			x: base.x,
			y: base.y,
			w: intersect.x - base.x,
			h: base.h,
		})
	}

	// Right portion
	rightEnd := base.x + base.w
	intersectRight := intersect.x + intersect.w
	if intersectRight < rightEnd {
		result = append(result, rect{
			x: intersectRight,
			y: base.y,
			w: rightEnd - intersectRight,
			h: base.h,
		})
	}

	// Top portion (between left and right)
	if intersect.y > base.y {
		left := max(base.x, intersect.x)
		right := min(base.x+base.w, intersectRight)
		result = append(result, rect{
			x: left,
			y: base.y,
			w: right - left,
			h: intersect.y - base.y,
		})
	}

	// Bottom portion
	bottomEnd := base.y + base.h
	intersectBottom := intersect.y + intersect.h
	if intersectBottom < bottomEnd {
		left := max(base.x, intersect.x)
		right := min(base.x+base.w, intersectRight)
		result = append(result, rect{
			x: left,
			y: intersectBottom,
			w: right - left,
			h: bottomEnd - intersectBottom,
		})
	}

	return result
}

func (o *Optimizer) intersects(r1, r2 rect) bool {
	return r1.x < r2.x+r2.w && r1.x+r1.w > r2.x &&
		r1.y < r2.y+r2.h && r1.y+r1.h > r2.y
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// selectBestStock finds the best stock sheet for the remaining parts.
// Uses a trial-packing heuristic: for each candidate stock that can fit the
// largest remaining part, it runs a quick packing simulation and picks the
// stock that yields the highest material efficiency. This minimizes waste
// when multiple stock sizes are available (e.g., large 2440x1220 and small
// 1220x610 sheets).
func (o *Optimizer) selectBestStock(stocks []model.StockSheet, parts []model.Part) int {
	if len(stocks) == 0 || len(parts) == 0 {
		return -1
	}

	// Find the largest remaining part
	var largestPart *model.Part
	maxPartArea := 0.0
	for i := range parts {
		area := parts[i].Width * parts[i].Height
		if area > maxPartArea {
			maxPartArea = area
			largestPart = &parts[i]
		}
	}

	usableWidth := func(s model.StockSheet) float64 {
		return s.Width - 2*o.Settings.EdgeTrim
	}
	usableHeight := func(s model.StockSheet) float64 {
		return s.Height - 2*o.Settings.EdgeTrim
	}

	// Find stocks that can fit the largest part (considering rotation)
	var candidates []int
	for i, stock := range stocks {
		uw := usableWidth(stock)
		uh := usableHeight(stock)
		kerf := o.Settings.KerfWidth

		fitsNormal := largestPart.Width+kerf <= uw && largestPart.Height+kerf <= uh
		fitsRotated := largestPart.Grain == model.GrainNone &&
			largestPart.Height+kerf <= uw && largestPart.Width+kerf <= uh

		if fitsNormal || fitsRotated {
			candidates = append(candidates, i)
		}
	}

	// If no stock can fit the largest part, return -1 to signal that we
	// cannot place this part on any available stock.
	if len(candidates) == 0 {
		return -1
	}

	// If only one candidate, skip trial packing.
	if len(candidates) == 1 {
		return candidates[0]
	}

	// De-duplicate candidates by stock dimensions to avoid redundant trials.
	// Multiple sheets of the same size would produce identical packing results.
	type stockKey struct {
		w, h float64
	}
	seen := make(map[stockKey]bool)
	var uniqueCandidates []int
	for _, idx := range candidates {
		key := stockKey{stocks[idx].Width, stocks[idx].Height}
		if !seen[key] {
			seen[key] = true
			uniqueCandidates = append(uniqueCandidates, idx)
		}
	}

	// Trial-pack on each unique candidate and measure efficiency.
	bestIdx := -1
	bestScore := -1.0

	for _, idx := range uniqueCandidates {
		stock := stocks[idx]
		tabConfig := stock.Tabs
		if !tabConfig.Enabled {
			tabConfig = o.Settings.StockTabs
		}
		freeRects := o.calculateFreeRects(stock, tabConfig)
		packer := newGuillotinePackerWithRects(freeRects, o.Settings.KerfWidth)

		placedArea := 0.0
		for _, part := range parts {
			placed := false
			if ok, _, _ := packer.insert(part.Width, part.Height); ok {
				placedArea += part.Width * part.Height
				placed = true
			}
			if !placed && part.Grain == model.GrainNone {
				if ok, _, _ := packer.insert(part.Height, part.Width); ok {
					placedArea += part.Width * part.Height
				}
			}
		}

		stockArea := stock.Width * stock.Height
		if stockArea == 0 {
			continue
		}
		efficiency := placedArea / stockArea

		if efficiency > bestScore {
			bestScore = efficiency
			bestIdx = idx
		}
	}

	if bestIdx < 0 {
		return candidates[0]
	}
	return bestIdx
}

// guillotinePacker implements the guillotine bin-packing algorithm.
// It maintains a list of free rectangles and splits them on each insertion.
type guillotinePacker struct {
	freeRects []rect
	kerf      float64
}

type rect struct {
	x, y, w, h float64
}

func newGuillotinePacker(width, height, kerf float64) *guillotinePacker {
	return &guillotinePacker{
		freeRects: []rect{{0, 0, width, height}},
		kerf:      kerf,
	}
}

// newGuillotinePackerWithRects creates a packer with predefined initial free rectangles.
// This is used when there are exclusion zones (like stock tabs).
func newGuillotinePackerWithRects(initialRects []rect, kerf float64) *guillotinePacker {
	return &guillotinePacker{
		freeRects: initialRects,
		kerf:      kerf,
	}
}

// insert tries to place a part of given dimensions. Returns success and position.
// Uses Best Area Fit (BAF) heuristic.
func (gp *guillotinePacker) insert(w, h float64) (bool, float64, float64) {
	bestIdx := -1
	bestAreaFit := float64(-1)
	wk := w + gp.kerf
	hk := h + gp.kerf

	for i, r := range gp.freeRects {
		if wk <= r.w+0.001 && hk <= r.h+0.001 {
			areaFit := (r.w * r.h) - (w * h)
			if bestIdx < 0 || areaFit < bestAreaFit {
				bestIdx = i
				bestAreaFit = areaFit
			}
		}
	}

	if bestIdx < 0 {
		return false, 0, 0
	}

	chosen := gp.freeRects[bestIdx]
	px, py := chosen.x, chosen.y

	// Remove chosen rect
	gp.freeRects = append(gp.freeRects[:bestIdx], gp.freeRects[bestIdx+1:]...)

	// Split the remaining space (guillotine split).
	// Choose split axis that maximizes the larger remaining rectangle.
	rightW := chosen.w - wk
	bottomH := chosen.h - hk

	// Shorter leftover axis split — tends to produce better results
	if rightW*chosen.h > chosen.w*bottomH {
		// Split vertically: right remainder gets full height
		if rightW > 0.001 {
			gp.freeRects = append(gp.freeRects, rect{
				x: chosen.x + wk,
				y: chosen.y,
				w: rightW,
				h: chosen.h,
			})
		}
		if bottomH > 0.001 {
			gp.freeRects = append(gp.freeRects, rect{
				x: chosen.x,
				y: chosen.y + hk,
				w: wk,
				h: bottomH,
			})
		}
	} else {
		// Split horizontally: bottom remainder gets full width
		if bottomH > 0.001 {
			gp.freeRects = append(gp.freeRects, rect{
				x: chosen.x,
				y: chosen.y + hk,
				w: chosen.w,
				h: bottomH,
			})
		}
		if rightW > 0.001 {
			gp.freeRects = append(gp.freeRects, rect{
				x: chosen.x + wk,
				y: chosen.y,
				w: rightW,
				h: hk,
			})
		}
	}

	return true, px, py
}
