package tui

import tea "charm.land/bubbletea/v2"

// receives per-process output and state; one implementation renders chrome,
// another streams plain stdout
type OutputSink interface {
	OnOutput(label, line string)
	OnState(label string, state ProcessState, code int)
}

// forwards to a bubbletea program as ProcessOutputMsg / ProcessStateMsg
type programSink struct {
	prog *tea.Program
}

func (s programSink) OnOutput(label, line string) {
	s.prog.Send(ProcessOutputMsg{Label: label, Line: line})
}

func (s programSink) OnState(label string, state ProcessState, code int) {
	s.prog.Send(ProcessStateMsg{Label: label, State: state, Code: code})
}
