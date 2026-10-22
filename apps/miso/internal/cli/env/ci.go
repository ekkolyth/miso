package env

import "strings"

const CIFlag = "--ci"

// only a leading --ci is miso's; after the command it belongs to the tool (jest --ci)
func ResolveCI(args []string, lookup func(string) (string, bool)) (bool, []string) {
	if len(args) > 0 && args[0] == CIFlag {
		return true, args[1:]
	}
	value, ok := lookup("CI")
	if !ok {
		return false, args
	}
	switch strings.ToLower(value) {
	case "", "0", "false":
		return false, args
	}
	return true, args
}
