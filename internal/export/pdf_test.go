package export

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

// buildTestResult creates a realistic optimization result for testing.
func buildTestResult() model.OptimizeResult {
	return model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{
					ID:       "s1",
					Label:    "Plywood 2440x1220",
					Width:    2440,
					Height:   1220,
					Quantity: 1,
					Tabs:     model.StockTabConfig{Enabled: false},
				},
				Placements: []model.Placement{
					{
						Part: model.Part{ID: "p1", Label: "Side Panel", Width: 600, Height: 400, Quantity: 2},
						X:    10, Y: 10, Rotated: false,
					},
					{
						Part: model.Part{ID: "p2", Label: "Top", Width: 500, Height: 300, Quantity: 1},
						X:    620, Y: 10, Rotated: false,
					},
					{
						Part: model.Part{ID: "p3", Label: "Shelf", Width: 400, Height: 300, Quantity: 1},
						X:    10, Y: 420, Rotated: true,
					},
				},
			},
			{
				Stock: model.StockSheet{
					ID:       "s2",
					Label:    "MDF 1200x600",
					Width:    1200,
					Height:   600,
					Quantity: 1,
					Tabs:     model.StockTabConfig{Enabled: false},
				},
				Placements: []model.Placement{
					{
						Part: model.Part{ID: "p4", Label: "Back Panel", Width: 800, Height: 500, Quantity: 1},
						X:    10, Y: 10, Rotated: false,
					},
				},
			},
		},
		UnplacedParts: nil,
	}
}

func buildTestSettings() model.CutSettings {
	s := model.DefaultSettings()
	s.StockTabs.Enabled = true
	return s
}

func TestExportPDF_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_output.pdf")

	result := buildTestResult()
	settings := buildTestSettings()

	err := ExportPDF(path, result, settings)
	if err != nil {
		t.Fatalf("ExportPDF returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
	// A valid PDF with 3 pages (2 sheets + summary) should be a reasonable size
	if info.Size() < 500 {
		t.Errorf("PDF file seems too small: %d bytes", info.Size())
	}
}

func TestExportPDF_EmptyResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.pdf")

	result := model.OptimizeResult{Sheets: nil}
	settings := model.DefaultSettings()

	err := ExportPDF(path, result, settings)
	if err == nil {
		t.Fatal("expected error for empty result, got nil")
	}
}

func TestExportPDF_WithUnplacedParts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unplaced.pdf")

	result := buildTestResult()
	result.UnplacedParts = []model.Part{
		{ID: "u1", Label: "Too Big", Width: 3000, Height: 2000, Quantity: 1},
		{ID: "u2", Label: "Another", Width: 1500, Height: 1500, Quantity: 2},
	}
	settings := buildTestSettings()

	err := ExportPDF(path, result, settings)
	if err != nil {
		t.Fatalf("ExportPDF returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}

func TestExportPDF_WithStockTabs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tabs.pdf")

	result := buildTestResult()
	// Enable stock-level tab override on the first sheet
	result.Sheets[0].Stock.Tabs = model.StockTabConfig{
		Enabled:       true,
		AdvancedMode:  false,
		TopPadding:    30,
		BottomPadding: 30,
		LeftPadding:   20,
		RightPadding:  20,
	}

	settings := buildTestSettings()

	err := ExportPDF(path, result, settings)
	if err != nil {
		t.Fatalf("ExportPDF returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}

func TestExportPDF_WithAdvancedTabs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "advanced_tabs.pdf")

	result := buildTestResult()
	result.Sheets[0].Stock.Tabs = model.StockTabConfig{
		Enabled:      true,
		AdvancedMode: true,
		CustomZones: []model.TabZone{
			{X: 0, Y: 0, Width: 100, Height: 50},
			{X: 2340, Y: 0, Width: 100, Height: 50},
		},
	}

	settings := model.DefaultSettings()
	settings.StockTabs.Enabled = false

	err := ExportPDF(path, result, settings)
	if err != nil {
		t.Fatalf("ExportPDF returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}

func TestExportPDF_SingleSheet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.pdf")

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{
					ID: "s1", Label: "Board", Width: 1000, Height: 500, Quantity: 1,
					Tabs: model.StockTabConfig{Enabled: false},
				},
				Placements: []model.Placement{
					{
						Part: model.Part{ID: "p1", Label: "A", Width: 200, Height: 200, Quantity: 1},
						X:    0, Y: 0, Rotated: false,
					},
				},
			},
		},
	}
	settings := model.DefaultSettings()
	settings.StockTabs.Enabled = false

	err := ExportPDF(path, result, settings)
	if err != nil {
		t.Fatalf("ExportPDF returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}

func TestExportPDF_ManyParts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "many_parts.pdf")

	// Generate more parts than colors to test color cycling
	placements := make([]model.Placement, 20)
	for i := range placements {
		placements[i] = model.Placement{
			Part: model.Part{
				ID:       fmt.Sprintf("p%d", i),
				Label:    fmt.Sprintf("Part %d", i+1),
				Width:    100,
				Height:   80,
				Quantity: 1,
			},
			X:       float64((i % 5) * 110),
			Y:       float64((i / 5) * 90),
			Rotated: i%3 == 0,
		}
	}

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{
					ID: "s1", Label: "Large Board", Width: 600, Height: 400, Quantity: 1,
					Tabs: model.StockTabConfig{Enabled: false},
				},
				Placements: placements,
			},
		},
	}

	settings := model.DefaultSettings()
	settings.StockTabs.Enabled = false

	err := ExportPDF(path, result, settings)
	if err != nil {
		t.Fatalf("ExportPDF returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}

func TestCountParts(t *testing.T) {
	result := buildTestResult()
	got := countParts(result)
	if got != 4 {
		t.Errorf("countParts() = %d, want 4", got)
	}
}

func TestLabelFontSize(t *testing.T) {
	tests := []struct {
		w, h float64
		want float64
	}{
		{50, 50, 8},
		{30, 25, 7},
		{10, 15, 6},
	}
	for _, tt := range tests {
		got := labelFontSize(tt.w, tt.h)
		if got != tt.want {
			t.Errorf("labelFontSize(%v, %v) = %v, want %v", tt.w, tt.h, got, tt.want)
		}
	}
}
