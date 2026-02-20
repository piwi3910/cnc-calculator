package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

func TestDefaultInventoryPath(t *testing.T) {
	path, err := DefaultInventoryPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if filepath.Base(path) != "inventory.json" {
		t.Errorf("expected filename inventory.json, got %s", filepath.Base(path))
	}
	dir := filepath.Base(filepath.Dir(path))
	if dir != ".slabcut" {
		t.Errorf("expected parent dir .slabcut, got %s", dir)
	}
}

func TestSaveAndLoadInventory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_inventory.json")

	inv := model.Inventory{
		Tools: []model.ToolProfile{
			model.NewToolProfile("Test Mill", 6.0, 1500, 500, 18000, 5.0, 18.0, 6.0),
		},
		Stocks: []model.StockPreset{
			model.NewStockPreset("Test Plywood", 2440, 1220, "Plywood"),
		},
	}

	// Save
	if err := SaveInventory(path, inv); err != nil {
		t.Fatalf("SaveInventory failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("inventory file was not created")
	}

	// Load
	loaded, err := LoadInventory(path)
	if err != nil {
		t.Fatalf("LoadInventory failed: %v", err)
	}

	if len(loaded.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(loaded.Tools))
	}
	if loaded.Tools[0].Name != "Test Mill" {
		t.Errorf("expected tool name 'Test Mill', got %q", loaded.Tools[0].Name)
	}
	if loaded.Tools[0].ToolDiameter != 6.0 {
		t.Errorf("expected tool diameter 6.0, got %f", loaded.Tools[0].ToolDiameter)
	}

	if len(loaded.Stocks) != 1 {
		t.Errorf("expected 1 stock, got %d", len(loaded.Stocks))
	}
	if loaded.Stocks[0].Name != "Test Plywood" {
		t.Errorf("expected stock name 'Test Plywood', got %q", loaded.Stocks[0].Name)
	}
	if loaded.Stocks[0].Width != 2440 {
		t.Errorf("expected width 2440, got %f", loaded.Stocks[0].Width)
	}
}

func TestLoadInventoryCreatesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent", "inventory.json")

	inv, err := LoadInventory(path)
	if err != nil {
		t.Fatalf("LoadInventory failed: %v", err)
	}

	// Should have created defaults
	if len(inv.Tools) == 0 {
		t.Error("expected default tools, got none")
	}
	if len(inv.Stocks) == 0 {
		t.Error("expected default stocks, got none")
	}

	// Should have written the file
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected default inventory file to be created")
	}
}

func TestImportInventory(t *testing.T) {
	tmpDir := t.TempDir()

	existing := model.Inventory{
		Tools: []model.ToolProfile{
			{ID: "tool-001", Name: "Existing Mill", ToolDiameter: 6.0},
		},
		Stocks: []model.StockPreset{
			{ID: "stock-001", Name: "Existing Plywood", Width: 2440, Height: 1220, Material: "Plywood"},
		},
	}

	imported := model.Inventory{
		Tools: []model.ToolProfile{
			{ID: "tool-001", Name: "Duplicate Mill", ToolDiameter: 6.0}, // same ID, should be skipped
			{ID: "tool-002", Name: "New Mill", ToolDiameter: 3.0},       // new, should be added
		},
		Stocks: []model.StockPreset{
			{ID: "stock-002", Name: "New MDF", Width: 1220, Height: 610, Material: "MDF"}, // new
		},
	}

	// Write import file
	importPath := filepath.Join(tmpDir, "import.json")
	data, _ := json.MarshalIndent(imported, "", "  ")
	if err := os.WriteFile(importPath, data, 0644); err != nil {
		t.Fatalf("failed to write import file: %v", err)
	}

	merged, err := ImportInventory(importPath, existing)
	if err != nil {
		t.Fatalf("ImportInventory failed: %v", err)
	}

	if len(merged.Tools) != 2 {
		t.Errorf("expected 2 tools after merge, got %d", len(merged.Tools))
	}
	if merged.Tools[0].Name != "Existing Mill" {
		t.Errorf("expected first tool to be 'Existing Mill', got %q", merged.Tools[0].Name)
	}
	if merged.Tools[1].Name != "New Mill" {
		t.Errorf("expected second tool to be 'New Mill', got %q", merged.Tools[1].Name)
	}

	if len(merged.Stocks) != 2 {
		t.Errorf("expected 2 stocks after merge, got %d", len(merged.Stocks))
	}
}

func TestExportInventory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "export.json")

	inv := model.DefaultInventory()
	if err := ExportInventory(path, inv); err != nil {
		t.Fatalf("ExportInventory failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	var loaded model.Inventory
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal exported inventory: %v", err)
	}

	if len(loaded.Tools) != len(inv.Tools) {
		t.Errorf("expected %d tools, got %d", len(inv.Tools), len(loaded.Tools))
	}
	if len(loaded.Stocks) != len(inv.Stocks) {
		t.Errorf("expected %d stocks, got %d", len(inv.Stocks), len(loaded.Stocks))
	}
}

func TestToolProfileApplyToSettings(t *testing.T) {
	tp := model.ToolProfile{
		ToolDiameter: 3.0,
		FeedRate:     1000,
		PlungeRate:   300,
		SpindleSpeed: 20000,
		SafeZ:        10.0,
		CutDepth:     12.0,
		PassDepth:    3.0,
	}

	settings := model.DefaultSettings()
	tp.ApplyToSettings(&settings)

	if settings.ToolDiameter != 3.0 {
		t.Errorf("expected ToolDiameter 3.0, got %f", settings.ToolDiameter)
	}
	if settings.FeedRate != 1000 {
		t.Errorf("expected FeedRate 1000, got %f", settings.FeedRate)
	}
	if settings.SpindleSpeed != 20000 {
		t.Errorf("expected SpindleSpeed 20000, got %d", settings.SpindleSpeed)
	}
	if settings.KerfWidth != 3.0 {
		t.Errorf("expected KerfWidth to be set to ToolDiameter 3.0, got %f", settings.KerfWidth)
	}
}

func TestStockPresetToStockSheet(t *testing.T) {
	sp := model.NewStockPreset("Plywood 2440x1220", 2440, 1220, "Plywood")
	sheet := sp.ToStockSheet(3)

	if sheet.Width != 2440 {
		t.Errorf("expected width 2440, got %f", sheet.Width)
	}
	if sheet.Height != 1220 {
		t.Errorf("expected height 1220, got %f", sheet.Height)
	}
	if sheet.Quantity != 3 {
		t.Errorf("expected quantity 3, got %d", sheet.Quantity)
	}
}

func TestInventoryFindByName(t *testing.T) {
	inv := model.DefaultInventory()

	tool := inv.FindToolByName("6mm End Mill")
	if tool == nil {
		t.Fatal("expected to find '6mm End Mill'")
	}
	if tool.ToolDiameter != 6.0 {
		t.Errorf("expected diameter 6.0, got %f", tool.ToolDiameter)
	}

	missing := inv.FindToolByName("Nonexistent Tool")
	if missing != nil {
		t.Error("expected nil for nonexistent tool")
	}

	stock := inv.FindStockByName("Plywood 2440x1220 (8'x4')")
	if stock == nil {
		t.Fatal("expected to find plywood stock preset")
	}

	missingStock := inv.FindStockByName("Nonexistent Stock")
	if missingStock != nil {
		t.Error("expected nil for nonexistent stock")
	}
}

func TestInventoryToolAndStockNames(t *testing.T) {
	inv := model.DefaultInventory()

	toolNames := inv.ToolNames()
	if len(toolNames) != len(inv.Tools) {
		t.Errorf("expected %d tool names, got %d", len(inv.Tools), len(toolNames))
	}

	stockNames := inv.StockNames()
	if len(stockNames) != len(inv.Stocks) {
		t.Errorf("expected %d stock names, got %d", len(inv.Stocks), len(stockNames))
	}
}
