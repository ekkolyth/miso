package tui

import (
	"slices"
	"testing"
)

func TestDelegatedColorEnv(t *testing.T) {
	t.Run("forces FORCE_COLOR when unset", func(t *testing.T) {
		got := delegatedColorEnv([]string{"PATH=/bin", "TERM=xterm-256color"})
		if !slices.Contains(got, "FORCE_COLOR=1") {
			t.Errorf("expected FORCE_COLOR=1 to be appended, got %v", got)
		}
	})

	t.Run("respects explicit NO_COLOR", func(t *testing.T) {
		got := delegatedColorEnv([]string{"PATH=/bin", "NO_COLOR=1"})
		if slices.Contains(got, "FORCE_COLOR=1") {
			t.Errorf("must not force color when NO_COLOR is set, got %v", got)
		}
	})

	t.Run("leaves a pre-set FORCE_COLOR untouched", func(t *testing.T) {
		base := []string{"PATH=/bin", "FORCE_COLOR=3"}
		got := delegatedColorEnv(base)
		if slices.Contains(got, "FORCE_COLOR=1") {
			t.Errorf("must not override caller's FORCE_COLOR, got %v", got)
		}
	})
}

func TestParseNxHeader(t *testing.T) {
	tests := []struct {
		line  string
		isHdr bool
		label string
	}{
		{"> nx run web:build", true, "web:build"},
		{"> nx run shared:test", true, "shared:test"},
		{"regular output", false, ""},
		{"> NX some other message", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			label, isHdr := parseNxHeader(tt.line)
			if isHdr != tt.isHdr {
				t.Errorf("isHeader = %v, want %v", isHdr, tt.isHdr)
			}
			if label != tt.label {
				t.Errorf("label = %q, want %q", label, tt.label)
			}
		})
	}
}
