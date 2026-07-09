package cli

import (
	"strings"
	"testing"
)

func TestLookupBuiltin(t *testing.T) {
	cases := []struct {
		token  string
		wantOK bool
		canon  string
	}{
		{"version", true, "version"},
		{"v", true, "version"},
		{"remove", true, "remove"},
		{"rm", true, "remove"},
		{"install", true, "install"},
		{"i", true, "install"},
		{"help", true, "help"},
		{"misox", false, ""},
		{"__complete", false, ""},
		{"nonsense", false, ""},
	}
	for _, tc := range cases {
		got, ok := LookupBuiltin(tc.token)
		if ok != tc.wantOK {
			t.Errorf("LookupBuiltin(%q) ok = %v, want %v", tc.token, ok, tc.wantOK)
			continue
		}
		if ok && got.Name != tc.canon {
			t.Errorf("LookupBuiltin(%q) canonical = %q, want %q", tc.token, got.Name, tc.canon)
		}
	}
}

func TestBuiltinNames(t *testing.T) {
	names := BuiltinNames()
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	for _, want := range []string{"install", "i", "add", "remove", "rm", "run", "dev", "scripts", "env", "init", "upgrade", "skills", "completion", "version", "v", "help"} {
		if !set[want] {
			t.Errorf("BuiltinNames missing %q", want)
		}
	}
	if set["misox"] {
		t.Error("BuiltinNames must not include misox")
	}
	if set["__complete"] {
		t.Error("BuiltinNames must not include __complete")
	}
}

func TestBuiltinsHaveNameAndSummary(t *testing.T) {
	for _, c := range Builtins {
		if strings.TrimSpace(c.Name) == "" {
			t.Errorf("builtin with empty name: %+v", c)
		}
		if strings.TrimSpace(c.Summary) == "" {
			t.Errorf("builtin %q has empty summary", c.Name)
		}
	}
}
