package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// lipgloss styles used across cli
type Styles struct {
	Heading lipgloss.Style
	Accent  lipgloss.Style
	Muted   lipgloss.Style
	Label   lipgloss.Style
	Flavor  lipgloss.Style
	Bright  lipgloss.Style
	Error   lipgloss.Style
}

// palette for workspace/app labels
var LabelColors = []color.Color{
	lipgloss.Color("#7c3aed"), // purple
	lipgloss.Color("#3b82f6"), // blue
	lipgloss.Color("#f59e0b"), // amber
	lipgloss.Color("#10b981"), // emerald
	lipgloss.Color("#ef4444"), // red
	lipgloss.Color("#ec4899"), // pink
	lipgloss.Color("#06b6d4"), // cyan
	lipgloss.Color("#f97316"), // orange
}

// warning-level highlights (e.g. var names in validation errors)
var WarningColor = lipgloss.Color("#f59e0b") // amber

func Default() Styles {
	accent := lipgloss.Color("#a855f7")  // purple
	heading := lipgloss.Color("#ec4899") // pink
	muted := lipgloss.Color("#64748b")
	flavor := lipgloss.Color("#0ea5e9")      // blue
	bright := lipgloss.Color("#ffffff")      // white
	destructive := lipgloss.Color("#ef4444") // red

	return Styles{
		Heading: lipgloss.NewStyle().Bold(true).Foreground(heading),
		Accent:  lipgloss.NewStyle().Foreground(accent),
		Muted:   lipgloss.NewStyle().Foreground(muted),
		Label:   lipgloss.NewStyle().Bold(true).Foreground(accent),
		Flavor:  lipgloss.NewStyle().Foreground(flavor),
		Bright:  lipgloss.NewStyle().Foreground(bright),
		Error:   lipgloss.NewStyle().Bold(true).Foreground(destructive),
	}
}
