package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/piwi3910/SlabCut/internal/model"
	"github.com/piwi3910/SlabCut/internal/project"
)

// showProfileManager opens the profile management dialog where users can
// view, create, edit, duplicate, delete, import, and export GCode profiles.
func (a *App) showProfileManager() {
	w := fyne.CurrentApp().NewWindow("GCode Profile Manager")
	w.Resize(fyne.NewSize(700, 500))

	var listWidget *widget.List
	var selectedIdx int = -1
	var detailContainer *fyne.Container

	profiles := model.AllProfiles()

	detailContainer = container.NewVBox(
		widget.NewLabel("Select a profile to view details."),
	)

	listWidget = widget.NewList(
		func() int {
			return len(profiles)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.DocumentIcon()),
				widget.NewLabel("Profile Name"),
				layout.NewSpacer(),
				widget.NewLabel("(built-in)"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			nameLabel := box.Objects[1].(*widget.Label)
			tagLabel := box.Objects[3].(*widget.Label)
			p := profiles[id]
			nameLabel.SetText(p.Name)
			if p.IsBuiltIn {
				tagLabel.SetText("(built-in)")
			} else {
				tagLabel.SetText("(custom)")
			}
		},
	)

	listWidget.OnSelected = func(id widget.ListItemID) {
		selectedIdx = id
		p := profiles[id]
		a.showProfileDetail(detailContainer, p, w, func() {
			profiles = model.AllProfiles()
			listWidget.Refresh()
			listWidget.UnselectAll()
			detailContainer.RemoveAll()
			detailContainer.Add(widget.NewLabel("Select a profile to view details."))
			detailContainer.Refresh()
			a.refreshProfileSelector()
		})
	}

	// Action buttons
	newBtn := widget.NewButtonWithIcon("New", theme.ContentAddIcon(), func() {
		a.showNewProfileDialog(w, func() {
			profiles = model.AllProfiles()
			listWidget.Refresh()
			a.refreshProfileSelector()
		})
	})

	duplicateBtn := widget.NewButtonWithIcon("Duplicate", theme.ContentCopyIcon(), func() {
		if selectedIdx < 0 || selectedIdx >= len(profiles) {
			dialog.ShowInformation("No Selection", "Select a profile to duplicate.", w)
			return
		}
		a.duplicateProfile(profiles[selectedIdx], w, func() {
			profiles = model.AllProfiles()
			listWidget.Refresh()
			a.refreshProfileSelector()
		})
	})

	importBtn := widget.NewButtonWithIcon("Import", theme.FolderOpenIcon(), func() {
		a.importProfileDialog(w, func() {
			profiles = model.AllProfiles()
			listWidget.Refresh()
			a.refreshProfileSelector()
		})
	})

	exportBtn := widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), func() {
		if selectedIdx < 0 || selectedIdx >= len(profiles) {
			dialog.ShowInformation("No Selection", "Select a profile to export.", w)
			return
		}
		a.exportProfileDialog(profiles[selectedIdx], w)
	})

	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if selectedIdx < 0 || selectedIdx >= len(profiles) {
			dialog.ShowInformation("No Selection", "Select a profile to delete.", w)
			return
		}
		p := profiles[selectedIdx]
		if p.IsBuiltIn {
			dialog.ShowInformation("Cannot Delete", "Built-in profiles cannot be deleted.", w)
			return
		}
		dialog.ShowConfirm("Delete Profile",
			fmt.Sprintf("Delete custom profile %q?", p.Name),
			func(ok bool) {
				if !ok {
					return
				}
				if err := model.RemoveCustomProfile(p.Name); err != nil {
					dialog.ShowError(err, w)
					return
				}
				a.persistCustomProfiles(w)
				profiles = model.AllProfiles()
				selectedIdx = -1
				listWidget.Refresh()
				listWidget.UnselectAll()
				detailContainer.RemoveAll()
				detailContainer.Add(widget.NewLabel("Select a profile to view details."))
				detailContainer.Refresh()
				a.refreshProfileSelector()
			},
			w,
		)
	})

	toolbar := container.NewHBox(newBtn, duplicateBtn, importBtn, exportBtn, deleteBtn)

	listPanel := container.NewBorder(
		widget.NewLabelWithStyle("Profiles", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		toolbar,
		nil, nil,
		listWidget,
	)

	detailScroll := container.NewVScroll(detailContainer)
	detailPanel := container.NewBorder(
		widget.NewLabelWithStyle("Profile Details", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		detailScroll,
	)

	split := container.NewHSplit(listPanel, detailPanel)
	split.SetOffset(0.35)

	w.SetContent(split)
	w.Show()
}

// showProfileDetail populates the detail pane with profile information and an edit button.
func (a *App) showProfileDetail(c *fyne.Container, p model.GCodeProfile, w fyne.Window, onChanged func()) {
	c.RemoveAll()

	startCode := strings.Join(p.StartCode, "\n")
	endCode := strings.Join(p.EndCode, "\n")

	info := container.NewVBox(
		widget.NewLabelWithStyle(p.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(p.Description),
		widget.NewSeparator(),

		container.NewGridWithColumns(2,
			widget.NewLabelWithStyle("Units:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel(p.Units),
			widget.NewLabelWithStyle("Decimal Places:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel(fmt.Sprintf("%d", p.DecimalPlaces)),
			widget.NewLabelWithStyle("Leading Zeros:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel(fmt.Sprintf("%v", p.LeadingZeros)),
		),

		widget.NewSeparator(),
		widget.NewLabelWithStyle("Motion Commands", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Rapid Move:"), widget.NewLabel(p.RapidMove),
			widget.NewLabel("Feed Move:"), widget.NewLabel(p.FeedMove),
			widget.NewLabel("Absolute Mode:"), widget.NewLabel(p.AbsoluteMode),
			widget.NewLabel("Feed Mode:"), widget.NewLabel(p.FeedMode),
		),

		widget.NewSeparator(),
		widget.NewLabelWithStyle("Spindle Commands", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Spindle Start:"), widget.NewLabel(p.SpindleStart),
			widget.NewLabel("Spindle Stop:"), widget.NewLabel(p.SpindleStop),
		),

		widget.NewSeparator(),
		widget.NewLabelWithStyle("Homing Commands", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Home All:"), widget.NewLabel(p.HomeAll),
			widget.NewLabel("Home XY:"), widget.NewLabel(p.HomeXY),
		),

		widget.NewSeparator(),
		widget.NewLabelWithStyle("Comment Style", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Prefix:"), widget.NewLabel(fmt.Sprintf("%q", p.CommentPrefix)),
			widget.NewLabel("Suffix:"), widget.NewLabel(fmt.Sprintf("%q", p.CommentSuffix)),
		),

		widget.NewSeparator(),
		widget.NewLabelWithStyle("Start Code", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(startCode),
		widget.NewLabelWithStyle("End Code", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(endCode),
	)

	if !p.IsBuiltIn {
		editBtn := widget.NewButtonWithIcon("Edit Profile", theme.DocumentCreateIcon(), func() {
			a.showEditProfileDialog(p, w, onChanged)
		})
		c.Add(editBtn)
	} else {
		c.Add(widget.NewLabel("Built-in profiles are read-only. Duplicate to customize."))
	}

	c.Add(info)
	c.Refresh()
}

// showNewProfileDialog shows a dialog to create a new custom profile.
func (a *App) showNewProfileDialog(w fyne.Window, onCreated func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("My Custom Profile")

	form := dialog.NewForm("New Custom Profile", "Create", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Profile Name", nameEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			if name == "" {
				dialog.ShowError(fmt.Errorf("profile name cannot be empty"), w)
				return
			}
			profile := model.NewCustomProfile(name)
			if err := model.AddCustomProfile(profile); err != nil {
				dialog.ShowError(err, w)
				return
			}
			a.persistCustomProfiles(w)
			onCreated()
		},
		w,
	)
	form.Resize(fyne.NewSize(400, 150))
	form.Show()
}

// duplicateProfile creates a copy of an existing profile with a new name.
func (a *App) duplicateProfile(source model.GCodeProfile, w fyne.Window, onCreated func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(source.Name + " (Copy)")

	form := dialog.NewForm("Duplicate Profile", "Create", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("New Profile Name", nameEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			if name == "" {
				dialog.ShowError(fmt.Errorf("profile name cannot be empty"), w)
				return
			}
			dup := source
			dup.Name = name
			dup.IsBuiltIn = false
			dup.Description = "Copy of " + source.Name
			// Deep copy slices
			dup.StartCode = make([]string, len(source.StartCode))
			copy(dup.StartCode, source.StartCode)
			dup.EndCode = make([]string, len(source.EndCode))
			copy(dup.EndCode, source.EndCode)

			if err := model.AddCustomProfile(dup); err != nil {
				dialog.ShowError(err, w)
				return
			}
			a.persistCustomProfiles(w)
			onCreated()
		},
		w,
	)
	form.Resize(fyne.NewSize(400, 150))
	form.Show()
}

// showEditProfileDialog shows a comprehensive editing dialog for a custom profile.
func (a *App) showEditProfileDialog(p model.GCodeProfile, w fyne.Window, onSaved func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(p.Name)

	descEntry := widget.NewEntry()
	descEntry.SetText(p.Description)

	unitsSelect := widget.NewSelect([]string{"mm", "inches"}, nil)
	unitsSelect.SetSelected(p.Units)

	decimalEntry := widget.NewEntry()
	decimalEntry.SetText(fmt.Sprintf("%d", p.DecimalPlaces))

	leadingZerosCheck := widget.NewCheck("", nil)
	leadingZerosCheck.Checked = p.LeadingZeros

	rapidEntry := widget.NewEntry()
	rapidEntry.SetText(p.RapidMove)

	feedEntry := widget.NewEntry()
	feedEntry.SetText(p.FeedMove)

	absoluteEntry := widget.NewEntry()
	absoluteEntry.SetText(p.AbsoluteMode)

	feedModeEntry := widget.NewEntry()
	feedModeEntry.SetText(p.FeedMode)

	spindleStartEntry := widget.NewEntry()
	spindleStartEntry.SetText(p.SpindleStart)

	spindleStopEntry := widget.NewEntry()
	spindleStopEntry.SetText(p.SpindleStop)

	homeAllEntry := widget.NewEntry()
	homeAllEntry.SetText(p.HomeAll)

	homeXYEntry := widget.NewEntry()
	homeXYEntry.SetText(p.HomeXY)

	commentPrefixEntry := widget.NewEntry()
	commentPrefixEntry.SetText(p.CommentPrefix)

	commentSuffixEntry := widget.NewEntry()
	commentSuffixEntry.SetText(p.CommentSuffix)

	startCodeEntry := widget.NewMultiLineEntry()
	startCodeEntry.SetText(strings.Join(p.StartCode, "\n"))
	startCodeEntry.SetMinRowsVisible(4)

	endCodeEntry := widget.NewMultiLineEntry()
	endCodeEntry.SetText(strings.Join(p.EndCode, "\n"))
	endCodeEntry.SetMinRowsVisible(4)

	// Preview section
	previewLabel := widget.NewMultiLineEntry()
	previewLabel.Disable()
	previewLabel.SetMinRowsVisible(8)

	updatePreview := func() {
		var preview strings.Builder
		preview.WriteString(commentPrefixEntry.Text + " Sample GCode Preview" + commentSuffixEntry.Text + "\n")
		preview.WriteString(commentPrefixEntry.Text + " Profile: " + nameEntry.Text + commentSuffixEntry.Text + "\n\n")

		for _, line := range splitLines(startCodeEntry.Text) {
			if line != "" {
				preview.WriteString(line + "\n")
			}
		}
		preview.WriteString(spindleStartEntry.Text + "\n")
		preview.WriteString(rapidEntry.Text + " X0.000 Y0.000\n")
		preview.WriteString(feedEntry.Text + " X100.000 Y50.000 F1500.000\n")
		preview.WriteString("\n")
		for _, line := range splitLines(endCodeEntry.Text) {
			if line != "" {
				preview.WriteString(line + "\n")
			}
		}
		preview.WriteString(spindleStopEntry.Text + "\n")
		previewLabel.SetText(preview.String())
	}
	updatePreview()

	previewBtn := widget.NewButtonWithIcon("Refresh Preview", theme.ViewRefreshIcon(), func() {
		updatePreview()
	})

	// Build tabbed form
	generalTab := container.NewTabItem("General", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Name"), nameEntry,
			widget.NewLabel("Description"), descEntry,
			widget.NewLabel("Units"), unitsSelect,
			widget.NewLabel("Decimal Places"), decimalEntry,
			widget.NewLabel("Leading Zeros"), leadingZerosCheck,
		),
	))

	motionTab := container.NewTabItem("Motion", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Rapid Move Command"), rapidEntry,
			widget.NewLabel("Feed Move Command"), feedEntry,
			widget.NewLabel("Absolute Mode"), absoluteEntry,
			widget.NewLabel("Feed Rate Mode"), feedModeEntry,
		),
	))

	spindleTab := container.NewTabItem("Spindle / Homing", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Spindle Start (use %d for RPM)"), spindleStartEntry,
			widget.NewLabel("Spindle Stop"), spindleStopEntry,
			widget.NewLabel("Home All Axes"), homeAllEntry,
			widget.NewLabel("Home XY"), homeXYEntry,
		),
	))

	commentsTab := container.NewTabItem("Comments", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Comment Prefix"), commentPrefixEntry,
			widget.NewLabel("Comment Suffix"), commentSuffixEntry,
		),
	))

	codeTab := container.NewTabItem("Start/End Code", container.NewVBox(
		widget.NewLabelWithStyle("Start Code (one command per line)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		startCodeEntry,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("End Code (one command per line)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		endCodeEntry,
	))

	previewTab := container.NewTabItem("Preview", container.NewVBox(
		previewBtn,
		previewLabel,
	))

	tabs := container.NewAppTabs(generalTab, motionTab, spindleTab, commentsTab, codeTab, previewTab)

	// Save and Cancel buttons
	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("profile name cannot be empty"), w)
			return
		}

		decimals, err := strconv.Atoi(decimalEntry.Text)
		if err != nil || decimals < 0 || decimals > 10 {
			dialog.ShowError(fmt.Errorf("decimal places must be a number between 0 and 10"), w)
			return
		}

		// If name changed, remove old profile first
		if name != p.Name {
			_ = model.RemoveCustomProfile(p.Name)
		}

		updated := model.GCodeProfile{
			Name:          name,
			Description:   descEntry.Text,
			IsBuiltIn:     false,
			Units:         unitsSelect.Selected,
			StartCode:     splitLines(startCodeEntry.Text),
			SpindleStart:  spindleStartEntry.Text,
			SpindleStop:   spindleStopEntry.Text,
			HomeAll:       homeAllEntry.Text,
			HomeXY:        homeXYEntry.Text,
			AbsoluteMode:  absoluteEntry.Text,
			FeedMode:      feedModeEntry.Text,
			RapidMove:     rapidEntry.Text,
			FeedMove:      feedEntry.Text,
			EndCode:       splitLines(endCodeEntry.Text),
			CommentPrefix: commentPrefixEntry.Text,
			CommentSuffix: commentSuffixEntry.Text,
			DecimalPlaces: decimals,
			LeadingZeros:  leadingZerosCheck.Checked,
		}

		if err := model.AddCustomProfile(updated); err != nil {
			dialog.ShowError(err, w)
			return
		}
		a.persistCustomProfiles(w)
		onSaved()
	})
	saveBtn.Importance = widget.HighImportance

	content := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), saveBtn),
		nil, nil,
		tabs,
	)

	editWindow := fyne.CurrentApp().NewWindow("Edit Profile: " + p.Name)
	editWindow.SetContent(content)
	editWindow.Resize(fyne.NewSize(600, 500))
	editWindow.Show()
}

// importProfileDialog opens a file dialog to import a profile from JSON.
func (a *App) importProfileDialog(w fyne.Window, onImported func()) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		profile, err := project.ImportProfile(reader.URI().Path())
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to import profile: %w", err), w)
			return
		}

		if err := model.AddCustomProfile(profile); err != nil {
			dialog.ShowError(err, w)
			return
		}
		a.persistCustomProfiles(w)
		onImported()
		dialog.ShowInformation("Import Complete",
			fmt.Sprintf("Profile %q imported successfully.", profile.Name), w)
	}, w)
}

// exportProfileDialog opens a file save dialog to export a profile to JSON.
func (a *App) exportProfileDialog(p model.GCodeProfile, w fyne.Window) {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		if err := project.ExportProfile(writer.URI().Path(), p); err != nil {
			dialog.ShowError(fmt.Errorf("failed to export profile: %w", err), w)
			return
		}
		dialog.ShowInformation("Export Complete",
			fmt.Sprintf("Profile %q exported successfully.", p.Name), w)
	}, w)
	d.SetFileName(strings.ReplaceAll(strings.ToLower(p.Name), " ", "_") + "_profile.json")
	d.Show()
}

// persistCustomProfiles saves the current custom profiles to disk.
func (a *App) persistCustomProfiles(w fyne.Window) {
	if err := project.SaveCustomProfilesToDefault(model.CustomProfiles); err != nil {
		dialog.ShowError(fmt.Errorf("failed to save profiles: %w", err), w)
	}
}

// splitLines splits a multiline string into non-empty lines.
func splitLines(text string) []string {
	raw := strings.Split(text, "\n")
	var lines []string
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
