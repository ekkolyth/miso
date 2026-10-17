package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func feedMerged(m MergedModel, label string, op LineOp) MergedModel {
	next, _ := m.Update(ProcessLineMsg{Label: label, Op: op})
	return next.(MergedModel)
}

func mergedTexts(m MergedModel) []string {
	out := make([]string, len(m.logLines))
	for i, line := range m.logLines {
		out[i] = line.text
	}
	return out
}

// a redraw burst with no other app interleaving collapses in place — the list
// holds the final frame, not one copy per tick.
func TestMergedRewritesInPlaceWhenContiguous(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{{Entry: TuiScriptEntry{Label: "web"}}}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true}}

	m = feedMerged(m, "web", OpAppend{Text: "web  Creating"})
	m = feedMerged(m, "web", OpAppend{Text: "db  Creating"})
	m = feedMerged(m, "web", OpRewrite{OffsetFromEnd: 1, Text: "web  Created"})
	m = feedMerged(m, "web", OpRewrite{OffsetFromEnd: 0, Text: "db  Created"})
	m = feedMerged(m, "web", OpRewrite{OffsetFromEnd: 1, Text: "web  Started"})
	m = feedMerged(m, "web", OpRewrite{OffsetFromEnd: 0, Text: "db  Started"})

	got := mergedTexts(m)
	want := []string{"web  Started", "db  Started"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merged lines = %#v, want %#v (contiguous redraw must collapse, not flood)", got, want)
	}
}

// once another app interleaves, the prior block is frozen — a following rewrite
// starts a fresh block instead of overwriting the committed lines.
func TestMergedFreezesBlockOnInterleave(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{
		{Entry: TuiScriptEntry{Label: "a"}},
		{Entry: TuiScriptEntry{Label: "b"}},
	}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true, 1: true}}

	m = feedMerged(m, "a", OpAppend{Text: "a1"})
	m = feedMerged(m, "a", OpAppend{Text: "a2"})
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "a2'"}) // still contiguous → in place
	m = feedMerged(m, "b", OpAppend{Text: "b1"})                     // interleave freezes a's block
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "a3"})  // must append, not touch a2'

	got := mergedTexts(m)
	want := []string{"a1", "a2'", "b1", "a3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merged lines = %#v, want %#v (interleave must freeze the prior block)", got, want)
	}
}

// clear resets the live block: prior lines are frozen history, and a later
// rewrite reaching past the fresh block appends rather than corrupting them.
func TestMergedClearResetsBlock(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{{Entry: TuiScriptEntry{Label: "web"}}}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true}}

	m = feedMerged(m, "web", OpAppend{Text: "line1"})
	m = feedMerged(m, "web", OpAppend{Text: "line2"})
	m = feedMerged(m, "web", OpClear{})
	m = feedMerged(m, "web", OpAppend{Text: "fresh"})
	m = feedMerged(m, "web", OpRewrite{OffsetFromEnd: 1, Text: "next"})

	got := mergedTexts(m)
	want := []string{"line1", "line2", "fresh", "next"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merged lines = %#v, want %#v (clear must freeze pre-clear lines)", got, want)
	}
}

// toggling another app's visibility is a render-time filter — it must not
// disturb the in-place bookkeeping kept on the full list.
func TestMergedFilterToggleDoesNotCorruptRewrites(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{
		{Entry: TuiScriptEntry{Label: "a"}},
		{Entry: TuiScriptEntry{Label: "b"}},
	}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true, 1: true}}

	m = feedMerged(m, "a", OpAppend{Text: "a1"})
	m = feedMerged(m, "a", OpAppend{Text: "a2"})
	m.visible[1] = false // hide b mid-stream
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 1, Text: "a1'"})
	m.visible[1] = true // show b again
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "a2'"})

	got := mergedTexts(m)
	want := []string{"a1'", "a2'"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merged lines = %#v, want %#v (filter toggle must not disturb rewrites)", got, want)
	}
}

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
	m.logLines = append(m.logLines, mergedLine{label: "web", text: "\x1b[37mone\x1b[0m", seq: m.logSeq})
	m.logSeq++
	m.logLines = append(m.logLines, mergedLine{label: "web", text: "two", seq: m.logSeq})
	m.sel = SelectionState{active: true, startSeq: 1, endSeq: 2}

	if got := m.selectedText(); got != "one\ntwo" {
		t.Errorf("selectedText() = %q, want %q", got, "one\ntwo")
	}
	if got := m.copyAllText(); got != "one\ntwo" {
		t.Errorf("copyAllText() = %q, want %q", got, "one\ntwo")
	}
}

// a line painted before its newline and completed with no interleave stays one row
func TestMergedPartialCompletesInPlaceWhenContiguous(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{{Entry: TuiScriptEntry{Label: "a"}}}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true}}

	m = feedMerged(m, "a", OpAppend{Text: "p", Pending: true})
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "p a", Pending: true})
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "p ab"})
	m = feedMerged(m, "a", OpAppend{Text: "next"})

	got := mergedTexts(m)
	want := []string{"p ab", "next"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merged lines = %#v, want %#v", got, want)
	}
}

// once another app writes below an unfinished line, the next update shows up
// as a fresh copy at the bottom and the old one stays as history
func TestMergedPartialRedisplaysAfterInterleave(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{
		{Entry: TuiScriptEntry{Label: "a"}},
		{Entry: TuiScriptEntry{Label: "b"}},
	}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true, 1: true}}

	m = feedMerged(m, "a", OpAppend{Text: "Resolving", Pending: true})
	m = feedMerged(m, "b", OpAppend{Text: "b1"})
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "Installing", Pending: true})
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "Saved"})

	got := mergedTexts(m)
	want := []string{"Resolving", "b1", "Saved"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merged lines = %#v, want %#v", got, want)
	}
}

// one app's live progress line must not corrupt another app's redraw frame
func TestMergedPartialDoesNotCorruptOtherRedraw(t *testing.T) {
	pm := &ProcessManager{Processes: []*Process{
		{Entry: TuiScriptEntry{Label: "a"}},
		{Entry: TuiScriptEntry{Label: "b"}},
	}}
	m := MergedModel{pm: pm, visible: map[int]bool{0: true, 1: true}}

	for _, line := range []string{"b1", "b2", "b3", "b4"} {
		m = feedMerged(m, "b", OpAppend{Text: line})
	}
	m = feedMerged(m, "a", OpAppend{Text: "p", Pending: true})
	m = feedMerged(m, "b", OpRewrite{OffsetFromEnd: 3, Text: "f1"})
	m = feedMerged(m, "b", OpRewrite{OffsetFromEnd: 2, Text: "f2"})
	m = feedMerged(m, "a", OpRewrite{OffsetFromEnd: 0, Text: "pa", Pending: true})
	m = feedMerged(m, "b", OpRewrite{OffsetFromEnd: 1, Text: "f3"})
	m = feedMerged(m, "b", OpRewrite{OffsetFromEnd: 0, Text: "f4"})

	got := mergedTexts(m)
	want := []string{"b1", "b2", "b3", "b4", "p", "f1", "f2", "pa", "f3", "f4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merged lines = %#v, want %#v", got, want)
	}
}
