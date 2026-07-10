package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTabbedWindowResizeForwards(t *testing.T) {
	var gotRows, gotCols int
	p := &Process{resize: func(rows, cols int) {
		gotRows, gotCols = rows, cols
	}}
	pm := &ProcessManager{Processes: []*Process{p}}
	m := TabbedModel{pm: pm}

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if gotRows != 30 || gotCols != 100 {
		t.Errorf("resize forwarded as (rows=%d, cols=%d), want (rows=30, cols=100)", gotRows, gotCols)
	}
}

func TestSidebarStartIdx(t *testing.T) {
	tests := []struct {
		selected   int
		listHeight int
		want       int
	}{
		{selected: 0, listHeight: 10, want: 0},
		{selected: 5, listHeight: 10, want: 0},  // fits, no scroll
		{selected: 10, listHeight: 10, want: 1}, // selected == listHeight, scroll by 1
		{selected: 15, listHeight: 10, want: 6}, // selected - listHeight + 1
	}
	for _, tt := range tests {
		m := TabbedModel{selected: tt.selected}
		got := m.sidebarStartIdx(tt.listHeight)
		if got != tt.want {
			t.Errorf("sidebarStartIdx(selected=%d, listHeight=%d) = %d, want %d",
				tt.selected, tt.listHeight, got, tt.want)
		}
	}
}

func TestMouseToTabIdx(t *testing.T) {
	tests := []struct {
		name       string
		x, y       int
		sidebarW   int
		listHeight int
		startIdx   int
		numProcs   int
		want       int // -1 = no-op
	}{
		{"click on header row", 2, 0, 20, 10, 0, 3, -1},
		{"click on divider row", 2, 1, 20, 10, 0, 3, -1},
		{"click first tab", 2, 2, 20, 10, 0, 3, 0},
		{"click second tab", 2, 3, 20, 10, 0, 3, 1},
		{"click past last tab", 2, 9, 20, 10, 0, 3, -1},
		{"click in log panel", 25, 3, 20, 10, 0, 3, -1},
		{"scrolled sidebar", 2, 2, 20, 10, 5, 8, 5},
		{"click below visible list", 2, 7, 20, 5, 0, 20, -1},
		// y=7 → tabRow=5, listHeight=5, so row isn't rendered — should be -1
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := TabbedModel{selected: 0}
			got := m.mouseToTabIdx(tt.x, tt.y, tt.sidebarW, tt.listHeight, tt.startIdx, tt.numProcs)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCopyAllText(t *testing.T) {
	rb := NewRingBuffer(100)
	rb.Write("line one")
	rb.Write("line two")
	rb.Write("line three")

	pm := &ProcessManager{
		Processes: []*Process{
			{Buffer: rb},
		},
	}
	m := TabbedModel{pm: pm, selected: 0}

	got := m.copyAllText()
	want := "line one\nline two\nline three"
	if got != want {
		t.Errorf("copyAllText() = %q, want %q", got, want)
	}

	// out-of-range selected
	m.selected = 5
	if m.copyAllText() != "" {
		t.Error("copyAllText() with out-of-range selected should return empty string")
	}
}

func TestTabbedSelectedTextBySeq(t *testing.T) {
	rb := NewRingBuffer(100)
	rb.Write("alpha")
	rb.Write("bravo")
	rb.Write("charlie")
	pm := &ProcessManager{Processes: []*Process{{Buffer: rb, Entry: TuiScriptEntry{Label: "x"}}}}
	m := TabbedModel{pm: pm, selected: 0, width: 80, height: 10}
	m.sel = SelectionState{active: true, startSeq: 0, endSeq: 1}

	want := "alpha\nbravo"
	if got := m.selectedText(); got != want {
		t.Errorf("selectedText() = %q, want %q", got, want)
	}

	// new output must not shift the selection off its logical lines
	rb.Write("delta")
	if got := m.selectedText(); got != want {
		t.Errorf("selection drifted after append: %q, want %q", got, want)
	}

	// #26 regression: a resize re-wraps the panel but copy reads raw buffer
	// lines by sequence, so the selection must not drift.
	next, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	m = next.(TabbedModel)
	if got := m.selectedText(); got != want {
		t.Errorf("selection drifted after resize: %q, want %q", got, want)
	}
}

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
		{
			name:  "ansi color re-emitted on every wrapped row",
			line:  "\x1b[31mAAAAAAAAAA\x1b[0m", // red, 10 cols
			width: 4,
			want: []string{
				"\x1b[31mAAAA\x1b[0m",
				"\x1b[31mAAAA\x1b[0m",
				"\x1b[31mAA\x1b[0m",
			},
		},
		{
			name:  "carry stops after mid-line reset",
			line:  "\x1b[31mAAAA\x1b[0mBBBBBB",
			width: 4,
			want: []string{
				"\x1b[31mAAAA\x1b[0m",
				"BBBB",
				"BB",
			},
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
