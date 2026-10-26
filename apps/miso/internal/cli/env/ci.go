package env

import "strings"

const (
	CIFlag  = "--ci"
	EnvFlag = "--env"
)

type LeadingFlags struct {
	CI       bool
	Validate bool
}

// only flags before the command are miso's; after it they belong to the tool
// (jest --ci, wrangler --env)
func ParseLeadingFlags(args []string, lookup func(string) (string, bool)) (LeadingFlags, []string) {
	var flags LeadingFlags
loop:
	for len(args) > 0 {
		switch args[0] {
		case CIFlag:
			flags.CI = true
		case EnvFlag:
			flags.Validate = true
		default:
			break loop
		}
		args = args[1:]
	}
	if !flags.CI {
		if value, ok := lookup("CI"); ok {
			switch strings.ToLower(value) {
			case "", "0", "false":
			default:
				flags.CI = true
			}
		}
	}
	return flags, args
}
