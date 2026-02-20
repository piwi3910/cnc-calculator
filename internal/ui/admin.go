package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/piwi3910/SlabCut/internal/model"
	"github.com/piwi3910/SlabCut/internal/project"
)

// showSettingsDialog displays the application settings editor.
func (a *App) showSettingsDialog() {
	cfg := a.config

	// Helper to create a float entry bound to a pointer
	floatEntry := func(val *float64) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(fmt.Sprintf("%.1f", *val))
		e.OnChanged = func(text string) {
			if v, err := strconv.ParseFloat(text, 64); err == nil {
				*val = v
			}
		}
		return e
	}

	intEntry := func(val *int) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(fmt.Sprintf("%d", *val))
		e.OnChanged = func(text string) {
			if v, err := strconv.Atoi(text); err == nil {
				*val = v
			}
		}
		return e
	}

	// GCode profile selector
	profileNames := model.GetProfileNames()
	profileSelect := widget.NewSelect(profileNames, func(selected string) {
		cfg.DefaultGCodeProfile = selected
	})
	profileSelect.SetSelected(cfg.DefaultGCodeProfile)

	// Theme selector
	themeSelect := widget.NewSelect([]string{"system", "light", "dark"}, func(selected string) {
		cfg.Theme = selected
	})
	themeSelect.SetSelected(cfg.Theme)

	// Auto-save interval
	autoSaveEntry := intEntry(&cfg.AutoSaveInterval)

	formItems := []*widget.FormItem{
		widget.NewFormItem("Theme", themeSelect),
		widget.NewFormItem("Auto-Save Interval (min, 0=off)", autoSaveEntry),
		widget.NewFormItem("", widget.NewSeparator()),
		widget.NewFormItem("Default Kerf Width (mm)", floatEntry(&cfg.DefaultKerfWidth)),
		widget.NewFormItem("Default Edge Trim (mm)", floatEntry(&cfg.DefaultEdgeTrim)),
		widget.NewFormItem("Default Tool Diameter (mm)", floatEntry(&cfg.DefaultToolDiameter)),
		widget.NewFormItem("Default Feed Rate (mm/min)", floatEntry(&cfg.DefaultFeedRate)),
		widget.NewFormItem("Default Plunge Rate (mm/min)", floatEntry(&cfg.DefaultPlungeRate)),
		widget.NewFormItem("Default Spindle Speed (RPM)", intEntry(&cfg.DefaultSpindleSpeed)),
		widget.NewFormItem("Default Safe Z (mm)", floatEntry(&cfg.DefaultSafeZ)),
		widget.NewFormItem("Default Cut Depth (mm)", floatEntry(&cfg.DefaultCutDepth)),
		widget.NewFormItem("Default Pass Depth (mm)", floatEntry(&cfg.DefaultPassDepth)),
		widget.NewFormItem("Default GCode Profile", profileSelect),
	}

	d := dialog.NewForm("Settings", "Save", "Cancel", formItems,
		func(ok bool) {
			if !ok {
				return
			}
			a.config = cfg
			if err := a.saveConfig(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to save settings: %w", err), a.window)
			} else {
				dialog.ShowInformation("Settings Saved", "Application settings have been saved.", a.window)
			}
		},
		a.window,
	)
	d.Resize(fyne.NewSize(500, 550))
	d.Show()
}

// showImportExportDialog displays the import/export data dialog.
func (a *App) showImportExportDialog() {
	exportBtn := widget.NewButton("Export All Data...", func() {
		d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			defer writer.Close()
			path := writer.URI().Path()
			if err := project.ExportAllData(path, a.config); err != nil {
				dialog.ShowError(err, a.window)
			} else {
				dialog.ShowInformation("Export Complete",
					fmt.Sprintf("All application data exported to:\n%s", path), a.window)
			}
		}, a.window)
		d.SetFileName("slabcut-backup.json")
		d.Show()
	})

	importBtn := widget.NewButton("Import All Data...", func() {
		dialog.ShowConfirm("Import Data",
			"Importing data will replace your current application settings.\n\nAre you sure you want to continue?",
			func(ok bool) {
				if !ok {
					return
				}
				d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
					if err != nil || reader == nil {
						return
					}
					defer reader.Close()
					path := reader.URI().Path()
					backup, err := project.ImportAllData(path)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					a.config = backup.Config
					if err := a.saveConfig(); err != nil {
						dialog.ShowError(fmt.Errorf("failed to save imported settings: %w", err), a.window)
						return
					}
					dialog.ShowInformation("Import Complete",
						fmt.Sprintf("Data imported successfully from backup created at %s.", backup.CreatedAt), a.window)
				}, a.window)
				d.Show()
			},
			a.window,
		)
	})

	content := container.NewVBox(
		widget.NewLabel("Export all application data (settings, preferences) to a backup file,\nor import from a previously exported backup."),
		widget.NewSeparator(),
		exportBtn,
		widget.NewSeparator(),
		importBtn,
	)

	d := dialog.NewCustom("Import / Export Data", "Close", content, a.window)
	d.Resize(fyne.NewSize(450, 250))
	d.Show()
}

// saveConfig persists the current app config to disk.
func (a *App) saveConfig() error {
	return project.SaveAppConfig(project.DefaultConfigPath(), a.config)
}
