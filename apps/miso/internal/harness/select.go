package harness

import "charm.land/huh/v2"

// Select shows a multiselect of the given harnesses (all pre-checked) and returns
// the chosen agent ids.
func Select(harnesses []Harness) ([]string, error) {
	var chosen []string

	opts := make([]huh.Option[string], 0, len(harnesses))
	for _, entry := range harnesses {
		opts = append(opts, huh.NewOption(entry.Label, entry.Agent).Selected(true))
	}

	field := huh.NewMultiSelect[string]().
		Title("Install the miso skill to which harnesses?").
		Description("Space to toggle, Enter to confirm • ctrl+c to bail").
		Value(&chosen).
		Options(opts...)

	form := huh.NewForm(huh.NewGroup(field)).WithTheme(huh.ThemeFunc(huh.ThemeCharm))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return chosen, nil
}
