package turbo

import "strings"

// SplitFlags returns the args to pass to turbo. miso's own flags go before the
// command, so everything here belongs to turbo.
//
// When chromeActive is true, --log-order (both --log-order=value and
// --log-order value forms) is stripped from the turbo args so the TUI can
// manage log ordering itself.
func SplitFlags(args []string, chromeActive bool) []string {
	turbo := make([]string, 0)

	skip := false
	for i, arg := range args {
		if skip {
			skip = false
			continue
		}

		// When chrome is active, strip --log-order in both forms.
		if chromeActive {
			// --log-order=value form
			if strings.HasPrefix(arg, "--log-order=") {
				continue
			}
			// --log-order value (space-separated) form
			if arg == "--log-order" {
				// Consume the next arg (the value) as well.
				if i+1 < len(args) {
					skip = true
				}
				continue
			}
		}

		turbo = append(turbo, arg)
	}

	return turbo
}
