package project

import (
	"path/filepath"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

func TestSaveAndLoadTemplates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "templates.json")

	store := model.NewTemplateStore()
	parts := []model.Part{model.NewPart("Shelf", 500, 300, 2)}
	stocks := []model.StockSheet{model.NewStockSheet("Board", 2440, 1220, 1)}
	settings := model.DefaultSettings()

	tmpl := model.NewProjectTemplate("Cabinet", "Standard cabinet", parts, stocks, settings)
	store.Add(tmpl)

	if err := SaveTemplates(path, store); err != nil {
		t.Fatalf("SaveTemplates error: %v", err)
	}

	loaded, err := LoadTemplates(path)
	if err != nil {
		t.Fatalf("LoadTemplates error: %v", err)
	}

	if len(loaded.Templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded.Templates))
	}
	if loaded.Templates[0].Name != "Cabinet" {
		t.Errorf("expected 'Cabinet', got %q", loaded.Templates[0].Name)
	}
	if len(loaded.Templates[0].Parts) != 1 {
		t.Errorf("expected 1 part, got %d", len(loaded.Templates[0].Parts))
	}
}

func TestLoadTemplates_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	store, err := LoadTemplates(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(store.Templates) != 0 {
		t.Errorf("expected empty store, got %d templates", len(store.Templates))
	}
}

func TestSaveAndLoadTemplates_Multiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "templates.json")

	store := model.NewTemplateStore()
	store.Add(model.NewProjectTemplate("T1", "First", nil, nil, model.DefaultSettings()))
	store.Add(model.NewProjectTemplate("T2", "Second", nil, nil, model.DefaultSettings()))
	store.Add(model.NewProjectTemplate("T3", "Third", nil, nil, model.DefaultSettings()))

	if err := SaveTemplates(path, store); err != nil {
		t.Fatalf("SaveTemplates error: %v", err)
	}

	loaded, err := LoadTemplates(path)
	if err != nil {
		t.Fatalf("LoadTemplates error: %v", err)
	}
	if len(loaded.Templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(loaded.Templates))
	}
}
