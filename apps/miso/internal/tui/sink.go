package tui

import tea "charm.land/bubbletea/v2"

// receives per-process output ops and state; one implementation renders chrome,
// another streams plain stdout
type OutputSink interface {
	OnLine(label string, op LineOp)
	OnState(label string, state ProcessState, code int)
}

// forwards to a bubbletea program as ProcessLineMsg / ProcessStateMsg
type programSink struct {
	prog *tea.Program
}

func (s programSink) OnLine(label string, op LineOp) {
	s.prog.Send(ProcessLineMsg{Label: label, Op: op})
}

func (s programSink) OnState(label string, state ProcessState, code int) {
	s.prog.Send(ProcessStateMsg{Label: label, State: state, Code: code})
}
