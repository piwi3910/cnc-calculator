package model

import (
	"testing"
)

func TestAllProfilesIncludesBuiltInAndCustom(t *testing.T) {
	// Reset custom profiles
	CustomProfiles = nil

	builtInCount := len(GCodeProfiles)
	all := AllProfiles()
	if len(all) != builtInCount {
		t.Errorf("expected %d profiles with no custom, got %d", builtInCount, len(all))
	}

	// Add a custom profile
	CustomProfiles = []GCodeProfile{
		{Name: "Custom1", Description: "Test custom"},
	}
	defer func() { CustomProfiles = nil }()

	all = AllProfiles()
	if len(all) != builtInCount+1 {
		t.Errorf("expected %d profiles with 1 custom, got %d", builtInCount+1, len(all))
	}
}

func TestGetProfileFindsCustom(t *testing.T) {
	CustomProfiles = []GCodeProfile{
		{Name: "MyCustom", Description: "Custom profile", RapidMove: "G0", FeedMove: "G1"},
	}
	defer func() { CustomProfiles = nil }()

	p := GetProfile("MyCustom")
	if p.Name != "MyCustom" {
		t.Errorf("expected MyCustom, got %s", p.Name)
	}
}

func TestGetProfileFallsBackToGeneric(t *testing.T) {
	p := GetProfile("NonExistent")
	if p.Name != "Generic" {
		t.Errorf("expected Generic fallback, got %s", p.Name)
	}
}

func TestGetProfileNamesIncludesCustom(t *testing.T) {
	CustomProfiles = []GCodeProfile{
		{Name: "CustomA"},
		{Name: "CustomB"},
	}
	defer func() { CustomProfiles = nil }()

	names := GetProfileNames()
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}

	if !found["Grbl"] {
		t.Error("missing built-in profile Grbl")
	}
	if !found["CustomA"] {
		t.Error("missing custom profile CustomA")
	}
	if !found["CustomB"] {
		t.Error("missing custom profile CustomB")
	}
}

func TestAddCustomProfile(t *testing.T) {
	CustomProfiles = nil
	defer func() { CustomProfiles = nil }()

	p := GCodeProfile{Name: "NewProfile", Description: "New"}
	if err := AddCustomProfile(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(CustomProfiles) != 1 {
		t.Fatalf("expected 1 custom profile, got %d", len(CustomProfiles))
	}
	if CustomProfiles[0].Name != "NewProfile" {
		t.Errorf("expected NewProfile, got %s", CustomProfiles[0].Name)
	}
}

func TestAddCustomProfileRejectsBuiltInName(t *testing.T) {
	CustomProfiles = nil
	defer func() { CustomProfiles = nil }()

	p := GCodeProfile{Name: "Grbl", Description: "Conflict"}
	if err := AddCustomProfile(p); err == nil {
		t.Fatal("expected error when adding profile with built-in name")
	}
}

func TestAddCustomProfileUpdatesExisting(t *testing.T) {
	CustomProfiles = nil
	defer func() { CustomProfiles = nil }()

	p1 := GCodeProfile{Name: "MyProfile", Description: "Version 1"}
	_ = AddCustomProfile(p1)

	p2 := GCodeProfile{Name: "MyProfile", Description: "Version 2"}
	_ = AddCustomProfile(p2)

	if len(CustomProfiles) != 1 {
		t.Fatalf("expected 1 custom profile after update, got %d", len(CustomProfiles))
	}
	if CustomProfiles[0].Description != "Version 2" {
		t.Errorf("expected updated description, got %s", CustomProfiles[0].Description)
	}
}

func TestRemoveCustomProfile(t *testing.T) {
	CustomProfiles = []GCodeProfile{
		{Name: "ToRemove", Description: "Remove me"},
	}
	defer func() { CustomProfiles = nil }()

	if err := RemoveCustomProfile("ToRemove"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(CustomProfiles) != 0 {
		t.Error("profile was not removed")
	}
}

func TestRemoveCustomProfileRejectsBuiltIn(t *testing.T) {
	if err := RemoveCustomProfile("Grbl"); err == nil {
		t.Fatal("expected error when removing built-in profile")
	}
}

func TestRemoveCustomProfileNotFound(t *testing.T) {
	CustomProfiles = nil
	if err := RemoveCustomProfile("NonExistent"); err == nil {
		t.Fatal("expected error when removing non-existent profile")
	}
}

func TestNewCustomProfile(t *testing.T) {
	p := NewCustomProfile("Test Custom")
	if p.Name != "Test Custom" {
		t.Errorf("expected name 'Test Custom', got %s", p.Name)
	}
	if p.IsBuiltIn {
		t.Error("custom profile should not be built-in")
	}
	// Should inherit Generic defaults
	if p.RapidMove != "G0" {
		t.Errorf("expected G0 rapid move from Generic, got %s", p.RapidMove)
	}
}

func TestBuiltInProfilesMarkedCorrectly(t *testing.T) {
	for _, p := range GCodeProfiles {
		if !p.IsBuiltIn {
			t.Errorf("built-in profile %s should have IsBuiltIn=true", p.Name)
		}
	}
}
