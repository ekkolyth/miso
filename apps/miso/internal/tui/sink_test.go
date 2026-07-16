package tui

import (
	"strings"
	"sync"
	"testing"
)

type recordingSink struct {
	mu      sync.Mutex
	outputs []ProcessOutputMsg
	states  []ProcessStateMsg
}

func (r *recordingSink) OnOutput(label, line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outputs = append(r.outputs, ProcessOutputMsg{Label: label, Line: line})
}

func (r *recordingSink) OnState(label string, state ProcessState, code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = append(r.states, ProcessStateMsg{Label: label, State: state, Code: code})
}

func TestProcessManagerRoutesOutputThroughSink(t *testing.T) {
	pm := NewProcessManager()
	rec := &recordingSink{}
	pm.SetSink(rec)

	p := pm.Add(TuiScriptEntry{Label: "web"}, "", nil, "", nil)
	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader("hello\nworld\n"), &wg)
	wg.Wait()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.outputs) != 2 {
		t.Fatalf("sink got %d lines, want 2: %v", len(rec.outputs), rec.outputs)
	}
	if rec.outputs[0].Label != "web" || rec.outputs[0].Line != "hello" {
		t.Errorf("line[0] = %+v, want {web hello}", rec.outputs[0])
	}
}
