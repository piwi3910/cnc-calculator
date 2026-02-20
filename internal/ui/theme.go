// Package ui provides the SlabCut application UI components.
//
// This file defines a custom compact Fyne theme for a professional, dense layout.

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// SlabCutTheme wraps the default Fyne theme with compact sizing overrides
// for a professional, information-dense CNC application layout.
type SlabCutTheme struct {
	base    fyne.Theme
	variant fyne.ThemeVariant
}

// NewSlabCutTheme creates a new SlabCutTheme with the system default variant.
func NewSlabCutTheme() *SlabCutTheme {
	return &SlabCutTheme{
		base:    theme.DefaultTheme(),
		variant: 0, // system default
	}
}

// NewSlabCutThemeWithVariant creates a SlabCutTheme with a specific light/dark variant.
func NewSlabCutThemeWithVariant(variant fyne.ThemeVariant) *SlabCutTheme {
	return &SlabCutTheme{
		base:    theme.DefaultTheme(),
		variant: variant,
	}
}

// SetVariant updates the theme variant (light/dark/system).
func (t *SlabCutTheme) SetVariant(variant fyne.ThemeVariant) {
	t.variant = variant
}

// Color delegates to the base theme with the stored variant.
func (t *SlabCutTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return t.base.Color(name, t.variant)
}

// Font delegates to the base theme.
func (t *SlabCutTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

// Icon delegates to the base theme.
func (t *SlabCutTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

// Size returns compact sizing overrides for a dense, professional layout.
func (t *SlabCutTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 12
	case theme.SizeNameCaptionText:
		return 9
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNamePadding:
		return 3
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameInlineIcon:
		return 16
	default:
		return t.base.Size(name)
	}
}
