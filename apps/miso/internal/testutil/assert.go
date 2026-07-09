package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Equal fails the test when got != want.
func Equal[T comparable](t testing.TB, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// NoError fails the test when err is non-nil.
func NoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ErrorContains fails the test when err is nil or its message lacks substr.
func ErrorContains(t testing.TB, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error containing %q, got nil", substr)
		return
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("error %q does not contain %q", err.Error(), substr)
	}
}

// WriteFiles creates empty files with the given names under dir.
func WriteFiles(t testing.TB, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}
