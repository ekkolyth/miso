package tui

import "testing"

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
