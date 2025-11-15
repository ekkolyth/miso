package theme

import "github.com/charmbracelet/lipgloss"

// Styles centralizes lipgloss styles used across the CLI.
type Styles struct {
	Heading lipgloss.Style
	Accent  lipgloss.Style
	Muted   lipgloss.Style
	Label   lipgloss.Style
	Flavor  lipgloss.Style
}

// Default returns the default theme for miso.
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
