package model

import (
	"time"

	"github.com/google/uuid"
)

// ProjectTemplate represents a reusable project configuration that captures
// parts, stock sheets, and settings but not optimization results.
type ProjectTemplate struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CreatedAt   string       `json:"created_at"`
	UpdatedAt   string       `json:"updated_at"`
	Parts       []Part       `json:"parts"`
	Stocks      []StockSheet `json:"stocks"`
	Settings    CutSettings  `json:"settings"`
}

// NewProjectTemplate creates a new template from the given project data.
// It copies parts, stocks, and settings but intentionally excludes results.
func NewProjectTemplate(name, description string, parts []Part, stocks []StockSheet, settings CutSettings) ProjectTemplate {
	now := time.Now().UTC().Format(time.RFC3339)
	return ProjectTemplate{
		ID:          uuid.New().String()[:8],
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Parts:       copyParts(parts),
		Stocks:      copyStocks(stocks),
		Settings:    settings,
	}
}

// ToProject creates a new Project from this template.
// Parts and stocks get fresh IDs so they are independent of the template.
func (t ProjectTemplate) ToProject(projectName string) Project {
	parts := make([]Part, len(t.Parts))
	for i, p := range t.Parts {
		parts[i] = NewPart(p.Label, p.Width, p.Height, p.Quantity)
		parts[i].Grain = p.Grain
		parts[i].Outline = p.Outline
	}

	stocks := make([]StockSheet, len(t.Stocks))
	for i, s := range t.Stocks {
		stocks[i] = NewStockSheet(s.Label, s.Width, s.Height, s.Quantity)
		stocks[i].Tabs = s.Tabs
	}

	return Project{
		Name:     projectName,
		Parts:    parts,
		Stocks:   stocks,
		Settings: t.Settings,
	}
}

// TemplateStore holds a collection of project templates.
type TemplateStore struct {
	Templates []ProjectTemplate `json:"templates"`
}

// NewTemplateStore creates an empty template store.
func NewTemplateStore() TemplateStore {
	return TemplateStore{
		Templates: []ProjectTemplate{},
	}
}

// Add adds a template to the store.
func (ts *TemplateStore) Add(t ProjectTemplate) {
	ts.Templates = append(ts.Templates, t)
}

// Remove removes a template by ID. Returns true if found and removed.
func (ts *TemplateStore) Remove(id string) bool {
	for i, t := range ts.Templates {
		if t.ID == id {
			ts.Templates = append(ts.Templates[:i], ts.Templates[i+1:]...)
			return true
		}
	}
	return false
}

// FindByID returns a pointer to the template with the given ID, or nil.
func (ts *TemplateStore) FindByID(id string) *ProjectTemplate {
	for i := range ts.Templates {
		if ts.Templates[i].ID == id {
			return &ts.Templates[i]
		}
	}
	return nil
}

// Names returns a list of template names for UI dropdowns.
func (ts *TemplateStore) Names() []string {
	names := make([]string, len(ts.Templates))
	for i, t := range ts.Templates {
		names[i] = t.Name
	}
	return names
}

// FindByName returns a pointer to the first template with the given name, or nil.
func (ts *TemplateStore) FindByName(name string) *ProjectTemplate {
	for i := range ts.Templates {
		if ts.Templates[i].Name == name {
			return &ts.Templates[i]
		}
	}
	return nil
}

// copyParts creates a deep copy of a parts slice.
func copyParts(parts []Part) []Part {
	if parts == nil {
		return []Part{}
	}
	cp := make([]Part, len(parts))
	copy(cp, parts)
	return cp
}

// copyStocks creates a deep copy of a stocks slice.
func copyStocks(stocks []StockSheet) []StockSheet {
	if stocks == nil {
		return []StockSheet{}
	}
	cp := make([]StockSheet, len(stocks))
	copy(cp, stocks)
	return cp
}
