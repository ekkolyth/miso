package ui

import "github.com/charmbracelet/lipgloss"

// lipgloss styles used across cli
type Styles struct {
	Heading lipgloss.Style
	Accent  lipgloss.Style
	Muted   lipgloss.Style
	Label   lipgloss.Style
	Flavor  lipgloss.Style
}

// LabelColors is the palette used to color workspace/app labels (matches merged TUI view).
var LabelColors = []lipgloss.Color{
	lipgloss.Color("#7c3aed"), // purple
	lipgloss.Color("#3b82f6"), // blue
	lipgloss.Color("#f59e0b"), // amber
	lipgloss.Color("#10b981"), // emerald
	lipgloss.Color("#ef4444"), // red
	lipgloss.Color("#ec4899"), // pink
	lipgloss.Color("#06b6d4"), // cyan
	lipgloss.Color("#f97316"), // orange
}

// WarningColor is used for warning-level highlights (e.g. variable names in validation errors).
var WarningColor = lipgloss.Color("#f59e0b") // amber

// return default miso theme
func Default() Styles {
	accent := lipgloss.Color("#7c3aed")  // purple
	heading := lipgloss.Color("#ec4899") // pink
	muted := lipgloss.Color("#64748b")
	flavor := lipgloss.Color("#0ea5e9") // blue

	return Styles{
		Heading: lipgloss.NewStyle().Bold(true).Foreground(heading),
		Accent:  lipgloss.NewStyle().Foreground(accent),
		Muted:   lipgloss.NewStyle().Foreground(muted),
		Label:   lipgloss.NewStyle().Bold(true).Foreground(accent),
		Flavor:  lipgloss.NewStyle().Foreground(flavor),
	}
}
