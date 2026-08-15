package tui

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ekkolyth/miso/internal/proc"
)

type ProcessState int

const (
	StateStarting ProcessState = iota
	StateRunning
	StateExited
)

// LineOp is one edit a child made to its output — append a tail line, overwrite
// a recent one, or clear the pane — so a consumer can mirror an in-place redraw
// instead of re-appending every frame.
type LineOp interface{ lineOp() }

type OpAppend struct{ Text string }

// OffsetFromEnd 0 is the newest line, matching RingBuffer.SetFromEnd.
type OpRewrite struct {
	OffsetFromEnd int
	Text          string
}

type OpClear struct{}

func (OpAppend) lineOp()  {}
func (OpRewrite) lineOp() {}
func (OpClear) lineOp()   {}

// ProcessLineMsg carries one output op from the capture goroutine to the model.
type ProcessLineMsg struct {
	Label string
	Op    LineOp
}

type ProcessStateMsg struct {
	Label string
	State ProcessState
	Code  int
}

// streams wired to a spawned process — unix: pty master is both reader and
// stdin writer; windows: separate stdout/stderr pipes + stdin pipe
type spawnResult struct {
	readers          []io.Reader
	stdin            io.Writer
	resize           func(rows, cols int)
	releaseAfterWait func()
	closer           func()
}

type Process struct {
	Entry     TuiScriptEntry
	Command   string
	Args      []string
	Dir       string   // working directory for the process
	Environ   []string // environment variables for the process (nil = inherit)
	NoPTY     bool     // plain mode: pipe stdio (no pty) so a tool self-formats for a non-tty
	State     ProcessState
	ExitCode  int
	StartedAt time.Time
	Buffer    *RingBuffer

	cmd    *exec.Cmd
	stdin  io.Writer // writable child stdin (pty master on unix, pipe on Windows)
	resize func(rows, cols int)
	done   chan struct{}
	mu     sync.Mutex
}

// ProcessManager owns the set of managed processes and dispatches output/state to a sink.
type ProcessManager struct {
	Processes []*Process
	mu        sync.Mutex
	sink      OutputSink
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{}
}

// SetSink registers the destination for process output and state.
func (pm *ProcessManager) SetSink(sink OutputSink) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.sink = sink
}

func (pm *ProcessManager) Add(entry TuiScriptEntry, command string, args []string, dir string, environ []string) *Process {
	p := &Process{
		Entry:   entry,
		Command: command,
		Args:    args,
		Dir:     dir,
		Environ: environ,
		State:   StateStarting,
		Buffer:  NewRingBuffer(DefaultBufferSize),
		done:    make(chan struct{}),
	}

	pm.mu.Lock()
	pm.Processes = append(pm.Processes, p)
	pm.mu.Unlock()

	return p
}

// PinLast moves the process with the given label to the end of the list — used
// to keep a meta tab (e.g. "turbo") below the real workspace tabs as they stream
// in. No-op if the label isn't present or is already last.
func (pm *ProcessManager) PinLast(label string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	idx := -1
	for i, p := range pm.Processes {
		if p.Entry.Label == label {
			idx = i
			break
		}
	}
	if idx < 0 || idx == len(pm.Processes)-1 {
		return
	}
	p := pm.Processes[idx]
	pm.Processes = append(pm.Processes[:idx], pm.Processes[idx+1:]...)
	pm.Processes = append(pm.Processes, p)
}

// Start spawns the process, captures stdout and stderr in separate goroutines,
// and manages the process lifecycle.
func (pm *ProcessManager) Start(p *Process) error {
	p.mu.Lock()
	p.State = StateStarting
	p.ExitCode = 0
	p.done = make(chan struct{})
	p.cmd = exec.Command(p.Command, p.Args...)
	if p.Dir != "" {
		p.cmd.Dir = p.Dir
	}
	if p.Environ != nil {
		p.cmd.Env = p.Environ
	}
	cmd := p.cmd
	done := p.done
	noPTY := p.NoPTY
	p.mu.Unlock()

	// pipes when noPTY (plain mode — the tool sees a non-tty and self-formats to
	// line output), else a pty so chrome children stay interactive and colored.
	var res *spawnResult
	var err error
	if noPTY {
		res, err = spawnPipes(cmd)
	} else {
		res, err = spawnProcess(cmd, 0, 0)
	}
	if err != nil {
		p.mu.Lock()
		p.State = StateExited
		p.ExitCode = -1
		p.mu.Unlock()
		pm.sendState(p, StateExited, -1)
		close(done)
		return err
	}

	p.mu.Lock()
	p.stdin = res.stdin
	p.resize = res.resize
	p.State = StateRunning
	p.StartedAt = time.Now()
	p.mu.Unlock()
	pm.sendState(p, StateRunning, 0)

	var wg sync.WaitGroup
	wg.Add(len(res.readers))
	for _, r := range res.readers {
		go pm.captureOutput(p, r, &wg)
	}

	go func() {
		var exitErr error
		if res.releaseAfterWait != nil {
			exitErr = cmd.Wait()
			res.releaseAfterWait()
			wg.Wait()
		} else {
			wg.Wait()
			exitErr = cmd.Wait()
		}
		if res.closer != nil {
			res.closer()
		}

		code := 0
		if exitErr != nil {
			if exitError, ok := exitErr.(*exec.ExitError); ok {
				code = exitError.ExitCode()
			} else {
				code = -1
			}
		}

		p.mu.Lock()
		p.State = StateExited
		p.ExitCode = code
		p.mu.Unlock()

		pm.sendState(p, StateExited, code)

		close(done)
	}()

	return nil
}

// Stop sends SIGTERM to the process, waits up to 5 seconds, then SIGKILLs.
// It does NOT call cmd.Wait() — the goroutine started in Start() handles that.
func (pm *ProcessManager) Stop(p *Process) {
	p.mu.Lock()
	cmd := p.cmd
	state := p.State
	done := p.done
	p.mu.Unlock()

	if state == StateExited || cmd == nil || cmd.Process == nil {
		return
	}

	// Kill the entire process group (negative PID) so child processes are cleaned up.
	pgid := cmd.Process.Pid
	_ = proc.KillGroup(pgid, syscall.SIGTERM)

	select {
	case <-done:
		// Process group exited cleanly after SIGTERM.
	case <-time.After(5 * time.Second):
		_ = proc.KillGroup(pgid, syscall.SIGKILL)
		// Wait for the done channel to be closed by the Start() goroutine.
		<-done
	}
}

// no-op when stdin is unset; used by interactive mode
func (p *Process) WriteStdin(b []byte) error {
	p.mu.Lock()
	w := p.stdin
	p.mu.Unlock()
	if w == nil {
		return nil
	}
	_, err := w.Write(b)
	return err
}

// no-op until the process is spawned
func (p *Process) Resize(rows, cols int) {
	p.mu.Lock()
	fn := p.resize
	p.mu.Unlock()
	if fn != nil {
		fn(rows, cols)
	}
}

// Restart stops the process and starts it again.
func (pm *ProcessManager) Restart(p *Process) error {
	pm.Stop(p)
	p.Buffer.Clear()
	return pm.Start(p)
}

// RestartAll restarts every process in the manager.
func (pm *ProcessManager) RestartAll() {
	pm.mu.Lock()
	procs := make([]*Process, len(pm.Processes))
	copy(procs, pm.Processes)
	pm.mu.Unlock()

	for _, p := range procs {
		_ = pm.Restart(p)
	}
}

// ResizeAll forwards a terminal size change to every process's pty.
func (pm *ProcessManager) ResizeAll(rows, cols int) {
	pm.mu.Lock()
	procs := make([]*Process, len(pm.Processes))
	copy(procs, pm.Processes)
	pm.mu.Unlock()

	for _, p := range procs {
		p.Resize(rows, cols)
	}
}

// StopAll stops every process in the manager.
func (pm *ProcessManager) StopAll() {
	pm.mu.Lock()
	procs := make([]*Process, len(pm.Processes))
	copy(procs, pm.Processes)
	pm.mu.Unlock()

	var wg sync.WaitGroup
	for _, p := range procs {
		wg.Add(1)
		go func(process *Process) {
			defer wg.Done()
			pm.Stop(process)
		}(p)
	}
	wg.Wait()
}

// captureOutput reads lines from r, resolves in-place redraws against the
// process buffer, and emits the resulting ops to the sink.
func (pm *ProcessManager) captureOutput(p *Process, r io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()

	// Retain up to 1MB per line; anything beyond is dropped from the stored line
	// but still consumed, so a child that writes a huge unbroken line (electron/
	// vite minified bundles) can't stall the pty and freeze the TUI.
	const maxLine = 1024 * 1024
	live := liveWriter{buf: p.Buffer}
	readLines(r, maxLine, func(raw string) {
		for _, op := range live.feed(raw) {
			pm.sendLine(p, op)
		}
	})
}

// liveWriter maps a child's redraw stream onto in-place buffer edits so a tool
// that repaints its output — docker compose's container block, a spinner, a
// vite banner after a screen clear — overwrites or resets recent lines instead
// of appending a fresh copy each tick. It handles the linear "move up N lines,
// rewrite them" and "clear the screen, start over" patterns only, not full
// cursor addressing.
type liveWriter struct {
	buf *RingBuffer
	// rows above the append point the write cursor sits on; 0 = append
	up int
}

// feed resolves one newline-terminated segment against the buffer — resetting
// the pane on a screen clear, overwriting a recent line on a cursor-up/CR
// redraw, otherwise appending — and returns the ops it applied so a consumer
// can mirror the same edit.
func (lw *liveWriter) feed(raw string) []LineOp {
	raw = strings.TrimSuffix(raw, "\r")
	cleared := strings.IndexByte(raw, '\x1b') >= 0 && screenResetRe.MatchString(raw)
	moveUp, raw := extractCursorUp(raw)
	raw = collapseLineRewrites(raw)
	raw = stripNonColorANSI(raw)

	var ops []LineOp
	if cleared {
		lw.buf.Clear()
		lw.up = 0
		ops = append(ops, OpClear{})
		if raw == "" {
			return ops // a bare screen clear commits no line
		}
	}
	if moveUp > 0 {
		lw.up += moveUp
		if room := lw.buf.Len(); lw.up > room {
			lw.up = room
		}
		if raw == "" {
			return ops // cursor-only reposition, no reprint to commit yet
		}
	}
	if lw.up == 0 {
		lw.buf.Write(raw)
		ops = append(ops, OpAppend{Text: raw})
	} else {
		offset := lw.up - 1
		lw.buf.SetFromEnd(offset, raw)
		ops = append(ops, OpRewrite{OffsetFromEnd: offset, Text: raw})
		lw.up--
	}
	return ops
}

// screenResetRe matches full-screen clears a redrawing child emits before
// repainting: erase-display (ESC[2J), erase-scrollback (ESC[3J), and cursor-home
// (ESC[H and its 0/1 row;col forms). It deliberately excludes arbitrary cursor
// addressing (ESC[<row>;<col>H) and partial erases (ESC[0J/ESC[1J).
var screenResetRe = regexp.MustCompile(`\x1b\[[23]J|\x1b\[[01]?(?:;[01]?)?H`)

// cursorUpRe matches cursor-up sequences (ESC[<n>A). ESC[A and ESC[0A both mean
// up one line.
var cursorUpRe = regexp.MustCompile(`\x1b\[([0-9]*)A`)

// extractCursorUp removes cursor-up sequences and returns their summed line
// count plus the remaining string.
func extractCursorUp(s string) (int, string) {
	if !strings.Contains(s, "\x1b[") {
		return 0, s
	}
	total := 0
	rest := cursorUpRe.ReplaceAllStringFunc(s, func(match string) string {
		digits := cursorUpRe.FindStringSubmatch(match)[1]
		n := 1
		if digits != "" {
			if v, err := strconv.Atoi(digits); err == nil && v > 0 {
				n = v
			}
		}
		total += n
		return ""
	})
	return total, rest
}

var eraseAndHomeRe = regexp.MustCompile(`\x1b\[2K\x1b\[[01]?G`)

// text after the final carriage return or erase-line/cursor-home pair wins
func collapseLineRewrites(s string) string {
	start := strings.LastIndexByte(s, '\r') + 1
	matches := eraseAndHomeRe.FindAllStringIndex(s, -1)
	if len(matches) > 0 {
		if end := matches[len(matches)-1][1]; end > start {
			start = end
		}
	}
	return s[start:]
}

// readLines calls emit once per '\n'-terminated line (newline excluded), plus
// any unterminated remainder at EOF. A line longer than maxLine is truncated in
// what's passed to emit but still fully drained from r, so an oversized line
// can never block the writer on the other end of a pty.
func readLines(r io.Reader, maxLine int, emit func(string)) {
	reader := bufio.NewReader(r)
	buf := make([]byte, 32*1024)
	var line []byte
	overflow := false
	for {
		n, err := reader.Read(buf)
		chunk := buf[:n]
		for len(chunk) > 0 {
			segment := chunk
			complete := false
			if idx := bytes.IndexByte(chunk, '\n'); idx >= 0 {
				segment, chunk, complete = chunk[:idx], chunk[idx+1:], true
			} else {
				chunk = nil
			}
			if !overflow {
				if room := maxLine - len(line); len(segment) >= room {
					line = append(line, segment[:room]...)
					overflow = true
				} else {
					line = append(line, segment...)
				}
			}
			if complete {
				emit(string(line))
				line = line[:0]
				overflow = false
			}
		}
		if err != nil {
			if len(line) > 0 {
				emit(string(line))
			}
			return
		}
	}
}

// sendLine dispatches one output op to the registered sink.
func (pm *ProcessManager) sendLine(p *Process, op LineOp) {
	pm.mu.Lock()
	sink := pm.sink
	pm.mu.Unlock()

	if sink != nil {
		sink.OnLine(p.Entry.Label, op)
	}
}

func (pm *ProcessManager) AllExited() bool {
	pm.mu.Lock()
	procs := make([]*Process, len(pm.Processes))
	copy(procs, pm.Processes)
	pm.mu.Unlock()

	if len(procs) == 0 {
		return false
	}
	for _, p := range procs {
		p.mu.Lock()
		state := p.State
		p.mu.Unlock()
		if state != StateExited {
			return false
		}
	}
	return true
}

func (pm *ProcessManager) FailedCount() int {
	pm.mu.Lock()
	procs := make([]*Process, len(pm.Processes))
	copy(procs, pm.Processes)
	pm.mu.Unlock()

	count := 0
	for _, p := range procs {
		p.mu.Lock()
		exited := p.State == StateExited
		code := p.ExitCode
		p.mu.Unlock()
		if exited && code != 0 {
			count++
		}
	}
	return count
}

// WaitAllExited blocks until every process in procs has reached StateExited.
func (pm *ProcessManager) WaitAllExited(procs []*Process) {
	for _, p := range procs {
		p.mu.Lock()
		done := p.done
		p.mu.Unlock()
		<-done
	}
}

// findProc returns the process with the given label, or nil if not found.
func (pm *ProcessManager) findProc(label string) *Process {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, p := range pm.Processes {
		if p.Entry.Label == label {
			return p
		}
	}
	return nil
}

// sendState dispatches process state to the registered sink.
func (pm *ProcessManager) sendState(p *Process, state ProcessState, code int) {
	pm.mu.Lock()
	sink := pm.sink
	pm.mu.Unlock()

	if sink != nil {
		sink.OnState(p.Entry.Label, state, code)
	}
}

// nonColorANSIRe matches ANSI escape sequences that are NOT SGR (color/style).
// It strips:
//   - Cursor movement:   ESC [ <params> [A-HJKSTf]  (but NOT m which is SGR)
//   - Erase sequences:   ESC [ <params> [JK]
//   - Private mode:      ESC [ ? <params> [hl]
//   - Other non-color:   ESC [ <params> [n]
//
// It does NOT match ESC [ <params> m  (SGR — color/bold/etc.), so those are preserved.
var nonColorANSIRe = regexp.MustCompile(
	`\x1b\[\?[0-9;]*[hl]` + // private mode: ESC[?<n>h  ESC[?<n>l
		`|\x1b\[[0-9;]*[A-HJKSTfn]`, // cursor movement, erase, etc. (excludes 'm')
)

// stripNonColorANSI removes cursor/screen control sequences while preserving
// SGR (color and style) sequences.
func stripNonColorANSI(s string) string {
	return nonColorANSIRe.ReplaceAllString(s, "")
}
