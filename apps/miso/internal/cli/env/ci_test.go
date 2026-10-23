package env

import (
	"slices"
	"testing"
)

func TestParseLeadingFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		ciValue      string
		ciSet        bool
		expected     LeadingFlags
		expectedArgs []string
	}{
		{name: "leading ci", args: []string{"--ci", "format", "@web"}, expected: LeadingFlags{CI: true}, expectedArgs: []string{"format", "@web"}},
		{name: "leading env", args: []string{"--env", "dev"}, expected: LeadingFlags{Validate: true}, expectedArgs: []string{"dev"}},
		{name: "both in either order", args: []string{"--env", "--ci", "dev"}, expected: LeadingFlags{CI: true, Validate: true}, expectedArgs: []string{"dev"}},
		{name: "ci after the command belongs to the tool", args: []string{"test", "--ci"}, expectedArgs: []string{"test", "--ci"}},
		{name: "env after the command belongs to the tool", args: []string{"wrangler", "deploy", "--env", "production"}, expectedArgs: []string{"wrangler", "deploy", "--env", "production"}},
		{name: "flags alone", args: []string{"--ci", "--env"}, expected: LeadingFlags{CI: true, Validate: true}, expectedArgs: []string{}},
		{name: "ci unset", args: []string{"format"}, expectedArgs: []string{"format"}},
		{name: "ci empty", args: []string{"format"}, ciSet: true, ciValue: "", expectedArgs: []string{"format"}},
		{name: "ci zero", args: []string{"format"}, ciSet: true, ciValue: "0", expectedArgs: []string{"format"}},
		{name: "ci false", args: []string{"format"}, ciSet: true, ciValue: "false", expectedArgs: []string{"format"}},
		{name: "ci false uppercase", args: []string{"format"}, ciSet: true, ciValue: "FALSE", expectedArgs: []string{"format"}},
		{name: "ci true", args: []string{"format"}, ciSet: true, ciValue: "true", expected: LeadingFlags{CI: true}, expectedArgs: []string{"format"}},
		{name: "ci one", args: []string{"format", "--ci"}, ciSet: true, ciValue: "1", expected: LeadingFlags{CI: true}, expectedArgs: []string{"format", "--ci"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				if key == "CI" && tt.ciSet {
					return tt.ciValue, true
				}
				return "", false
			}
			got, gotArgs := ParseLeadingFlags(tt.args, lookup)
			if got != tt.expected {
				t.Errorf("flags = %+v, want %+v", got, tt.expected)
			}
			if !slices.Equal(gotArgs, tt.expectedArgs) {
				t.Errorf("args = %q, want %q", gotArgs, tt.expectedArgs)
			}
		})
	}
}
