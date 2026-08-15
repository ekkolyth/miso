package tui

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLiveWriterFeedEmitsOps(t *testing.T) {
	tests := []struct {
		name    string
		seed    []string // segments fed first to establish buffer state
		segment string
		want    []LineOp
	}{
		{
			name:    "plain append",
			segment: "hello",
			want:    []LineOp{OpAppend{Text: "hello"}},
		},
		{
			name:    "carriage return collapses to one append",
			segment: "Progress: 0%\rProgress: 100%",
			want:    []LineOp{OpAppend{Text: "Progress: 100%"}},
		},
		{
			name:    "erase and home collapses spinner frames",
			segment: "⠋ Installing CocoaPods...\x1b[2K\x1b[1G⠙ Installing CocoaPods...\x1b[2K\x1b[1G✓ Installed CocoaPods",
			want:    []LineOp{OpAppend{Text: "✓ Installed CocoaPods"}},
		},
		{
			name:    "bare screen clear",
			segment: "\x1b[2J",
			want:    []LineOp{OpClear{}},
		},
		{
			name:    "clear then reprint on one segment",
			segment: "\x1b[2J\x1b[Hfresh banner",
			want:    []LineOp{OpClear{}, OpAppend{Text: "fresh banner"}},
		},
		{
			name:    "cursor-up reprint rewrites from end",
			seed:    []string{"web  Creating", "db  Creating"},
			segment: "\x1b[2Aweb  Created",
			want:    []LineOp{OpRewrite{OffsetFromEnd: 1, Text: "web  Created"}},
		},
		{
			name:    "bare cursor-up emits no op",
			seed:    []string{"web  Creating", "db  Creating"},
			segment: "\x1b[2A",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := liveWriter{buf: NewRingBuffer(DefaultBufferSize)}
			for _, seed := range tt.seed {
				lw.feed(seed)
			}
			got := lw.feed(tt.segment)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("feed(%q) = %#v, want %#v", tt.segment, got, tt.want)
			}
		})
	}
}

// a multi-line redraw frame walks the rewrite offset down to 0.
func TestLiveWriterFeedFrameOffsetsDescend(t *testing.T) {
	lw := liveWriter{buf: NewRingBuffer(DefaultBufferSize)}
	lw.feed("web  Creating")
	lw.feed("db  Creating")

	var got []LineOp
	got = append(got, lw.feed("\x1b[2Aweb  Created")...)
	got = append(got, lw.feed("db  Created")...)

	want := []LineOp{
		OpRewrite{OffsetFromEnd: 1, Text: "web  Created"},
		OpRewrite{OffsetFromEnd: 0, Text: "db  Created"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frame ops = %#v, want %#v", got, want)
	}
}

func TestReadLinesTruncatesButKeepsDraining(t *testing.T) {
	// A single line far longer than the cap must not stall the reader: it is
	// truncated in what's emitted but fully consumed, so the lines after it
	// still arrive. This is the electron/vite bundle-line hang (#33).
	big := strings.Repeat("x", 50)           // > maxLine below
	input := big + "\n" + "after\n" + "tail" // "tail" = trailing partial, no newline

	var got []string
	readLines(strings.NewReader(input), 10, func(s string) { got = append(got, s) })

	if len(got) != 3 {
		t.Fatalf("emitted %d lines, want 3: %#v", len(got), got)
	}
	if len(got[0]) != 10 {
		t.Errorf("oversized line should be truncated to cap 10, got len %d", len(got[0]))
	}
	if got[1] != "after" {
		t.Errorf("line after the huge line = %q, want %q (reader kept draining)", got[1], "after")
	}
	if got[2] != "tail" {
		t.Errorf("trailing partial line = %q, want %q", got[2], "tail")
	}
}

func TestReadLinesReassemblesAndCountsExactly(t *testing.T) {
	var got []string
	readLines(strings.NewReader("a\nbb\nccc\n"), 1024, func(s string) { got = append(got, s) })
	want := []string{"a", "bb", "ccc"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestProcessManagerPinLast(t *testing.T) {
	pm := NewProcessManager()
	pm.Add(TuiScriptEntry{Label: "turbo"}, "", nil, "", nil) // meta tab created first
	pm.Add(TuiScriptEntry{Label: "api:dev"}, "", nil, "", nil)
	pm.PinLast("turbo")
	pm.Add(TuiScriptEntry{Label: "web:dev"}, "", nil, "", nil)
	pm.PinLast("turbo")

	var order []string
	for _, p := range pm.Processes {
		order = append(order, p.Entry.Label)
	}
	if got := strings.Join(order, ","); got != "api:dev,web:dev,turbo" {
		t.Errorf("tab order = %q, want api:dev,web:dev,turbo", got)
	}

	pm.PinLast("absent") // no-op
	if pm.Processes[len(pm.Processes)-1].Entry.Label != "turbo" {
		t.Error("PinLast(absent) must not change order")
	}
}

func TestProcessManager_SpawnAndCapture(t *testing.T) {
	pm := NewProcessManager()

	entry := TuiScriptEntry{
		Label:      "test-echo",
		ScriptName: "echo",
	}

	p := pm.Add(entry, "sh", []string{"-c", `printf 'hello world\n'`}, "", nil)
	if p == nil {
		t.Fatal("expected non-nil process")
	}

	if err := pm.Start(p); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// PTY flush timing varies by runner; poll for the child's output rather
	// than sleeping a fixed interval.
	var lines []string
	found := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		lines = p.Buffer.Lines()
		for _, line := range lines {
			if strings.Contains(line, "hello world") {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if len(lines) == 0 {
		t.Fatal("expected at least one line in buffer, got none")
	}
	if !found {
		t.Errorf("expected 'hello world' in buffer, got: %v", lines)
	}
}

// TestStartNoPTYSpawnsPipes proves the pty-vs-pipe split: a NoPTY child sees
// stdout is NOT a tty (`[ -t 1 ]` exits 1), the default pty child sees a tty
// (exit 0). This is what makes docker/vite emit plain output under plain mode.
func TestStartNoPTYSpawnsPipes(t *testing.T) {
	run := func(noPTY bool) string {
		pm := NewProcessManager()
		p := pm.Add(TuiScriptEntry{Label: "tty"}, "sh",
			[]string{"-c", `[ -t 1 ]; echo notty=$?`}, "", nil)
		p.NoPTY = noPTY
		if err := pm.Start(p); err != nil {
			t.Fatalf("Start: %v", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			for _, line := range p.Buffer.Lines() {
				if strings.HasPrefix(line, "notty=") {
					return line
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("no notty= line captured")
		return ""
	}

	if got := run(true); got != "notty=1" {
		t.Errorf("pipe (NoPTY) child = %q, want notty=1 (stdout is not a tty)", got)
	}
	if got := run(false); got != "notty=0" {
		t.Errorf("pty child = %q, want notty=0 (stdout is a tty)", got)
	}
}

// TestStartNoPTYSuppressesCursorRedraw drives a child that only emits a cursor-up
// redraw under a tty; on pipes it takes the plain branch, so the buffer holds one
// clean line with no escape artifacts — no flood to collapse.
func TestStartNoPTYSuppressesCursorRedraw(t *testing.T) {
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "redraw"}, "sh",
		[]string{"-c", `if [ -t 1 ]; then printf 'X\033[1Aredraw\n'; else printf 'plain\n'; fi`},
		"", nil)
	p.NoPTY = true
	if err := pm.Start(p); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var lines []string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		lines = p.Buffer.Lines()
		if len(lines) == 1 && lines[0] == "plain" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(lines) != 1 || lines[0] != "plain" {
		t.Fatalf("buffer = %#v, want [plain] (no tty ⇒ no cursor redraw)", lines)
	}
	if strings.ContainsRune(lines[0], '\x1b') {
		t.Errorf("plain line retained an escape sequence: %q", lines[0])
	}
}

func TestProcessManager_State(t *testing.T) {
	pm := NewProcessManager()

	entry := TuiScriptEntry{
		Label:      "test-state",
		ScriptName: "echo",
	}

	p := pm.Add(entry, "echo", []string{"state test"}, "", nil)

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

	p := pm.Add(entry, "sleep", []string{"60"}, "", nil)

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

// pausingSink blocks the first StateExited notification until released, so a
// test can force a Restart to land inside the exit goroutine's window
// between sendState and close(done).
type pausingSink struct {
	once    sync.Once
	reached chan struct{}
	proceed chan struct{}
}

func (s *pausingSink) OnLine(_ string, _ LineOp) {}

func (s *pausingSink) OnState(_ string, state ProcessState, _ int) {
	if state != StateExited {
		return
	}
	s.once.Do(func() {
		close(s.reached)
		<-s.proceed
	})
}

func TestProcessManager_RestartDuringExitDoesNotDoubleCloseDone(t *testing.T) {
	pm := NewProcessManager()
	entry := TuiScriptEntry{Label: "restart-race", ScriptName: "echo"}
	p := pm.Add(entry, "echo", []string{"hi"}, "", nil)

	sink := &pausingSink{reached: make(chan struct{}), proceed: make(chan struct{})}
	pm.SetSink(sink)

	if err := pm.Start(p); err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case <-sink.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("exit goroutine never reached sendState(StateExited)")
	}

	p.mu.Lock()
	oldDone := p.done
	p.mu.Unlock()

	// Stop() observes StateExited (already set before sendState blocked
	// above) and returns immediately; Start() then swaps in a new done
	// channel while the original exit goroutine is still paused.
	if err := pm.Restart(p); err != nil {
		t.Fatalf("Restart: %v", err)
	}

	p.mu.Lock()
	newDone := p.done
	p.mu.Unlock()

	if oldDone == newDone {
		t.Fatal("Restart should have replaced p.done with a new channel")
	}

	close(sink.proceed) // release the original exit goroutine

	select {
	case <-oldDone:
	case <-time.After(5 * time.Second):
		t.Fatal("original done channel was never closed")
	}

	select {
	case <-newDone:
	case <-time.After(5 * time.Second):
		t.Fatal("restarted process's done channel was never closed")
	}
}

func TestProcessWriteStdin(t *testing.T) {
	var sink strings.Builder
	p := &Process{Buffer: NewRingBuffer(10)}

	// nil stdin: no-op, no error
	if err := p.WriteStdin([]byte("x")); err != nil {
		t.Fatalf("WriteStdin with nil stdin should be a no-op, got %v", err)
	}

	p.stdin = &sink
	if err := p.WriteStdin([]byte("hello")); err != nil {
		t.Fatalf("WriteStdin: %v", err)
	}
	if sink.String() != "hello" {
		t.Errorf("stdin got %q, want %q", sink.String(), "hello")
	}
}

func TestCaptureOutputTrimsCarriageReturn(t *testing.T) {
	// The child emits bare "\n"; the pty's ONLCR maps it to "\r\n" on the
	// master, so captured lines carry a trailing "\r" that must be trimmed.
	pm := NewProcessManager()
	// Linger after writing so the reader drains before the slave closes — see
	// TestProcessManager_SpawnAndCapture for the macOS pty race (#38).
	p := pm.Add(TuiScriptEntry{Label: "cr"}, "sh", []string{"-c", `printf 'one\ntwo\n'; sleep 0.2`}, "", nil)
	if err := pm.Start(p); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	lines := p.Buffer.Lines()
	if len(lines) == 0 {
		t.Fatal("expected captured lines, got none")
	}
	for _, line := range lines {
		if strings.HasSuffix(line, "\r") {
			t.Errorf("line retained trailing CR: %q", line)
		}
	}
}

func TestCaptureOutputInPlaceRedrawOverwrites(t *testing.T) {
	// docker compose redraws its container block by moving the cursor up N
	// lines and reprinting. The buffer must end holding the final N lines, not
	// the accumulated flood of every frame.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "compose"}, "", nil, "", nil)

	stream := "web  Creating\ndb  Creating\n" + // frame 1
		"\x1b[2Aweb  Created\ndb  Created\n" // up 2, reprint (frame 2)

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	want := []string{"web  Created", "db  Created"}
	if len(got) != len(want) {
		t.Fatalf("buffer has %d lines, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCaptureOutputInPlaceRedrawGrows(t *testing.T) {
	// a redraw frame taller than the previous one overwrites what it can and
	// appends the rest.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "compose"}, "", nil, "", nil)

	stream := "web  Creating\ndb  Creating\n" +
		"\x1b[2Aweb  Started\ndb  Started\ncache  Started\n"

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	want := []string{"web  Started", "db  Started", "cache  Started"}
	if len(got) != len(want) {
		t.Fatalf("buffer has %d lines, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCaptureOutputBareCursorUpDoesNotBlankLines(t *testing.T) {
	// a cursor-up segment with no reprint text on it (the child issued the
	// escape on its own line) must not blank the line it points at — it only
	// repositions the cursor for whatever segment reprints next.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "spin"}, "", nil, "", nil)

	stream := "web  Creating\ndb  Creating\n" + // frame 1
		"\x1b[2A\n" + // bare cursor-up, no reprint text
		"web  Created\ndb  Created\n" // reprint arrives on its own lines

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	want := []string{"web  Created", "db  Created"}
	if len(got) != len(want) {
		t.Fatalf("buffer has %d lines, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCaptureOutputRedrawWithBlankRowStillOverwrites(t *testing.T) {
	// a real multi-line reprint can legitimately include a blank row — that
	// must still overwrite its slot. Only a segment that is nothing but a
	// cursor reposition (no reprint text of its own) is skipped.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "compose"}, "", nil, "", nil)

	stream := "web  Creating\ndb  Creating\nlog line\n" + // frame 1: 3 lines
		"\x1b[3Aweb  Created\n\ndb  Created\n" // up 3, middle row reprints blank

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	want := []string{"web  Created", "", "db  Created"}
	if len(got) != len(want) {
		t.Fatalf("buffer has %d lines, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCaptureOutputCarriageReturnProgressCollapses(t *testing.T) {
	// a \r progress bar rewrites one line in place; the buffer holds a single
	// line at the final percentage, not one per tick.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "pull"}, "", nil, "", nil)

	stream := "Progress: 0%\rProgress: 50%\rProgress: 100%\n"

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	if len(got) != 1 {
		t.Fatalf("buffer has %d lines, want 1: %#v", len(got), got)
	}
	if got[0] != "Progress: 100%" {
		t.Errorf("line = %q, want %q", got[0], "Progress: 100%")
	}
}

func TestCaptureOutputInPlacePreservesColor(t *testing.T) {
	// SGR color survives an in-place overwrite unchanged.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "compose"}, "", nil, "", nil)

	stream := "\x1b[33mweb  Creating\x1b[0m\n" +
		"\x1b[1Aweb  \x1b[32mStarted\x1b[0m\n"

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	if len(got) != 1 {
		t.Fatalf("buffer has %d lines, want 1: %#v", len(got), got)
	}
	want := "web  \x1b[32mStarted\x1b[0m"
	if got[0] != want {
		t.Errorf("line = %q, want %q", got[0], want)
	}
}

func TestCaptureOutputScreenClearResetsPane(t *testing.T) {
	// vite clears the terminal before printing its banner on each reload. The
	// stripped clear used to let pre-clear output accumulate, overflowing the
	// panel so it tailed and the banner sank to the bottom. A clear must wipe the
	// pane so only post-clear content remains.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "web"}, "", nil, "", nil)

	stream := "line1\nline2\n\x1b[2J\x1b[Hfresh banner\n"

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	want := []string{"fresh banner"}
	if len(got) != len(want) {
		t.Fatalf("buffer has %d lines, want %d: %#v", len(got), len(want), got)
	}
	if got[0] != want[0] {
		t.Errorf("line = %q, want %q", got[0], want[0])
	}
}

func TestCaptureOutputHomeEraseResetsPane(t *testing.T) {
	// home + erase-below (ESC[H ESC[0J) is the other full-screen clear idiom.
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "web"}, "", nil, "", nil)

	stream := "stale one\nstale two\n\x1b[H\x1b[0J\x1b[32mready\x1b[0m\n"

	var wg sync.WaitGroup
	wg.Add(1)
	pm.captureOutput(p, strings.NewReader(stream), &wg)
	wg.Wait()

	got := p.Buffer.Lines()
	want := []string{"\x1b[32mready\x1b[0m"}
	if len(got) != len(want) {
		t.Fatalf("buffer has %d lines, want %d: %#v", len(got), len(want), got)
	}
	if got[0] != want[0] {
		t.Errorf("line = %q, want %q (color must survive)", got[0], want[0])
	}
}

func TestProcessManager_ResizeAll(t *testing.T) {
	type call struct{ rows, cols int }
	var calls []call

	spy := func(rows, cols int) {
		calls = append(calls, call{rows: rows, cols: cols})
	}

	pm := &ProcessManager{
		Processes: []*Process{
			{resize: spy},
			{resize: spy},
		},
	}

	pm.ResizeAll(30, 100)

	if len(calls) != 2 {
		t.Fatalf("expected 2 resize calls, got %d", len(calls))
	}
	for _, c := range calls {
		if c.rows != 30 || c.cols != 100 {
			t.Errorf("resize call = (rows=%d, cols=%d), want (rows=30, cols=100)", c.rows, c.cols)
		}
	}
}

func TestProcessResizeNilSafe(_ *testing.T) {
	p := &Process{}
	p.Resize(10, 20) // must not panic
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

// The preamble must land in the ring buffer (what the tabbed and merged
// renderers draw from) ahead of the child's own first line.
func TestStartEmitsPreambleBeforeChildOutput(t *testing.T) {
	pm := NewProcessManager()
	p := pm.Add(TuiScriptEntry{Label: "web"}, "sh",
		[]string{"-c", `printf 'next dev booting\n'`}, "", nil)
	p.Preamble = []string{"env validated — 2 scopes, 3 variables"}

	if err := pm.Start(p); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	pm.WaitAllExited([]*Process{p})

	lines := p.Buffer.Lines()
	if len(lines) < 2 {
		t.Fatalf("got %d lines, want preamble + child output: %v", len(lines), lines)
	}
	if lines[0] != "env validated — 2 scopes, 3 variables" {
		t.Errorf("first line = %q, want the preamble", lines[0])
	}
	if !strings.Contains(lines[1], "next dev booting") {
		t.Errorf("second line = %q, want the child's output", lines[1])
	}
}
