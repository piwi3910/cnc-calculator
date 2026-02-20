package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/piwi3910/SlabCut/internal/model"
)

// BackupData is the top-level structure for import/export of all application data.
type BackupData struct {
	Version   string          `json:"version"`
	CreatedAt string          `json:"created_at"`
	Config    model.AppConfig `json:"config"`
}

// ExportAllData exports all application data (config and future inventory data)
// to a single JSON file at the specified path.
func ExportAllData(exportPath string, config model.AppConfig) error {
	backup := BackupData{
		Version:   "1.0.0",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Config:    config,
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup data: %w", err)
	}

	dir := filepath.Dir(exportPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}
	return nil
}

// ImportAllData reads a backup JSON file and returns the contained data.
// The caller is responsible for applying the imported config.
func ImportAllData(importPath string) (BackupData, error) {
	data, err := os.ReadFile(importPath)
	if err != nil {
		return BackupData{}, fmt.Errorf("failed to read backup file: %w", err)
	}
	var backup BackupData
	if err := json.Unmarshal(data, &backup); err != nil {
		return BackupData{}, fmt.Errorf("failed to parse backup file: %w", err)
	}
	if backup.Version == "" {
		return BackupData{}, fmt.Errorf("invalid backup file: missing version field")
	}
	// Ensure RecentProjects is never nil
	if backup.Config.RecentProjects == nil {
		backup.Config.RecentProjects = []string{}
	}
	return backup, nil
}
