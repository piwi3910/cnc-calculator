package project

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/piwi3910/SlabCut/internal/model"
)

// DefaultInventoryPath returns the default file path for the inventory file.
// This is located at ~/.slabcut/inventory.json.
func DefaultInventoryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".slabcut", "inventory.json"), nil
}

// SaveInventory writes the inventory to the specified JSON file.
// It creates parent directories if they do not exist.
func SaveInventory(path string, inv model.Inventory) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadInventory reads the inventory from the specified JSON file.
// If the file does not exist, it returns the default inventory and saves it.
func LoadInventory(path string) (model.Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			inv := model.DefaultInventory()
			if saveErr := SaveInventory(path, inv); saveErr != nil {
				return inv, saveErr
			}
			return inv, nil
		}
		return model.Inventory{}, err
	}
	var inv model.Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return model.Inventory{}, err
	}
	return inv, nil
}

// LoadOrCreateInventory loads the inventory from the default path.
// If the file does not exist, it creates one with default entries.
func LoadOrCreateInventory() (model.Inventory, string, error) {
	path, err := DefaultInventoryPath()
	if err != nil {
		return model.DefaultInventory(), "", err
	}
	inv, err := LoadInventory(path)
	return inv, path, err
}

// ExportInventory exports the inventory to a user-specified JSON file.
func ExportInventory(path string, inv model.Inventory) error {
	return SaveInventory(path, inv)
}

// ImportInventory imports an inventory from a user-specified JSON file,
// merging it with the existing inventory. Duplicate IDs are skipped.
func ImportInventory(path string, existing model.Inventory) (model.Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return existing, err
	}
	var imported model.Inventory
	if err := json.Unmarshal(data, &imported); err != nil {
		return existing, err
	}

	// Build sets of existing IDs for deduplication
	toolIDs := make(map[string]bool, len(existing.Tools))
	for _, t := range existing.Tools {
		toolIDs[t.ID] = true
	}
	stockIDs := make(map[string]bool, len(existing.Stocks))
	for _, s := range existing.Stocks {
		stockIDs[s.ID] = true
	}

	// Merge tools
	for _, t := range imported.Tools {
		if !toolIDs[t.ID] {
			existing.Tools = append(existing.Tools, t)
			toolIDs[t.ID] = true
		}
	}

	// Merge stocks
	for _, s := range imported.Stocks {
		if !stockIDs[s.ID] {
			existing.Stocks = append(existing.Stocks, s)
			stockIDs[s.ID] = true
		}
	}

	return existing, nil
}
