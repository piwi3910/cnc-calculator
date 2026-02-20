package engine

import (
	"math"
	"sort"

	"github.com/piwi3910/SlabCut/internal/model"
)

// Optimizer runs the 2D bin-packing algorithm.
type Optimizer struct {
	Settings model.CutSettings
}

func New(settings model.CutSettings) *Optimizer {
	return &Optimizer{Settings: settings}
}

// Optimize takes parts and stock sheets, returns an optimized layout.
// When parts and stocks have material types set, optimization is performed
// per material group: parts with a specific material are only placed on
// stocks of the same material. Parts or stocks with empty material are
// treated as universal (compatible with anything).
func (o *Optimizer) Optimize(parts []model.Part, stocks []model.StockSheet) model.OptimizeResult {
	// Group parts and stocks by material for multi-material optimization
	groups := groupByMaterial(parts, stocks)

	combined := model.OptimizeResult{}
	for _, g := range groups {
		var groupResult model.OptimizeResult
		if o.Settings.Algorithm == model.AlgorithmGenetic {
			groupResult = OptimizeGenetic(o.Settings, g.parts, g.stocks)
		} else {
			groupResult = o.optimizeGuillotine(g.parts, g.stocks)
		}
		combined.Sheets = append(combined.Sheets, groupResult.Sheets...)
		combined.UnplacedParts = append(combined.UnplacedParts, groupResult.UnplacedParts...)
	}
	return combined
}

// materialGroup holds parts and stocks for a single material type.
type materialGroup struct {
	material string
	parts    []model.Part
	stocks   []model.StockSheet
}

// groupByMaterial splits parts and stocks into groups by material type.
// Parts with an empty material can go on any stock, so they are added to
// every group. Stocks with an empty material can accept any part, so they
// are added to every group. If no materials are specified at all, everything
// goes into one group.
func groupByMaterial(parts []model.Part, stocks []model.StockSheet) []materialGroup {
	// Collect all unique non-empty material names
	materialSet := make(map[string]bool)
	for _, p := range parts {
		if p.Material != "" {
			materialSet[p.Material] = true
		}
	}
	for _, s := range stocks {
		if s.Material != "" {
			materialSet[s.Material] = true
		}
	}

	// If no materials specified, return a single group with everything
	if len(materialSet) == 0 {
		return []materialGroup{{parts: parts, stocks: stocks}}
	}

	materials := make([]string, 0, len(materialSet))
	for m := range materialSet {
		materials = append(materials, m)
	}
	sort.Strings(materials)

	// Build groups
	groups := make([]materialGroup, 0, len(materials))
	var universalParts []model.Part
	var universalStocks []model.StockSheet

	for _, p := range parts {
		if p.Material == "" {
			universalParts = append(universalParts, p)
		}
	}
	for _, s := range stocks {
		if s.Material == "" {
			universalStocks = append(universalStocks, s)
		}
	}

	for _, mat := range materials {
		g := materialGroup{material: mat}
		for _, p := range parts {
			if p.Material == mat {
				g.parts = append(g.parts, p)
			}
		}
		for _, s := range stocks {
			if s.Material == mat {
				g.stocks = append(g.stocks, s)
			}
		}
		// Universal stocks can be used by any material group
		g.stocks = append(g.stocks, universalStocks...)
		groups = append(groups, g)
	}

	// If there are universal parts (no material), create a group for them
	// using all stocks (universal stocks + all material stocks)
	if len(universalParts) > 0 {
		g := materialGroup{parts: universalParts}
		g.stocks = append(g.stocks, stocks...)
		groups = append(groups, g)
	}

	return groups
}

// addCutoutFreeRects injects free rectangles into the packer for interior cutout
// zones of a placed part. This allows smaller parts to be nested inside the
// waste holes of larger parts, maximizing material utilization.
func addCutoutFreeRects(packer *guillotinePacker, part model.Part, partX, partY float64, rotated bool, kerf float64) {
	cutoutRects := part.CutoutBounds()
	for _, cr := range cutoutRects {
		// Transform cutout coordinates from part-local to sheet-absolute
		var absX, absY, absW, absH float64
		if rotated {
			// When the part is rotated 90 degrees, X and Y swap
			absX = partX + cr.Y
			absY = partY + cr.X
			absW = cr.Height
			absH = cr.Width
		} else {
			absX = partX + cr.X
			absY = partY + cr.Y
			absW = cr.Width
			absH = cr.Height
		}

		// Shrink by kerf to account for the cut line around the cutout
		absX += kerf
		absY += kerf
		absW -= 2 * kerf
		absH -= 2 * kerf

		// Only add if the cutout area is large enough to be useful
		if absW > 1 && absH > 1 {
			packer.freeRects = append(packer.freeRects, rect{
				x: absX,
				y: absY,
				w: absW,
				h: absH,
			})
		}
	}
}

// tryOutlineRotations attempts to place an outline part at multiple rotation angles.
// It rotates the outline, computes the new bounding box, and tries to insert it.
// The rotation that uses the smallest bounding box area is tried first.
// Returns true if the part was placed.
func (o *Optimizer) tryOutlineRotations(packer *guillotinePacker, sheet *model.SheetResult, part model.Part, numRotations int) bool {
	if numRotations < 1 {
		numRotations = 2
	}

	type rotationCandidate struct {
		angle   float64
		outline model.Outline
		width   float64
		height  float64
		area    float64
	}

	var candidates []rotationCandidate
	angleStep := math.Pi / float64(numRotations)

	for i := 0; i < numRotations; i++ {
		angle := float64(i) * angleStep
		rotated := part.Outline.Rotate(angle)
		min, max := rotated.BoundingBox()
		w := max.X - min.X
		h := max.Y - min.Y
		if w > 0 && h > 0 {
			candidates = append(candidates, rotationCandidate{
				angle:   angle,
				outline: rotated,
				width:   w,
				height:  h,
				area:    w * h,
			})
		}
	}

	// Sort by bounding box area ascending (tightest fit first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].area < candidates[j].area
	})

	for _, c := range candidates {
		if ok, x, y := packer.insert(c.width, c.height); ok {
			// Create a copy of the part with the rotated outline and updated dimensions
			placedPart := part
			placedPart.Outline = c.outline
			placedPart.Width = c.width
			placedPart.Height = c.height

			sheet.Placements = append(sheet.Placements, model.Placement{
				Part:    placedPart,
				X:       x,
				Y:       y,
				Rotated: false, // Rotation is baked into the outline
			})
			return true
		}
	}
	return false
}

// optimizeGuillotine uses a guillotine-based shelf algorithm with best-fit decreasing heuristic.
func (o *Optimizer) optimizeGuillotine(parts []model.Part, stocks []model.StockSheet) model.OptimizeResult {
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

		// Try multiple rotation strategies and keep the best result for this sheet
		bestSheet, bestUnplaced := o.packSheetBestStrategy(stock, remaining)

		if len(bestSheet.Placements) > 0 {
			result.Sheets = append(result.Sheets, bestSheet)
		}
		remaining = bestUnplaced
	}

	result.UnplacedParts = remaining
	return result
}

// rotationStrategy controls how parts are rotated during packing.
type rotationStrategy int

const (
	rotBestFit    rotationStrategy = iota // Compare both orientations, pick tighter fit
	rotAllNormal                          // Always use normal orientation (fallback to rotated if doesn't fit)
	rotAllRotated                         // Prefer rotated for all no-grain parts (fallback to normal if doesn't fit)
)

// packSheetBestStrategy tries multiple rotation strategies and returns the best result.
func (o *Optimizer) packSheetBestStrategy(stock model.StockSheet, parts []model.Part) (model.SheetResult, []model.Part) {
	strategies := []rotationStrategy{rotBestFit, rotAllNormal, rotAllRotated}

	var bestSheet model.SheetResult
	var bestUnplaced []model.Part
	bestPlaced := -1

	for _, strat := range strategies {
		sheet, unplaced := o.packSheet(stock, parts, strat)
		placed := len(sheet.Placements)
		if placed > bestPlaced {
			bestPlaced = placed
			bestSheet = sheet
			bestUnplaced = unplaced
		} else if placed == bestPlaced && placed > 0 {
			// Same number placed — pick higher efficiency
			if sheet.Efficiency() > bestSheet.Efficiency() {
				bestSheet = sheet
				bestUnplaced = unplaced
			}
		}
	}
	return bestSheet, bestUnplaced
}

// packSheet packs parts into a single stock sheet using the given rotation strategy.
func (o *Optimizer) packSheet(stock model.StockSheet, parts []model.Part, strategy rotationStrategy) (model.SheetResult, []model.Part) {
	sheet := model.SheetResult{Stock: stock}
	var unplaced []model.Part

	tabConfig := stock.Tabs
	if !tabConfig.Enabled {
		tabConfig = o.Settings.StockTabs
	}

	freeRects := o.calculateFreeRects(stock, tabConfig)
	packer := newGuillotinePackerWithRects(freeRects, o.Settings.KerfWidth)

	for _, part := range parts {
		placed := false
		var placedX, placedY float64
		var placedRotated bool

		canNormal, canRotated := model.CanPlaceWithGrain(part.Grain, stock.Grain)

		// For outline parts with NestingRotations > 2, try multiple angles
		if len(part.Outline) > 0 && o.Settings.NestingRotations > 2 && part.Grain == model.GrainNone {
			placed = o.tryOutlineRotations(packer, &sheet, part, o.Settings.NestingRotations)
		}

		if !placed {
			switch strategy {
			case rotAllRotated:
				// Prefer rotated for no-grain parts
				if canRotated && part.Width != part.Height {
					if ok, x, y := packer.insert(part.Height, part.Width); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part: part, X: x, Y: y, Rotated: true,
						})
						placed = true
						placedX, placedY = x, y
						placedRotated = true
					}
				}
				if !placed && canNormal {
					if ok, x, y := packer.insert(part.Width, part.Height); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part: part, X: x, Y: y, Rotated: false,
						})
						placed = true
						placedX, placedY = x, y
						placedRotated = false
					}
				}

			case rotBestFit:
				// Compare both orientations and pick the tighter fit
				if canNormal && canRotated && part.Width != part.Height {
					normalFit := packer.bestFit(part.Width, part.Height)
					rotatedFit := packer.bestFit(part.Height, part.Width)

					preferRotated := false
					if normalFit < 0 && rotatedFit >= 0 {
						preferRotated = true
					} else if normalFit >= 0 && rotatedFit >= 0 && rotatedFit < normalFit {
						preferRotated = true
					}

					if preferRotated {
						if ok, x, y := packer.insert(part.Height, part.Width); ok {
							sheet.Placements = append(sheet.Placements, model.Placement{
								Part: part, X: x, Y: y, Rotated: true,
							})
							placed = true
							placedX, placedY = x, y
							placedRotated = true
						}
					} else if normalFit >= 0 {
						if ok, x, y := packer.insert(part.Width, part.Height); ok {
							sheet.Placements = append(sheet.Placements, model.Placement{
								Part: part, X: x, Y: y, Rotated: false,
							})
							placed = true
							placedX, placedY = x, y
							placedRotated = false
						}
					}
				}
				// Fallback for grain-restricted or square parts
				if !placed && canNormal {
					if ok, x, y := packer.insert(part.Width, part.Height); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part: part, X: x, Y: y, Rotated: false,
						})
						placed = true
						placedX, placedY = x, y
						placedRotated = false
					}
				}
				if !placed && canRotated {
					if ok, x, y := packer.insert(part.Height, part.Width); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part: part, X: x, Y: y, Rotated: true,
						})
						placed = true
						placedX, placedY = x, y
						placedRotated = true
					}
				}

			default: // rotAllNormal
				if canNormal {
					if ok, x, y := packer.insert(part.Width, part.Height); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part: part, X: x, Y: y, Rotated: false,
						})
						placed = true
						placedX, placedY = x, y
						placedRotated = false
					}
				}
				if !placed && canRotated {
					if ok, x, y := packer.insert(part.Height, part.Width); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part: part, X: x, Y: y, Rotated: true,
						})
						placed = true
						placedX, placedY = x, y
						placedRotated = true
					}
				}
			}
		}

		if placed && len(part.Cutouts) > 0 {
			addCutoutFreeRects(packer, part, placedX, placedY, placedRotated, o.Settings.KerfWidth)
		}

		if !placed {
			unplaced = append(unplaced, part)
		}
	}

	return sheet, unplaced
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

	// Get exclusion zones from stock tabs
	var exclusions []model.TabZone
	if tabConfig.Enabled {
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
	}

	// Add clamp zone exclusions (always applied regardless of tab config)
	for _, cz := range o.Settings.ClampZones {
		exclusions = append(exclusions, model.TabZone{
			X:      cz.X,
			Y:      cz.Y,
			Width:  cz.Width,
			Height: cz.Height,
		})
	}

	// If no exclusions at all, return the base rect directly
	if len(exclusions) == 0 {
		return []rect{baseRect}
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

	// Find stocks that can fit the largest part (considering rotation and grain)
	var candidates []int
	for i, stock := range stocks {
		uw := usableWidth(stock)
		uh := usableHeight(stock)
		kerf := o.Settings.KerfWidth

		canNormal, canRotated := model.CanPlaceWithGrain(largestPart.Grain, stock.Grain)

		fitsNormal := canNormal && largestPart.Width+kerf <= uw && largestPart.Height+kerf <= uh
		fitsRotated := canRotated &&
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
			canNormal, canRotated := model.CanPlaceWithGrain(part.Grain, stock.Grain)
			if canNormal {
				if ok, _, _ := packer.insert(part.Width, part.Height); ok {
					placedArea += part.Width * part.Height
					placed = true
				}
			}
			if !placed && canRotated {
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

	// Maximal rectangles approach: split ALL overlapping free rects around the placed piece.
	// This produces larger free areas than pure guillotine splitting, allowing parts to
	// be rotated and placed in remaining strips that span multiple previous guillotine cuts.
	placed := rect{x: px, y: py, w: wk, h: hk}
	gp.splitAroundPlacement(placed)

	return true, px, py
}

// splitAroundPlacement removes all free rects that overlap with the placed rect
// and generates maximal sub-rects from each overlap. Then prunes contained rects.
func (gp *guillotinePacker) splitAroundPlacement(placed rect) {
	var newRects []rect

	for _, r := range gp.freeRects {
		if !rectsOverlap(r, placed) {
			newRects = append(newRects, r)
			continue
		}

		// Generate up to 4 maximal sub-rects from the non-overlapping portions.
		// Left strip (full height of original rect)
		if placed.x > r.x+0.001 {
			newRects = append(newRects, rect{
				x: r.x, y: r.y,
				w: placed.x - r.x, h: r.h,
			})
		}
		// Right strip (full height of original rect)
		if placed.x+placed.w < r.x+r.w-0.001 {
			newRects = append(newRects, rect{
				x: placed.x + placed.w, y: r.y,
				w: (r.x + r.w) - (placed.x + placed.w), h: r.h,
			})
		}
		// Top strip (full width of original rect)
		if placed.y > r.y+0.001 {
			newRects = append(newRects, rect{
				x: r.x, y: r.y,
				w: r.w, h: placed.y - r.y,
			})
		}
		// Bottom strip (full width of original rect)
		if placed.y+placed.h < r.y+r.h-0.001 {
			newRects = append(newRects, rect{
				x: r.x, y: placed.y + placed.h,
				w: r.w, h: (r.y + r.h) - (placed.y + placed.h),
			})
		}
	}

	// Prune rects that are fully contained within another
	gp.freeRects = pruneContained(newRects)
}

// rectsOverlap returns true if two rectangles overlap (not just touch).
func rectsOverlap(a, b rect) bool {
	return a.x < b.x+b.w-0.001 && a.x+a.w > b.x+0.001 &&
		a.y < b.y+b.h-0.001 && a.y+a.h > b.y+0.001
}

// pruneContained removes any rect that is fully contained within another.
func pruneContained(rects []rect) []rect {
	if len(rects) <= 1 {
		return rects
	}
	kept := make([]rect, 0, len(rects))
	for i, a := range rects {
		contained := false
		for j, b := range rects {
			if i != j && containsRect(b, a) {
				contained = true
				break
			}
		}
		if !contained {
			kept = append(kept, a)
		}
	}
	return kept
}

// containsRect returns true if outer fully contains inner.
func containsRect(outer, inner rect) bool {
	return outer.x <= inner.x+0.001 && outer.y <= inner.y+0.001 &&
		outer.x+outer.w >= inner.x+inner.w-0.001 &&
		outer.y+outer.h >= inner.y+inner.h-0.001
}

// bestFit returns the area waste for inserting a piece of size w x h
// without modifying the packer state. Returns -1 if it doesn't fit.
func (gp *guillotinePacker) bestFit(w, h float64) float64 {
	wk := w + gp.kerf
	hk := h + gp.kerf
	best := float64(-1)

	for _, r := range gp.freeRects {
		if wk <= r.w+0.001 && hk <= r.h+0.001 {
			areaFit := (r.w * r.h) - (w * h)
			if best < 0 || areaFit < best {
				best = areaFit
			}
		}
	}
	return best
}
