package turbo

import "strings"

// misoFlags is the canonical set of flags owned by miso and not passed to turbo.
var misoFlags = map[string]bool{
	"--env": true,
}

// SplitFlags partitions args into miso-owned flags and turbo passthrough flags.
//
// When tuiActive is true, --log-order (both --log-order=value and
// --log-order value forms) is stripped from the turbo args so the TUI can
// manage log ordering itself.
func SplitFlags(args []string, tuiActive bool) (miso []string, turbo []string) {
	miso = make([]string, 0)
	turbo = make([]string, 0)

	skip := false
	for i, arg := range args {
		if skip {
			skip = false
			continue
		}

		// Check if this is a miso-owned flag.
		if misoFlags[arg] {
			miso = append(miso, arg)
			continue
		}

		// When TUI is active, strip --log-order in both forms.
		if tuiActive {
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

	return miso, turbo
}
