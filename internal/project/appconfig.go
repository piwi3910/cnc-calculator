package project

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/piwi3910/SlabCut/internal/model"
)

// DefaultConfigDir returns the default directory for application configuration.
// On all platforms this is ~/.slabcut/
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".slabcut")
}

// DefaultConfigPath returns the default path for the application config file.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.json")
}

// SaveAppConfig persists an AppConfig to the given path as JSON.
// It creates any missing parent directories automatically.
func SaveAppConfig(path string, config model.AppConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadAppConfig reads an AppConfig from the given path.
// If the file does not exist, it returns DefaultAppConfig with no error.
func LoadAppConfig(path string) (model.AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.DefaultAppConfig(), nil
		}
		return model.AppConfig{}, err
	}
	var config model.AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return model.AppConfig{}, err
	}
	// Ensure RecentProjects is never nil
	if config.RecentProjects == nil {
		config.RecentProjects = []string{}
	}
	return config, nil
}
