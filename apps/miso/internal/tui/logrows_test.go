package tui

import (
	"strings"
	"testing"
)

// #26: an underfilled log panel keeps content at the TOP and grows downward
// (like a terminal filling before it tails); the empty bottom-padding rows carry
// seq -1 so clicks there are no-ops.
func TestTabbedBuildLogVisualRowsTopAlignsWhileSparse(t *testing.T) {
	rb := NewRingBuffer(100)
	rb.Write("alpha")
	rb.Write("bravo")
	pm := &ProcessManager{Processes: []*Process{{Buffer: rb, Entry: TuiScriptEntry{Label: "x"}}}}
	m := TabbedModel{pm: pm, selected: 0, width: 80, height: 10}

	logHeight := 5
	rows, seqs := m.buildLogVisualRows(40, logHeight)

	if len(rows) != logHeight || len(seqs) != logHeight {
		t.Fatalf("rows/seqs len = %d/%d, want %d", len(rows), len(seqs), logHeight)
	}
	base := rb.BaseSeq()
	if rows[0] != "alpha" || rows[1] != "bravo" {
		t.Errorf("top rows = %q,%q, want alpha/bravo", rows[0], rows[1])
	}
	if seqs[0] != base || seqs[1] != base+1 {
		t.Errorf("content seqs = %d,%d, want %d,%d", seqs[0], seqs[1], base, base+1)
	}
	for i := 2; i < logHeight; i++ {
		if rows[i] != "" || seqs[i] != -1 {
			t.Errorf("bottom-padding row %d = %q seq %d, want empty/-1", i, rows[i], seqs[i])
		}
	}
}

func TestMergedBuildLogVisualRowsTopAlignsWhileSparse(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{{Entry: TuiScriptEntry{Label: "web"}}}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true}, width: 80, height: 10}
	m.logSeq++
	m.logLines = append(m.logLines, mergedLine{label: "web", text: "one", seq: m.logSeq})
	m.logSeq++
	m.logLines = append(m.logLines, mergedLine{label: "web", text: "two", seq: m.logSeq})

	logHeight := 5
	rows, seqs := m.buildLogVisualRows(logHeight, SelectionState{})

	if len(rows) != logHeight || len(seqs) != logHeight {
		t.Fatalf("rows/seqs len = %d/%d, want %d", len(rows), len(seqs), logHeight)
	}
	if !strings.Contains(rows[0], "one") || !strings.Contains(rows[1], "two") {
		t.Errorf("top rows = %q,%q, want one/two", rows[0], rows[1])
	}
	if seqs[0] != 1 || seqs[1] != 2 {
		t.Errorf("content seqs = %d,%d, want 1,2", seqs[0], seqs[1])
	}
	for i := 2; i < logHeight; i++ {
		if rows[i] != "" || seqs[i] != -1 {
			t.Errorf("bottom-padding row %d = %q seq %d, want empty/-1", i, rows[i], seqs[i])
		}
	}
}

// #26: merged mouse-to-row offset must derive from the rendered header height,
// so a tab row that wraps on narrow widths self-corrects instead of desyncing.
func TestMergedHeaderHeightWrapsTabRow(t *testing.T) {
	procs := []*Process{
		{Entry: TuiScriptEntry{Label: "web"}},
		{Entry: TuiScriptEntry{Label: "api"}},
		{Entry: TuiScriptEntry{Label: "worker"}},
		{Entry: TuiScriptEntry{Label: "db"}},
	}
	pm := &ProcessManager{Processes: procs}
	vis := map[int]bool{}
	for i := range procs {
		vis[i] = true
	}
	m := MergedModel{pm: pm, visible: vis, width: 20, height: 30}

	hh := m.headerHeight()
	if hh <= 3 {
		t.Fatalf("headerHeight() = %d, want > 3 (tab row should wrap at width 20)", hh)
	}
	if got, want := m.logHeight(), m.height-hh; got != want {
		t.Errorf("logHeight() = %d, want %d", got, want)
	}
	if got := m.mouseToLogRow(5, hh); got != 0 {
		t.Errorf("mouseToLogRow(y=headerHeight) = %d, want 0", got)
	}
	if got := m.mouseToLogRow(5, hh-1); got != -1 {
		t.Errorf("mouseToLogRow(y just above logs) = %d, want -1", got)
	}
}
