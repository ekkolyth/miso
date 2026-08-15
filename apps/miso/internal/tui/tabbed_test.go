package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// a line op just refreshes tabbed — it re-reads p.Buffer, which the op already
// edited.
func TestTabbedLineMsgRefreshesFromBuffer(t *testing.T) {
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "web"}, "", nil, "", nil)
	p.Buffer.Write("hello world")
	m := TabbedModel{pm: pm, selected: 0, width: 40, height: 8}

	next, _ := m.Update(ProcessLineMsg{Label: "web", Op: OpAppend{Text: "hello world"}})
	m = next.(TabbedModel)

	rows, _ := m.buildLogVisualRows(m.width-1, m.logHeight())
	found := false
	for _, row := range rows {
		if strings.Contains(row, "hello world") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tabbed did not render buffer content after line msg: %#v", rows)
	}
}

func TestReassertBg(t *testing.T) {
	// the colored span's reset must not drop the selection bg for the text after it
	got := reassertBg("a\x1b[0mc", "<BG>")
	want := "<BG>a\x1b[0m<BG>c\x1b[0m"
	if got != want {
		t.Errorf("reassertBg = %q, want %q", got, want)
	}
}

func TestBuildLogVisualRowsFreezesDuringSelection(t *testing.T) {
	pm := NewProcessManager()
	proc := pm.Add(TuiScriptEntry{Label: "api:dev"}, "", nil, "", nil)
	for i := 0; i < 20; i++ {
		proc.Buffer.Write(fmt.Sprintf("line %d", i))
	}
	m := TabbedModel{pm: pm, selected: 0, width: 40, height: 8}

	bottom := m.currentBottomSeq()
	m.sel = SelectionState{active: true, dragging: true, startSeq: bottom, endSeq: bottom, anchorBottomSeq: bottom}

	lastReal := func(seqs []int64) int64 {
		for i := len(seqs) - 1; i >= 0; i-- {
			if seqs[i] >= 0 {
				return seqs[i]
			}
		}
		return -1
	}

	_, before := m.buildLogVisualRows(m.width-1, m.logHeight())
	for i := 20; i < 40; i++ { // stream in more output mid-selection
		proc.Buffer.Write(fmt.Sprintf("line %d", i))
	}
	_, after := m.buildLogVisualRows(m.width-1, m.logHeight())

	if lastReal(before) != lastReal(after) {
		t.Errorf("window moved during selection: bottom seq %d -> %d (should be frozen)", lastReal(before), lastReal(after))
	}
	if lastReal(after) != bottom {
		t.Errorf("frozen bottom seq = %d, want anchored %d", lastReal(after), bottom)
	}
}

func TestBuildLogVisualRowsScrollsAfterSelectionRelease(t *testing.T) {
	pm := NewProcessManager()
	proc := pm.Add(TuiScriptEntry{Label: "api:dev"}, "", nil, "", nil)
	for i := 0; i < 20; i++ {
		proc.Buffer.Write(fmt.Sprintf("line %d", i))
	}
	m := TabbedModel{pm: pm, selected: 0, width: 40, height: 8, scrollOffset: 3}
	bottom := m.currentBottomSeq()
	m.sel = SelectionState{active: true, startSeq: bottom, endSeq: bottom, anchorBottomSeq: bottom + 3}

	_, seqs := m.buildLogVisualRows(m.width-1, m.logHeight())
	wantBottom := proc.Buffer.BaseSeq() + int64(proc.Buffer.Len()-1-m.scrollOffset)
	if got := lastRealSeq(seqs); got != wantBottom {
		t.Errorf("completed selection froze scrolling at %d, want %d", got, wantBottom)
	}
}

func lastRealSeq(seqs []int64) int64 {
	for i := len(seqs) - 1; i >= 0; i-- {
		if seqs[i] >= 0 {
			return seqs[i]
		}
	}
	return -1
}

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
	rb.Write("\x1b[37mline one\x1b[0m")
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
	rb.Write("\x1b[36malpha\x1b[0m")
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
