package project

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/piwi3910/SlabCut/internal/model"
)

// DefaultTemplatePath returns the default file path for the templates store.
// This is located at ~/.slabcut/templates.json.
func DefaultTemplatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".slabcut")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "templates.json"), nil
}

// SaveTemplates writes the template store to a JSON file.
func SaveTemplates(path string, store model.TemplateStore) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadTemplates reads a template store from a JSON file.
// If the file does not exist, returns an empty store.
func LoadTemplates(path string) (model.TemplateStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.NewTemplateStore(), nil
		}
		return model.TemplateStore{}, err
	}
	var store model.TemplateStore
	if err := json.Unmarshal(data, &store); err != nil {
		return model.TemplateStore{}, err
	}
	if store.Templates == nil {
		store.Templates = []model.ProjectTemplate{}
	}
	return store, nil
}

// LoadDefaultTemplates loads templates from the default path.
func LoadDefaultTemplates() (model.TemplateStore, error) {
	path, err := DefaultTemplatePath()
	if err != nil {
		return model.NewTemplateStore(), err
	}
	return LoadTemplates(path)
}

// SaveDefaultTemplates saves templates to the default path.
func SaveDefaultTemplates(store model.TemplateStore) error {
	path, err := DefaultTemplatePath()
	if err != nil {
		return err
	}
	return SaveTemplates(path, store)
}
