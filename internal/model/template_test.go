package model

import (
	"testing"
)

func TestNewProjectTemplate(t *testing.T) {
	parts := []Part{
		NewPart("Side", 600, 400, 2),
		NewPart("Top", 500, 300, 1),
	}
	stocks := []StockSheet{
		NewStockSheet("Plywood", 2440, 1220, 1),
	}
	settings := DefaultSettings()

	tmpl := NewProjectTemplate("Cabinet", "Standard cabinet template", parts, stocks, settings)

	if tmpl.Name != "Cabinet" {
		t.Errorf("expected name 'Cabinet', got %q", tmpl.Name)
	}
	if tmpl.Description != "Standard cabinet template" {
		t.Errorf("expected description 'Standard cabinet template', got %q", tmpl.Description)
	}
	if tmpl.ID == "" {
		t.Error("expected non-empty ID")
	}
	if tmpl.CreatedAt == "" {
		t.Error("expected non-empty CreatedAt")
	}
	if len(tmpl.Parts) != 2 {
		t.Errorf("expected 2 parts, got %d", len(tmpl.Parts))
	}
	if len(tmpl.Stocks) != 1 {
		t.Errorf("expected 1 stock, got %d", len(tmpl.Stocks))
	}
}

func TestProjectTemplate_ToProject(t *testing.T) {
	parts := []Part{
		NewPart("Side", 600, 400, 2),
	}
	stocks := []StockSheet{
		NewStockSheet("Plywood", 2440, 1220, 1),
	}
	settings := DefaultSettings()
	settings.KerfWidth = 5.0

	tmpl := NewProjectTemplate("Test", "desc", parts, stocks, settings)
	proj := tmpl.ToProject("My Project")

	if proj.Name != "My Project" {
		t.Errorf("expected project name 'My Project', got %q", proj.Name)
	}
	if len(proj.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(proj.Parts))
	}
	if proj.Parts[0].Label != "Side" {
		t.Errorf("expected part label 'Side', got %q", proj.Parts[0].Label)
	}
	// Parts should have fresh IDs
	if proj.Parts[0].ID == tmpl.Parts[0].ID {
		t.Error("project parts should have fresh IDs, not template IDs")
	}
	if proj.Settings.KerfWidth != 5.0 {
		t.Errorf("expected kerf width 5.0, got %.1f", proj.Settings.KerfWidth)
	}
	if proj.Result != nil {
		t.Error("project from template should have no result")
	}
}

func TestTemplateStore_AddRemoveFind(t *testing.T) {
	store := NewTemplateStore()

	tmpl1 := NewProjectTemplate("T1", "", nil, nil, DefaultSettings())
	tmpl2 := NewProjectTemplate("T2", "", nil, nil, DefaultSettings())

	store.Add(tmpl1)
	store.Add(tmpl2)

	if len(store.Templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(store.Templates))
	}

	// FindByID
	found := store.FindByID(tmpl1.ID)
	if found == nil {
		t.Fatal("FindByID returned nil for existing template")
	}
	if found.Name != "T1" {
		t.Errorf("expected 'T1', got %q", found.Name)
	}

	// FindByName
	found = store.FindByName("T2")
	if found == nil {
		t.Fatal("FindByName returned nil for existing template")
	}

	// Names
	names := store.Names()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}

	// Remove
	ok := store.Remove(tmpl1.ID)
	if !ok {
		t.Error("Remove should return true for existing template")
	}
	if len(store.Templates) != 1 {
		t.Errorf("expected 1 template after remove, got %d", len(store.Templates))
	}

	// Remove non-existent
	ok = store.Remove("nonexistent")
	if ok {
		t.Error("Remove should return false for non-existent ID")
	}
}

func TestTemplateStore_Empty(t *testing.T) {
	store := NewTemplateStore()

	if len(store.Templates) != 0 {
		t.Errorf("new store should be empty, got %d templates", len(store.Templates))
	}
	if store.FindByID("x") != nil {
		t.Error("FindByID should return nil in empty store")
	}
	if store.FindByName("x") != nil {
		t.Error("FindByName should return nil in empty store")
	}
	if len(store.Names()) != 0 {
		t.Error("Names should return empty slice for empty store")
	}
}

func TestNewProjectTemplate_NilSlices(t *testing.T) {
	tmpl := NewProjectTemplate("Empty", "", nil, nil, DefaultSettings())

	if tmpl.Parts == nil {
		t.Error("Parts should not be nil (should be empty slice)")
	}
	if tmpl.Stocks == nil {
		t.Error("Stocks should not be nil (should be empty slice)")
	}
}
