package tui

import (
	"bytes"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestKeyToBytes(t *testing.T) {
	tests := []struct {
		name string
		key  tea.Key
		want []byte
	}{
		{"printable", tea.Key{Text: "a"}, []byte("a")},
		{"shifted", tea.Key{Text: "A"}, []byte("A")},
		{"digit", tea.Key{Text: "1"}, []byte("1")},
		{"enter", tea.Key{Code: tea.KeyEnter}, []byte{'\r'}},
		{"tab", tea.Key{Code: tea.KeyTab}, []byte{'\t'}},
		{"backspace", tea.Key{Code: tea.KeyBackspace}, []byte{0x7f}},
		{"escape", tea.Key{Code: tea.KeyEscape}, []byte{0x1b}},
		{"up", tea.Key{Code: tea.KeyUp}, []byte("\x1b[A")},
		{"down", tea.Key{Code: tea.KeyDown}, []byte("\x1b[B")},
		{"right", tea.Key{Code: tea.KeyRight}, []byte("\x1b[C")},
		{"left", tea.Key{Code: tea.KeyLeft}, []byte("\x1b[D")},
		{"ctrl+c", tea.Key{Code: 'c', Mod: tea.ModCtrl}, []byte{0x03}},
		{"ctrl+r", tea.Key{Code: 'r', Mod: tea.ModCtrl}, []byte{0x12}},
		{"unknown", tea.Key{Code: tea.KeyF5}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyToBytes(tt.key)
			if string(got) != string(tt.want) {
				t.Errorf("keyToBytes(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestTabbedInteractiveRouting(t *testing.T) {
	var sink bytes.Buffer
	proc := &Process{stdin: &sink, State: StateRunning, Buffer: NewRingBuffer(10)}
	pm := &ProcessManager{Processes: []*Process{proc}}
	m := TabbedModel{pm: pm, keys: DefaultTabbedKeyMap()}

	// 'i' enters interactive mode
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	m = next.(TabbedModel)
	if !m.interactive {
		t.Fatal("expected interactive mode after 'i'")
	}

	// 'r' forwards to the child instead of restarting
	next, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	m = next.(TabbedModel)
	if sink.String() != "r" {
		t.Errorf("expected 'r' forwarded to stdin, got %q", sink.String())
	}

	// ctrl+z exits interactive mode
	next, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}))
	m = next.(TabbedModel)
	if m.interactive {
		t.Error("expected interactive mode off after ctrl+z")
	}
}

func TestMergedInteractiveRouting(t *testing.T) {
	var sink bytes.Buffer
	proc := &Process{stdin: &sink, State: StateRunning, Buffer: NewRingBuffer(10)}
	pm := &ProcessManager{Processes: []*Process{proc}}
	m := MergedModel{pm: pm, keys: DefaultMergedKeyMap(), visible: map[int]bool{0: true}}

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	m = next.(MergedModel)
	if !m.interactive {
		t.Fatal("expected interactive mode after 'i'")
	}

	next, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(MergedModel)
	if sink.String() != "\r" {
		t.Errorf("expected enter forwarded as CR, got %q", sink.String())
	}

	next, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}))
	m = next.(MergedModel)
	if m.interactive {
		t.Error("expected interactive mode off after ctrl+z")
	}
}
