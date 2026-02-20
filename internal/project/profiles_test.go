package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

func TestSaveAndLoadCustomProfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")

	profiles := []model.GCodeProfile{
		{
			Name:          "TestProfile1",
			Description:   "Test profile one",
			IsBuiltIn:     false,
			Units:         "mm",
			StartCode:     []string{"G90", "G21"},
			SpindleStart:  "M3 S%d",
			SpindleStop:   "M5",
			HomeAll:       "$H",
			HomeXY:        "$H",
			AbsoluteMode:  "G90",
			FeedMode:      "G94",
			RapidMove:     "G0",
			FeedMove:      "G1",
			EndCode:       []string{"M5", "M2"},
			CommentPrefix: ";",
			CommentSuffix: "",
			DecimalPlaces: 3,
			LeadingZeros:  false,
		},
		{
			Name:          "TestProfile2",
			Description:   "Test profile two",
			IsBuiltIn:     false,
			Units:         "inches",
			StartCode:     []string{"G90", "G20"},
			SpindleStart:  "M3 S%d",
			SpindleStop:   "M5",
			HomeAll:       "G28",
			HomeXY:        "G28 X0 Y0",
			AbsoluteMode:  "G90",
			FeedMode:      "G94",
			RapidMove:     "G0",
			FeedMove:      "G1",
			EndCode:       []string{"M5", "M30"},
			CommentPrefix: "(",
			CommentSuffix: ")",
			DecimalPlaces: 4,
			LeadingZeros:  true,
		},
	}

	// Save
	err := SaveCustomProfiles(path, profiles)
	if err != nil {
		t.Fatalf("SaveCustomProfiles: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("profiles file was not created")
	}

	// Load
	loaded, err := LoadCustomProfiles(path)
	if err != nil {
		t.Fatalf("LoadCustomProfiles: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(loaded))
	}

	if loaded[0].Name != "TestProfile1" {
		t.Errorf("expected name TestProfile1, got %s", loaded[0].Name)
	}
	if loaded[1].Name != "TestProfile2" {
		t.Errorf("expected name TestProfile2, got %s", loaded[1].Name)
	}

	// Ensure IsBuiltIn is forced to false on load
	if loaded[0].IsBuiltIn {
		t.Error("loaded profile should not be marked as built-in")
	}
}

func TestLoadCustomProfilesNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	profiles, err := LoadCustomProfiles(path)
	if err != nil {
		t.Fatalf("expected no error for nonexistent file, got: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles for nonexistent file, got %d", len(profiles))
	}
}

func TestLoadCustomProfilesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	err := os.WriteFile(path, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadCustomProfiles(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExportAndImportProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exported.json")

	original := model.GCodeProfile{
		Name:          "ExportedProfile",
		Description:   "A profile for export testing",
		IsBuiltIn:     true, // Should be stripped on export
		Units:         "mm",
		StartCode:     []string{"G90", "G21"},
		SpindleStart:  "M3 S%d",
		SpindleStop:   "M5",
		HomeAll:       "$H",
		HomeXY:        "$H",
		AbsoluteMode:  "G90",
		FeedMode:      "G94",
		RapidMove:     "G0",
		FeedMove:      "G1",
		EndCode:       []string{"M5", "M2"},
		CommentPrefix: ";",
		CommentSuffix: "",
		DecimalPlaces: 3,
		LeadingZeros:  false,
	}

	// Export
	err := ExportProfile(path, original)
	if err != nil {
		t.Fatalf("ExportProfile: %v", err)
	}

	// Import
	imported, err := ImportProfile(path)
	if err != nil {
		t.Fatalf("ImportProfile: %v", err)
	}

	if imported.Name != "ExportedProfile" {
		t.Errorf("expected name ExportedProfile, got %s", imported.Name)
	}

	// IsBuiltIn should be false after import
	if imported.IsBuiltIn {
		t.Error("imported profile should not be marked as built-in")
	}

	if len(imported.StartCode) != 2 {
		t.Errorf("expected 2 start codes, got %d", len(imported.StartCode))
	}
}

func TestImportProfileNoName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noname.json")

	err := os.WriteFile(path, []byte(`{"description": "no name"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ImportProfile(path)
	if err == nil {
		t.Fatal("expected error for profile without name")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "profiles.json")

	err := SaveCustomProfiles(path, []model.GCodeProfile{})
	if err != nil {
		t.Fatalf("SaveCustomProfiles should create directories: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created in nested directory")
	}
}
