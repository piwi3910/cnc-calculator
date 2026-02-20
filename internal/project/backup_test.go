package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

func TestExportAndImportAllData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.json")

	cfg := model.DefaultAppConfig()
	cfg.DefaultFeedRate = 2000.0
	cfg.Theme = "dark"

	if err := ExportAllData(path, cfg); err != nil {
		t.Fatalf("ExportAllData failed: %v", err)
	}

	backup, err := ImportAllData(path)
	if err != nil {
		t.Fatalf("ImportAllData failed: %v", err)
	}

	if backup.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", backup.Version)
	}
	if backup.CreatedAt == "" {
		t.Error("expected non-empty CreatedAt")
	}
	if backup.Config.DefaultFeedRate != 2000.0 {
		t.Errorf("expected DefaultFeedRate=2000.0, got %f", backup.Config.DefaultFeedRate)
	}
	if backup.Config.Theme != "dark" {
		t.Errorf("expected Theme=dark, got %s", backup.Config.Theme)
	}
}

func TestImportAllDataMissingFile(t *testing.T) {
	_, err := ImportAllData(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestImportAllDataInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not json}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ImportAllData(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestImportAllDataMissingVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noversion.json")
	data := []byte(`{"config":{"theme":"dark"}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ImportAllData(path)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestExportAllDataCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "backup.json")

	cfg := model.DefaultAppConfig()
	if err := ExportAllData(path, cfg); err != nil {
		t.Fatalf("ExportAllData should create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("backup file was not created")
	}
}

func TestImportAllDataNilRecentProjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.json")
	data := []byte(`{"version":"1.0.0","created_at":"2025-01-01T00:00:00Z","config":{"recent_projects":null}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	backup, err := ImportAllData(path)
	if err != nil {
		t.Fatalf("ImportAllData failed: %v", err)
	}
	if backup.Config.RecentProjects == nil {
		t.Error("RecentProjects should not be nil after import")
	}
}
