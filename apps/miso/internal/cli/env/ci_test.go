package env

import (
	"slices"
	"testing"
)

func TestResolveCI(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		ciValue      string
		ciSet        bool
		expectedCI   bool
		expectedArgs []string
	}{
		{name: "leading flag", args: []string{"--ci", "format", "@web"}, expectedCI: true, expectedArgs: []string{"format", "@web"}},
		{name: "flag after the command belongs to the tool", args: []string{"test", "--ci"}, expectedCI: false, expectedArgs: []string{"test", "--ci"}},
		{name: "flag alone", args: []string{"--ci"}, expectedCI: true, expectedArgs: []string{}},
		{name: "ci unset", args: []string{"format"}, expectedCI: false, expectedArgs: []string{"format"}},
		{name: "ci empty", args: []string{"format"}, ciSet: true, ciValue: "", expectedCI: false, expectedArgs: []string{"format"}},
		{name: "ci zero", args: []string{"format"}, ciSet: true, ciValue: "0", expectedCI: false, expectedArgs: []string{"format"}},
		{name: "ci false", args: []string{"format"}, ciSet: true, ciValue: "false", expectedCI: false, expectedArgs: []string{"format"}},
		{name: "ci false uppercase", args: []string{"format"}, ciSet: true, ciValue: "FALSE", expectedCI: false, expectedArgs: []string{"format"}},
		{name: "ci true", args: []string{"format"}, ciSet: true, ciValue: "true", expectedCI: true, expectedArgs: []string{"format"}},
		{name: "ci one", args: []string{"format", "--ci"}, ciSet: true, ciValue: "1", expectedCI: true, expectedArgs: []string{"format", "--ci"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				if key == "CI" && tt.ciSet {
					return tt.ciValue, true
				}
				return "", false
			}
			gotCI, gotArgs := ResolveCI(tt.args, lookup)
			if gotCI != tt.expectedCI {
				t.Errorf("ci = %v, want %v", gotCI, tt.expectedCI)
			}
			if !slices.Equal(gotArgs, tt.expectedArgs) {
				t.Errorf("args = %q, want %q", gotArgs, tt.expectedArgs)
			}
		})
	}
}
