package tui

import "testing"

func TestParseTurboLine(t *testing.T) {
	tests := []struct {
		line  string
		label string
		text  string
	}{
		{"web:build: compiling...", "web:build", "compiling..."},
		{"shared:build: done in 1.2s", "shared:build", "done in 1.2s"},
		{"no prefix here", "", "no prefix here"},
		{" cache hit", "", " cache hit"},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			label, text := parseTurboLine(tt.line)
			if label != tt.label {
				t.Errorf("label = %q, want %q", label, tt.label)
			}
			if text != tt.text {
				t.Errorf("text = %q, want %q", text, tt.text)
			}
		})
	}
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
