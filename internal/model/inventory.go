package model

import "github.com/google/uuid"

// ToolProfile represents a reusable cutting tool configuration.
type ToolProfile struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ToolDiameter float64 `json:"tool_diameter"`
	FeedRate     float64 `json:"feed_rate"`
	PlungeRate   float64 `json:"plunge_rate"`
	SpindleSpeed int     `json:"spindle_speed"`
	SafeZ        float64 `json:"safe_z"`
	CutDepth     float64 `json:"cut_depth"`
	PassDepth    float64 `json:"pass_depth"`
}

// NewToolProfile creates a new ToolProfile with a generated ID.
func NewToolProfile(name string, diameter, feedRate, plungeRate float64, spindleSpeed int, safeZ, cutDepth, passDepth float64) ToolProfile {
	return ToolProfile{
		ID:           uuid.New().String()[:8],
		Name:         name,
		ToolDiameter: diameter,
		FeedRate:     feedRate,
		PlungeRate:   plungeRate,
		SpindleSpeed: spindleSpeed,
		SafeZ:        safeZ,
		CutDepth:     cutDepth,
		PassDepth:    passDepth,
	}
}

// ApplyToSettings copies this tool profile's parameters into the given CutSettings.
func (tp ToolProfile) ApplyToSettings(s *CutSettings) {
	s.ToolDiameter = tp.ToolDiameter
	s.FeedRate = tp.FeedRate
	s.PlungeRate = tp.PlungeRate
	s.SpindleSpeed = tp.SpindleSpeed
	s.SafeZ = tp.SafeZ
	s.CutDepth = tp.CutDepth
	s.PassDepth = tp.PassDepth
	// Also update kerf width to match tool diameter
	s.KerfWidth = tp.ToolDiameter
}

// StockPreset represents a reusable stock sheet definition.
type StockPreset struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Material string  `json:"material"`
}

// NewStockPreset creates a new StockPreset with a generated ID.
func NewStockPreset(name string, width, height float64, material string) StockPreset {
	return StockPreset{
		ID:       uuid.New().String()[:8],
		Name:     name,
		Width:    width,
		Height:   height,
		Material: material,
	}
}

// ToStockSheet converts a StockPreset into a StockSheet with the given quantity.
func (sp StockPreset) ToStockSheet(qty int) StockSheet {
	return NewStockSheet(sp.Name, sp.Width, sp.Height, qty)
}

// Inventory holds the user's saved tool profiles and stock presets.
type Inventory struct {
	Tools  []ToolProfile `json:"tools"`
	Stocks []StockPreset `json:"stocks"`
}

// DefaultInventory returns an inventory populated with common defaults.
func DefaultInventory() Inventory {
	return Inventory{
		Tools: []ToolProfile{
			NewToolProfile("6mm End Mill", 6.0, 1500, 500, 18000, 5.0, 18.0, 6.0),
			NewToolProfile("3mm End Mill", 3.0, 1000, 300, 20000, 5.0, 12.0, 3.0),
			NewToolProfile("1/4\" End Mill (6.35mm)", 6.35, 1500, 500, 18000, 5.0, 18.0, 6.0),
			NewToolProfile("1/8\" End Mill (3.175mm)", 3.175, 800, 250, 22000, 5.0, 12.0, 3.0),
			NewToolProfile("V-Bit 60deg 6mm", 6.0, 800, 300, 18000, 5.0, 3.0, 1.0),
		},
		Stocks: []StockPreset{
			NewStockPreset("Plywood 2440x1220 (8'x4')", 2440, 1220, "Plywood"),
			NewStockPreset("MDF 2440x1220 (8'x4')", 2440, 1220, "MDF"),
			NewStockPreset("MDF 1220x610 (4'x2')", 1220, 610, "MDF"),
			NewStockPreset("Plywood 1220x610 (4'x2')", 1220, 610, "Plywood"),
			NewStockPreset("Acrylic 600x400", 600, 400, "Acrylic"),
			NewStockPreset("Aluminium 600x300", 600, 300, "Aluminium"),
		},
	}
}

// FindToolByID returns a pointer to the tool with the given ID, or nil.
func (inv *Inventory) FindToolByID(id string) *ToolProfile {
	for i := range inv.Tools {
		if inv.Tools[i].ID == id {
			return &inv.Tools[i]
		}
	}
	return nil
}

// FindStockByID returns a pointer to the stock preset with the given ID, or nil.
func (inv *Inventory) FindStockByID(id string) *StockPreset {
	for i := range inv.Stocks {
		if inv.Stocks[i].ID == id {
			return &inv.Stocks[i]
		}
	}
	return nil
}

// ToolNames returns a list of tool profile names for UI dropdowns.
func (inv *Inventory) ToolNames() []string {
	names := make([]string, len(inv.Tools))
	for i, t := range inv.Tools {
		names[i] = t.Name
	}
	return names
}

// StockNames returns a list of stock preset names for UI dropdowns.
func (inv *Inventory) StockNames() []string {
	names := make([]string, len(inv.Stocks))
	for i, s := range inv.Stocks {
		names[i] = s.Name
	}
	return names
}

// FindToolByName returns a pointer to the first tool with the given name, or nil.
func (inv *Inventory) FindToolByName(name string) *ToolProfile {
	for i := range inv.Tools {
		if inv.Tools[i].Name == name {
			return &inv.Tools[i]
		}
	}
	return nil
}

// FindStockByName returns a pointer to the first stock preset with the given name, or nil.
func (inv *Inventory) FindStockByName(name string) *StockPreset {
	for i := range inv.Stocks {
		if inv.Stocks[i].Name == name {
			return &inv.Stocks[i]
		}
	}
	return nil
}
