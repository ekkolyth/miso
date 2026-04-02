package tui

import (
	"testing"
)

func TestWrapLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  []string
	}{
		{
			name:  "short line fits",
			line:  "hello",
			width: 80,
			want:  []string{"hello"},
		},
		{
			name:  "empty line",
			line:  "",
			width: 80,
			want:  []string{""},
		},
		{
			name:  "exact fit",
			line:  "hello",
			width: 5,
			want:  []string{"hello"},
		},
		{
			name:  "wraps at word boundary (no word-awareness, just rune boundary)",
			line:  "hello world",
			width: 7,
			want:  []string{"hello w", "orld"},
		},
		{
			name:  "wraps into multiple rows",
			line:  "abcdefghij",
			width: 3,
			want:  []string{"abc", "def", "ghi", "j"},
		},
		{
			name:  "width zero returns line unchanged",
			line:  "hello",
			width: 0,
			want:  []string{"hello"},
		},
		{
			name:  "wide runes (CJK, each 2 cols)",
			line:  "你好世界", // 4 chars × 2 cols = 8 cols total
			width: 4,
			want:  []string{"你好", "世界"}, // 2 chars × 2 cols = 4 each
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLine(tt.line, tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("wrapLine(%q, %d) = %v (len %d), want %v (len %d)", tt.line, tt.width, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("wrapLine(%q, %d)[%d] = %q, want %q", tt.line, tt.width, i, got[i], tt.want[i])
				}
			}
		})
	}
}
