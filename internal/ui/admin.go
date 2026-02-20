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
			a.applyTheme()
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

// showTemplateManager displays the project template management dialog.
func (a *App) showTemplateManager() {
	listContainer := container.NewVBox()

	var refreshList func()
	refreshList = func() {
		listContainer.RemoveAll()

		if len(a.templates.Templates) == 0 {
			listContainer.Add(widget.NewLabel("No templates saved yet."))
			return
		}

		header := container.NewGridWithColumns(4,
			widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Parts / Stocks", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
			widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{}),
		)
		listContainer.Add(header)
		listContainer.Add(widget.NewSeparator())

		for i := range a.templates.Templates {
			idx := i
			tmpl := a.templates.Templates[idx]

			loadBtn := widget.NewButton("Load", func() {
				dialog.ShowConfirm("Load Template",
					fmt.Sprintf("Load template %q? This will replace your current project.", tmpl.Name),
					func(ok bool) {
						if !ok {
							return
						}
						a.saveState("Load Template")
						proj := tmpl.ToProject(tmpl.Name)
						a.project = proj
						a.refreshPartsList()
						a.refreshStockList()
						a.refreshResults()
						dialog.ShowInformation("Template Loaded",
							fmt.Sprintf("Loaded template %q with %d parts and %d stock sheets.",
								tmpl.Name, len(tmpl.Parts), len(tmpl.Stocks)),
							a.window)
					},
					a.window,
				)
			})

			deleteBtn := widget.NewButton("Delete", func() {
				dialog.ShowConfirm("Delete Template",
					fmt.Sprintf("Delete template %q?", tmpl.Name),
					func(ok bool) {
						if !ok {
							return
						}
						a.templates.Remove(tmpl.ID)
						if err := project.SaveDefaultTemplates(a.templates); err != nil {
							dialog.ShowError(fmt.Errorf("failed to save templates: %w", err), a.window)
						}
						refreshList()
					},
					a.window,
				)
			})

			row := container.NewGridWithColumns(4,
				widget.NewLabel(tmpl.Name),
				widget.NewLabel(fmt.Sprintf("%d parts, %d stocks", len(tmpl.Parts), len(tmpl.Stocks))),
				loadBtn,
				deleteBtn,
			)
			listContainer.Add(row)
		}
	}

	refreshList()

	// Save current project as template button
	saveAsBtn := widget.NewButton("Save Current Project as Template...", func() {
		a.showSaveAsTemplateDialog(refreshList)
	})

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("Manage project templates. Save your current project configuration\nas a reusable template, or load a template to start a new project."),
			widget.NewSeparator(),
			saveAsBtn,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		container.NewVScroll(listContainer),
	)

	d := dialog.NewCustom("Project Templates", "Close", content, a.window)
	d.Resize(fyne.NewSize(600, 450))
	d.Show()
}

// showSaveAsTemplateDialog shows a form to save the current project as a template.
func (a *App) showSaveAsTemplateDialog(onSave func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(a.project.Name)
	nameEntry.SetPlaceHolder("Template name")

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Optional description")
	descEntry.SetMinRowsVisible(3)

	form := dialog.NewForm("Save as Template", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", nameEntry),
			widget.NewFormItem("Description", descEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := nameEntry.Text
			if name == "" {
				dialog.ShowError(fmt.Errorf("template name cannot be empty"), a.window)
				return
			}

			tmpl := model.NewProjectTemplate(
				name,
				descEntry.Text,
				a.project.Parts,
				a.project.Stocks,
				a.project.Settings,
			)
			a.templates.Add(tmpl)

			if err := project.SaveDefaultTemplates(a.templates); err != nil {
				dialog.ShowError(fmt.Errorf("failed to save templates: %w", err), a.window)
				return
			}

			dialog.ShowInformation("Template Saved",
				fmt.Sprintf("Template %q saved with %d parts and %d stock sheets.",
					name, len(tmpl.Parts), len(tmpl.Stocks)),
				a.window)

			if onSave != nil {
				onSave()
			}
		},
		a.window,
	)
	form.Resize(fyne.NewSize(450, 300))
	form.Show()
}
