package tui

import "testing"

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
