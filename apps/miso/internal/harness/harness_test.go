package harness

import "testing"

func TestDetect(t *testing.T) {
	original := lookPath
	t.Cleanup(func() { lookPath = original })

	// only claude and pnpm-style multi-bin cursor resolve
	installed := map[string]bool{"claude": true, "cursor-agent": true}
	lookPath = func(bin string) (string, error) {
		if installed[bin] {
			return "/usr/bin/" + bin, nil
		}
		return "", errNotFound
	}

	got := Detect()
	if len(got) != 2 {
		t.Fatalf("expected 2 harnesses, got %d: %+v", len(got), got)
	}
	if got[0].Agent != "claude-code" || got[1].Agent != "cursor" {
		t.Errorf("wrong agents/order: %+v", got)
	}
}

// the skills CLI rejects any -a id outside its own list and exits 1
func TestDetectAgentIDs(t *testing.T) {
	original := lookPath
	t.Cleanup(func() { lookPath = original })
	lookPath = func(bin string) (string, error) { return "/usr/bin/" + bin, nil }

	want := []string{"claude-code", "opencode", "codex", "gemini-cli", "cursor"}
	got := Detect()
	if len(got) != len(want) {
		t.Fatalf("expected %d harnesses, got %d: %+v", len(want), len(got), got)
	}
	for i, entry := range got {
		if entry.Agent != want[i] {
			t.Errorf("harness %d: agent = %q, want %q", i, entry.Agent, want[i])
		}
	}
}

func TestDetectNone(t *testing.T) {
	original := lookPath
	t.Cleanup(func() { lookPath = original })
	lookPath = func(string) (string, error) { return "", errNotFound }
	if got := Detect(); len(got) != 0 {
		t.Errorf("expected none, got %+v", got)
	}
}
