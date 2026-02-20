package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/piwi3910/SlabCut/internal/model"
)

// DefaultProfilesDir returns the default directory for storing custom profiles.
func DefaultProfilesDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "slabcut")
	return dir, nil
}

// DefaultProfilesPath returns the default file path for custom profiles.
func DefaultProfilesPath() (string, error) {
	dir, err := DefaultProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

// SaveCustomProfiles saves custom profiles to a JSON file.
func SaveCustomProfiles(path string, profiles []model.GCodeProfile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadCustomProfiles loads custom profiles from a JSON file.
// Returns an empty slice if the file does not exist.
func LoadCustomProfiles(path string) ([]model.GCodeProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []model.GCodeProfile{}, nil
		}
		return nil, err
	}

	var profiles []model.GCodeProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, err
	}

	// Ensure loaded profiles are not marked as built-in
	for i := range profiles {
		profiles[i].IsBuiltIn = false
	}
	return profiles, nil
}

// SaveCustomProfilesToDefault saves custom profiles to the default path.
func SaveCustomProfilesToDefault(profiles []model.GCodeProfile) error {
	path, err := DefaultProfilesPath()
	if err != nil {
		return err
	}
	return SaveCustomProfiles(path, profiles)
}

// LoadCustomProfilesFromDefault loads custom profiles from the default path.
func LoadCustomProfilesFromDefault() ([]model.GCodeProfile, error) {
	path, err := DefaultProfilesPath()
	if err != nil {
		return nil, err
	}
	return LoadCustomProfiles(path)
}

// ExportProfile exports a single profile to a JSON file (for sharing).
func ExportProfile(path string, profile model.GCodeProfile) error {
	profile.IsBuiltIn = false
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ImportProfile imports a single profile from a JSON file.
func ImportProfile(path string) (model.GCodeProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.GCodeProfile{}, err
	}

	var profile model.GCodeProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return model.GCodeProfile{}, err
	}

	profile.IsBuiltIn = false
	if profile.Name == "" {
		return model.GCodeProfile{}, errors.New("imported profile has no name")
	}
	return profile, nil
}
