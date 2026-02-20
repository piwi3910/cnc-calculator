package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

func TestSaveAndLoadAppConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := model.DefaultAppConfig()
	cfg.DefaultKerfWidth = 4.0
	cfg.Theme = "dark"
	cfg.AutoSaveInterval = 5
	cfg.RecentProjects = []string{"/tmp/proj1.cnccalc", "/tmp/proj2.cnccalc"}

	if err := SaveAppConfig(path, cfg); err != nil {
		t.Fatalf("SaveAppConfig failed: %v", err)
	}

	loaded, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig failed: %v", err)
	}

	if loaded.DefaultKerfWidth != 4.0 {
		t.Errorf("expected DefaultKerfWidth=4.0, got %f", loaded.DefaultKerfWidth)
	}
	if loaded.Theme != "dark" {
		t.Errorf("expected Theme=dark, got %s", loaded.Theme)
	}
	if loaded.AutoSaveInterval != 5 {
		t.Errorf("expected AutoSaveInterval=5, got %d", loaded.AutoSaveInterval)
	}
	if len(loaded.RecentProjects) != 2 {
		t.Errorf("expected 2 recent projects, got %d", len(loaded.RecentProjects))
	}
}

func TestLoadAppConfigMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "config.json")

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}

	defaults := model.DefaultAppConfig()
	if cfg.DefaultKerfWidth != defaults.DefaultKerfWidth {
		t.Errorf("expected default kerf width %f, got %f", defaults.DefaultKerfWidth, cfg.DefaultKerfWidth)
	}
	if cfg.Theme != "system" {
		t.Errorf("expected theme=system, got %s", cfg.Theme)
	}
}

func TestLoadAppConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAppConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSaveAppConfigCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "config.json")

	cfg := model.DefaultAppConfig()
	if err := SaveAppConfig(path, cfg); err != nil {
		t.Fatalf("SaveAppConfig should create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}
}

func TestLoadAppConfigNilRecentProjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write config with null recent_projects
	data := []byte(`{"default_kerf_width":3.2,"theme":"light","recent_projects":null}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig failed: %v", err)
	}
	if cfg.RecentProjects == nil {
		t.Error("RecentProjects should not be nil after loading")
	}
}
