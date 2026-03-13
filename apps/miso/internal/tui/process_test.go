package tui

import (
	"strings"
	"testing"
	"time"
)

func TestProcessManager_SpawnAndCapture(t *testing.T) {
	pm := NewProcessManager()

	entry := TuiScriptEntry{
		Label:      "test-echo",
		ScriptName: "echo",
	}

	p := pm.Add(entry, "echo", []string{"hello world"}, "")
	if p == nil {
		t.Fatal("expected non-nil process")
	}

	if err := pm.Start(p); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for process to finish
	time.Sleep(500 * time.Millisecond)

	lines := p.Buffer.Lines()
	if len(lines) == 0 {
		t.Fatal("expected at least one line in buffer, got none")
	}

	found := false
	for _, line := range lines {
		if strings.Contains(line, "hello world") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'hello world' in buffer, got: %v", lines)
	}
}

func TestProcessManager_State(t *testing.T) {
	pm := NewProcessManager()

	entry := TuiScriptEntry{
		Label:      "test-state",
		ScriptName: "echo",
	}

	p := pm.Add(entry, "echo", []string{"state test"}, "")

	if p.State != StateStarting {
		t.Errorf("expected StateStarting before Start, got %v", p.State)
	}

	if err := pm.Start(p); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give the process a moment to transition to Running
	time.Sleep(50 * time.Millisecond)

	// The process may already be Exited since echo is fast; that's acceptable.
	// We just verify it progressed past Starting.
	p.mu.Lock()
	state := p.State
	p.mu.Unlock()

	if state == StateStarting {
		t.Errorf("process should have advanced from StateStarting, still in StateStarting")
	}

	// Wait for exit
	time.Sleep(500 * time.Millisecond)

	p.mu.Lock()
	finalState := p.State
	finalCode := p.ExitCode
	p.mu.Unlock()

	if finalState != StateExited {
		t.Errorf("expected StateExited, got %v", finalState)
	}
	if finalCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", finalCode)
	}
}

func TestProcessManager_StopAll(t *testing.T) {
	pm := NewProcessManager()

	entry := TuiScriptEntry{
		Label:      "test-sleep",
		ScriptName: "sleep",
	}

	p := pm.Add(entry, "sleep", []string{"60"}, "")

	if err := pm.Start(p); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give the process time to start
	time.Sleep(100 * time.Millisecond)

	p.mu.Lock()
	state := p.State
	p.mu.Unlock()

	if state != StateRunning {
		t.Errorf("expected StateRunning after start, got %v", state)
	}

	pm.StopAll()

	// Wait for processes to stop
	time.Sleep(2 * time.Second)

	p.mu.Lock()
	finalState := p.State
	p.mu.Unlock()

	if finalState != StateExited {
		t.Errorf("expected StateExited after StopAll, got %v", finalState)
	}
}

func TestStripNonColorANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "preserve color SGR",
			input: "\x1b[31mred text\x1b[0m",
			want:  "\x1b[31mred text\x1b[0m",
		},
		{
			name:  "strip cursor up",
			input: "\x1b[2Ahello",
			want:  "hello",
		},
		{
			name:  "strip cursor position",
			input: "\x1b[1;1Hhello",
			want:  "hello",
		},
		{
			name:  "strip erase display",
			input: "\x1b[2Jhello",
			want:  "hello",
		},
		{
			name:  "strip private mode set",
			input: "\x1b[?25lhello",
			want:  "hello",
		},
		{
			name:  "strip private mode reset",
			input: "\x1b[?25hhello",
			want:  "hello",
		},
		{
			name:  "preserve bold color",
			input: "\x1b[1;32mgreen bold\x1b[0m",
			want:  "\x1b[1;32mgreen bold\x1b[0m",
		},
		{
			name:  "mixed: strip cursor, preserve color",
			input: "\x1b[2A\x1b[32mhello\x1b[0m\x1b[K",
			want:  "\x1b[32mhello\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripNonColorANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripNonColorANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
