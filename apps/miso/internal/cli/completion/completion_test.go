package completion

import "testing"

func TestGetCandidates_IncludesEveryBuiltin(t *testing.T) {
	got := getCandidates("miso", "", t.TempDir())
	set := map[string]bool{}
	for _, c := range got {
		set[c] = true
	}
	for _, want := range []string{
		"install", "i", "add", "remove", "rm", "run", "dev", "scripts", "env",
		"init", "upgrade", "skills", "completion", "version", "v", "help",
	} {
		if !set[want] {
			t.Errorf("completion missing built-in %q", want)
		}
	}
	if set["misox"] {
		t.Error("completion must not offer misox")
	}
	if set["__complete"] {
		t.Error("completion must not offer __complete")
	}
}
