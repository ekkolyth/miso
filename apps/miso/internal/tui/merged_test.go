package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMergedWindowResizeForwards(t *testing.T) {
	var gotRows, gotCols int
	p := &Process{resize: func(rows, cols int) {
		gotRows, gotCols = rows, cols
	}}
	pm := &ProcessManager{Processes: []*Process{p}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true}}

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if gotRows != 30 || gotCols != 100 {
		t.Errorf("resize forwarded as (rows=%d, cols=%d), want (rows=30, cols=100)", gotRows, gotCols)
	}
}

func TestMergedSelectedTextBySeq(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{{Entry: TuiScriptEntry{Label: "web"}}}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true}, width: 80, height: 10}
	m.logSeq++
	m.logLines = append(m.logLines, mergedLine{label: "web", text: "one", seq: m.logSeq})
	m.logSeq++
	m.logLines = append(m.logLines, mergedLine{label: "web", text: "two", seq: m.logSeq})
	m.sel = SelectionState{active: true, startSeq: 1, endSeq: 2}

	if got := m.selectedText(); got != "one\ntwo" {
		t.Errorf("selectedText() = %q, want %q", got, "one\ntwo")
	}
}
