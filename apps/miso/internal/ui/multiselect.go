package ui

import "charm.land/lipgloss/v2"

// MultiSelectHeight is the Height a huh MultiSelect needs to show every option.
// Without one, huh v2.0.3 sizes the list to the option count and then
// subtracts the title and description lines, hiding the last of them. Counts
// unwrapped lines, so a terminal narrow enough to wrap either still hides rows.
func MultiSelectHeight(title, description string, optionCount int) int {
	return lipgloss.Height(title) + lipgloss.Height(description) + optionCount
}
