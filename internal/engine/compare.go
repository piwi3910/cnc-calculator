package engine

import (
	"fmt"

	"github.com/piwi3910/SlabCut/internal/model"
)

// ComparisonScenario defines a named set of settings to compare.
type ComparisonScenario struct {
	Name     string
	Settings model.CutSettings
}

// ComparisonResult holds the optimization result and computed statistics
// for a single scenario.
type ComparisonResult struct {
	Scenario      ComparisonScenario
	Result        model.OptimizeResult
	SheetsUsed    int
	TotalCuts     int
	WastePercent  float64
	UnplacedCount int
}

// CompareScenarios runs optimization for each scenario and returns the results
// sorted by scenario order. This enables side-by-side comparison of different
// optimization parameters (e.g., different algorithms, kerf widths, etc.).
func CompareScenarios(scenarios []ComparisonScenario, parts []model.Part, stocks []model.StockSheet) []ComparisonResult {
	results := make([]ComparisonResult, 0, len(scenarios))

	for _, scenario := range scenarios {
		opt := New(scenario.Settings)
		result := opt.Optimize(parts, stocks)

		totalCuts := 0
		for _, sheet := range result.Sheets {
			totalCuts += len(sheet.Placements)
		}

		wastePercent := 100.0 - result.TotalEfficiency()

		results = append(results, ComparisonResult{
			Scenario:      scenario,
			Result:        result,
			SheetsUsed:    len(result.Sheets),
			TotalCuts:     totalCuts,
			WastePercent:  wastePercent,
			UnplacedCount: len(result.UnplacedParts),
		})
	}

	return results
}

// BuildDefaultScenarios generates a set of comparison scenarios based on
// the current settings, varying key parameters to show what-if alternatives.
func BuildDefaultScenarios(baseSettings model.CutSettings) []ComparisonScenario {
	scenarios := []ComparisonScenario{
		{
			Name:     "Current Settings",
			Settings: baseSettings,
		},
	}

	// Scenario: Try the other algorithm
	altAlgo := baseSettings
	if baseSettings.Algorithm == model.AlgorithmGuillotine {
		altAlgo.Algorithm = model.AlgorithmGenetic
		scenarios = append(scenarios, ComparisonScenario{
			Name:     "Genetic Algorithm",
			Settings: altAlgo,
		})
	} else {
		altAlgo.Algorithm = model.AlgorithmGuillotine
		scenarios = append(scenarios, ComparisonScenario{
			Name:     "Guillotine Algorithm",
			Settings: altAlgo,
		})
	}

	// Scenario: Tighter kerf (simulate thinner blade)
	if baseSettings.KerfWidth > 1.0 {
		tightKerf := baseSettings
		tightKerf.KerfWidth = baseSettings.KerfWidth * 0.5
		scenarios = append(scenarios, ComparisonScenario{
			Name:     fmt.Sprintf("Kerf %.1fmm (half)", tightKerf.KerfWidth),
			Settings: tightKerf,
		})
	}

	// Scenario: No edge trim
	if baseSettings.EdgeTrim > 0 {
		noTrim := baseSettings
		noTrim.EdgeTrim = 0
		scenarios = append(scenarios, ComparisonScenario{
			Name:     "No Edge Trim",
			Settings: noTrim,
		})
	}

	return scenarios
}
