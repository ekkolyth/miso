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
	if got[0].Agent != "claude" || got[1].Agent != "cursor" {
		t.Errorf("wrong agents/order: %+v", got)
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
