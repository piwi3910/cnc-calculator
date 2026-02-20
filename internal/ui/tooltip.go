// Package ui provides the SlabCut application UI components.
//
// This file provides tooltip-enabled button helpers using the fyne-tooltip library.

package ui

import (
	"fyne.io/fyne/v2"

	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

// newIconButtonWithTooltip creates an icon-only button with a tooltip that appears on hover.
func newIconButtonWithTooltip(icon fyne.Resource, tooltip string, tapped func()) *ttwidget.Button {
	btn := ttwidget.NewButtonWithIcon("", icon, tapped)
	btn.SetToolTip(tooltip)
	return btn
}
