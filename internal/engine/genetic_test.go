package engine

import (
	"testing"

	"github.com/piwi3910/cnc-calculator/internal/model"
)

func makeTestParts() []model.Part {
	return []model.Part{
		{ID: "p1", Label: "A", Width: 400, Height: 300, Quantity: 1, Grain: model.GrainNone},
		{ID: "p2", Label: "B", Width: 200, Height: 150, Quantity: 2, Grain: model.GrainNone},
		{ID: "p3", Label: "C", Width: 500, Height: 400, Quantity: 1, Grain: model.GrainNone},
	}
}

func makeTestStock() []model.StockSheet {
	return []model.StockSheet{
		{ID: "s1", Label: "Sheet", Width: 2440, Height: 1220, Quantity: 2},
	}
}

func makeTestSettings() model.CutSettings {
	s := model.DefaultSettings()
	s.Algorithm = model.AlgorithmGenetic
	s.KerfWidth = 3.0
	s.EdgeTrim = 10.0
	s.StockTabs.Enabled = false
	return s
}

func TestGeneticOptimizerPlacesAllParts(t *testing.T) {
	parts := makeTestParts()
	stocks := makeTestStock()
	settings := makeTestSettings()

	result := OptimizeGenetic(settings, parts, stocks)

	// All parts should be placed (total quantity = 1+2+1 = 4)
	totalPlaced := 0
	for _, sheet := range result.Sheets {
		totalPlaced += len(sheet.Placements)
	}

	if totalPlaced != 4 {
		t.Errorf("expected 4 parts placed, got %d", totalPlaced)
	}

	if len(result.UnplacedParts) != 0 {
		t.Errorf("expected 0 unplaced parts, got %d", len(result.UnplacedParts))
	}
}

func TestGeneticOptimizerEfficiency(t *testing.T) {
	parts := makeTestParts()
	stocks := makeTestStock()
	settings := makeTestSettings()

	result := OptimizeGenetic(settings, parts, stocks)

	eff := result.TotalEfficiency()
	if eff <= 0 {
		t.Errorf("expected positive efficiency, got %.2f%%", eff)
	}
}

func TestGeneticOptimizerRespectsGrainDirection(t *testing.T) {
	parts := []model.Part{
		{ID: "g1", Label: "GrainH", Width: 600, Height: 300, Quantity: 1, Grain: model.GrainHorizontal},
		{ID: "g2", Label: "GrainV", Width: 400, Height: 200, Quantity: 1, Grain: model.GrainVertical},
	}
	stocks := makeTestStock()
	settings := makeTestSettings()

	result := OptimizeGenetic(settings, parts, stocks)

	for _, sheet := range result.Sheets {
		for _, p := range sheet.Placements {
			if p.Part.Grain != model.GrainNone && p.Rotated {
				t.Errorf("part %s with grain %s should not be rotated", p.Part.Label, p.Part.Grain)
			}
		}
	}
}

func TestGeneticOptimizerEmptyInput(t *testing.T) {
	settings := makeTestSettings()

	// No parts
	result := OptimizeGenetic(settings, nil, makeTestStock())
	if len(result.Sheets) != 0 {
		t.Errorf("expected no sheets for empty parts, got %d", len(result.Sheets))
	}

	// No stocks
	result = OptimizeGenetic(settings, makeTestParts(), nil)
	if len(result.Sheets) != 0 {
		t.Errorf("expected no sheets for empty stocks, got %d", len(result.Sheets))
	}
}

func TestGeneticOptimizerBetterThanOrEqualToGreedy(t *testing.T) {
	// Use a problem where the GA should find at least as good a solution
	parts := []model.Part{
		{ID: "p1", Label: "A", Width: 600, Height: 400, Quantity: 3, Grain: model.GrainNone},
		{ID: "p2", Label: "B", Width: 300, Height: 200, Quantity: 4, Grain: model.GrainNone},
		{ID: "p3", Label: "C", Width: 450, Height: 350, Quantity: 2, Grain: model.GrainNone},
		{ID: "p4", Label: "D", Width: 150, Height: 100, Quantity: 6, Grain: model.GrainNone},
	}
	stocks := []model.StockSheet{
		{ID: "s1", Label: "Sheet", Width: 2440, Height: 1220, Quantity: 5},
	}

	settings := model.DefaultSettings()
	settings.KerfWidth = 3.0
	settings.EdgeTrim = 10.0
	settings.StockTabs.Enabled = false

	// Greedy result
	settings.Algorithm = model.AlgorithmGuillotine
	opt := New(settings)
	greedyResult := opt.Optimize(parts, stocks)

	// Genetic result
	geneticResult := OptimizeGenetic(settings, parts, stocks)

	greedyPlaced := 0
	for _, s := range greedyResult.Sheets {
		greedyPlaced += len(s.Placements)
	}
	geneticPlaced := 0
	for _, s := range geneticResult.Sheets {
		geneticPlaced += len(s.Placements)
	}

	// GA should place at least as many parts as greedy
	if geneticPlaced < greedyPlaced {
		t.Errorf("genetic placed %d parts, greedy placed %d - GA should do at least as well", geneticPlaced, greedyPlaced)
	}
}

func TestGeneticViaOptimizerDispatch(t *testing.T) {
	parts := makeTestParts()
	stocks := makeTestStock()
	settings := makeTestSettings()

	// Use the Optimizer dispatch path
	opt := New(settings)
	result := opt.Optimize(parts, stocks)

	totalPlaced := 0
	for _, sheet := range result.Sheets {
		totalPlaced += len(sheet.Placements)
	}

	if totalPlaced != 4 {
		t.Errorf("expected 4 parts placed via dispatch, got %d", totalPlaced)
	}
}

func TestOrderCrossoverPreservesAllGenes(t *testing.T) {
	parts := []model.Part{
		{ID: "p1", Label: "A", Width: 100, Height: 100, Quantity: 1, Grain: model.GrainNone},
		{ID: "p2", Label: "B", Width: 200, Height: 200, Quantity: 1, Grain: model.GrainNone},
		{ID: "p3", Label: "C", Width: 300, Height: 300, Quantity: 1, Grain: model.GrainNone},
		{ID: "p4", Label: "D", Width: 400, Height: 400, Quantity: 1, Grain: model.GrainNone},
		{ID: "p5", Label: "E", Width: 500, Height: 500, Quantity: 1, Grain: model.GrainNone},
	}
	stocks := makeTestStock()
	settings := makeTestSettings()

	ga := newGeneticOptimizer(settings, DefaultGeneticConfig(), parts, stocks, 123)

	parent1 := chromosome{genes: []gene{
		{partIndex: 0}, {partIndex: 1}, {partIndex: 2}, {partIndex: 3}, {partIndex: 4},
	}}
	parent2 := chromosome{genes: []gene{
		{partIndex: 4}, {partIndex: 3}, {partIndex: 2}, {partIndex: 1}, {partIndex: 0},
	}}

	child := ga.orderCrossover(parent1, parent2)

	if len(child.genes) != 5 {
		t.Fatalf("expected 5 genes, got %d", len(child.genes))
	}

	seen := make(map[int]bool)
	for _, g := range child.genes {
		if seen[g.partIndex] {
			t.Errorf("duplicate part index %d in child", g.partIndex)
		}
		seen[g.partIndex] = true
	}

	for i := 0; i < 5; i++ {
		if !seen[i] {
			t.Errorf("missing part index %d in child", i)
		}
	}
}

func TestGeneticOptimizerPartTooLargeForStock(t *testing.T) {
	parts := []model.Part{
		{ID: "big", Label: "TooBig", Width: 5000, Height: 3000, Quantity: 1, Grain: model.GrainNone},
	}
	stocks := []model.StockSheet{
		{ID: "s1", Label: "Small", Width: 1000, Height: 500, Quantity: 1},
	}
	settings := makeTestSettings()

	result := OptimizeGenetic(settings, parts, stocks)

	if len(result.UnplacedParts) != 1 {
		t.Errorf("expected 1 unplaced part, got %d", len(result.UnplacedParts))
	}
}
