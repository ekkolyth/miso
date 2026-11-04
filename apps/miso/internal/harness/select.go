package harness

import (
	"charm.land/huh/v2"

	"github.com/ekkolyth/miso/internal/ui"
)

// Select shows a multiselect of the given harnesses (all pre-checked) and returns
// the chosen agent ids.
func Select(harnesses []Harness) ([]string, error) {
	var chosen []string

	form := huh.NewForm(huh.NewGroup(selectField(harnesses, &chosen))).WithTheme(huh.ThemeFunc(huh.ThemeCharm))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return chosen, nil
}

func selectField(harnesses []Harness, chosen *[]string) *huh.MultiSelect[string] {
	opts := make([]huh.Option[string], 0, len(harnesses))
	for _, entry := range harnesses {
		opts = append(opts, huh.NewOption(entry.Label, entry.Agent).Selected(true))
	}

	title := "Install the miso skill to which harnesses?"
	description := "Space to toggle, Enter to confirm • ctrl+c to bail"
	return huh.NewMultiSelect[string]().
		Title(title).
		Description(description).
		Value(chosen).
		Options(opts...).
		Height(ui.MultiSelectHeight(title, description, len(opts)))
}
