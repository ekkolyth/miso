package tui

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/ekkolyth/miso/internal/cli/env"
	"github.com/ekkolyth/miso/internal/proc"
	"github.com/ekkolyth/miso/internal/ui"
)

type ProcessState int

const (
	StateStarting ProcessState = iota
	StateRunning
	StateExited
	// never started because a dependency failed
	StateSkipped
)

// LineOp is one edit a child made to its output — append a tail line, overwrite
// a recent one, or clear the pane — so a consumer can mirror an in-place redraw
// instead of re-appending every frame.
type LineOp interface{ lineOp() }

// Pending on OpAppend and OpRewrite marks a line the child hasn't ended yet;
// the next non-pending op for that label is an OpRewrite committing the same row.
type OpAppend struct {
	Text    string
	Pending bool
}

// OffsetFromEnd 0 is the newest line, matching RingBuffer.SetFromEnd.
type OpRewrite struct {
	OffsetFromEnd int
	Text          string
	Pending       bool
}

type OpClear struct{}

// drops the newest Count lines, the rows a child erased below its cursor
type OpTruncate struct{ Count int }

func (OpAppend) lineOp()   {}
func (OpRewrite) lineOp()  {}
func (OpClear) lineOp()    {}
func (OpTruncate) lineOp() {}

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
	Dir       string                                                  // working directory for the process
	BuildEnv  func() (environ []string, preamble []string, err error) // runs on every Start; a nil environ inherits
	NoPTY     bool                                                    // plain mode: pipe stdio (no pty) so a tool self-formats for a non-tty
	Preamble  []string                                                // miso's own lines, emitted into the pane before the child starts
	State     ProcessState
	ExitCode  int
	StartedAt time.Time
	Buffer    *RingBuffer

	cmd    *exec.Cmd
	stdin  io.Writer // writable child stdin (pty master on unix, pipe on Windows)
	resize func(rows, cols int)
	done   chan struct{}
	mu     sync.Mutex
	// one Start, Stop, Skip or Restart at a time, so a Stop waits for a Start
	// still building env instead of missing the child it is about to spawn
	lifecycle sync.Mutex
}

// ProcessManager owns the set of managed processes and dispatches output/state to a sink.
type ProcessManager struct {
	Processes []*Process
	mu        sync.Mutex
	sink      OutputSink
	// set by StopAll so a quit can't be followed by a restart spawning children
	stopped bool
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
		Entry:    entry,
		Command:  command,
		Args:     args,
		Dir:      dir,
		BuildEnv: func() ([]string, []string, error) { return environ, nil, nil },
		State:    StateStarting,
		Buffer:   NewRingBuffer(DefaultBufferSize),
		done:     make(chan struct{}),
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
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	return pm.start(p)
}

func (pm *ProcessManager) start(p *Process) error {
	pm.mu.Lock()
	quitting := pm.stopped
	pm.mu.Unlock()
	if quitting {
		return nil
	}
	// a process has at most one live child
	pm.stop(p)

	p.mu.Lock()
	p.State = StateStarting
	p.ExitCode = 0
	done := make(chan struct{})
	p.done = done
	buildEnv := p.BuildEnv
	command := p.Command
	args := p.Args
	dir := p.Dir
	noPTY := p.NoPTY
	preamble := append([]string(nil), p.Preamble...)
	p.mu.Unlock()

	// file I/O, so it runs outside the lock
	environ, envPreamble, envErr := buildEnv()
	if envErr != nil {
		lines := envErrorLines(envErr)
		lines[0] = ui.Default().MisoLine(lines[0])
		for _, line := range lines {
			p.Buffer.Write(line)
			pm.sendLine(p, OpAppend{Text: line})
		}
		p.mu.Lock()
		p.State = StateExited
		p.ExitCode = -1
		p.mu.Unlock()
		pm.sendState(p, StateExited, -1)
		close(done)
		return envErr
	}

	p.mu.Lock()
	p.cmd = exec.Command(command, args...)
	if dir != "" {
		p.cmd.Dir = dir
	}
	if environ != nil {
		p.cmd.Env = environ
	}
	cmd := p.cmd
	p.mu.Unlock()

	// Emitted before the child's first byte so a pane opens with what miso did
	// on its behalf. Buffer write mirrors captureOutput so every renderer — the
	// buffer-rendering ones and the op-streaming plain sink — sees it the same.
	for _, line := range append(preamble, envPreamble...) {
		p.Buffer.Write(line)
		pm.sendLine(p, OpAppend{Text: line})
	}

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

// renders a BuildEnv failure as pane lines: a validation error expands to its
// full report, anything else is a single line
func envErrorLines(err error) []string {
	var validationErr *env.ValidationError
	if !errors.As(err, &validationErr) {
		return []string{err.Error()}
	}
	var rendered bytes.Buffer
	validationErr.Render(&rendered)
	var lines []string
	for _, line := range strings.Split(rendered.String(), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return []string{err.Error()}
	}
	return lines
}

// Stop sends SIGTERM to the process, waits up to 5 seconds, then SIGKILLs.
// It does NOT call cmd.Wait() — the goroutine started in Start() handles that.
func (pm *ProcessManager) Stop(p *Process) {
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	pm.stop(p)
}

func (pm *ProcessManager) stop(p *Process) {
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

// no-op once the process has spawned
func (pm *ProcessManager) Skip(p *Process) {
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	p.mu.Lock()
	if p.State != StateStarting || p.cmd != nil {
		p.mu.Unlock()
		return
	}
	p.State = StateSkipped
	close(p.done)
	p.mu.Unlock()
	pm.sendState(p, StateSkipped, 0)
}

// Restart stops the process and starts it again.
func (pm *ProcessManager) Restart(p *Process) error {
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	pm.mu.Lock()
	quitting := pm.stopped
	pm.mu.Unlock()
	if quitting {
		return nil
	}
	pm.stop(p)
	p.Buffer.Clear()
	return pm.start(p)
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
	pm.stopped = true
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
	p.mu.Lock()
	noPTY := p.NoPTY
	p.mu.Unlock()
	// plain output stays one line per newline, so only a pty child's prompt or
	// live echo is painted before its newline arrives
	var partial func(string)
	if !noPTY {
		partial = func(raw string) {
			for _, op := range live.feedPartial(raw) {
				pm.sendLine(p, op)
			}
		}
	}
	readLines(r, maxLine, func(raw string) {
		for _, op := range live.feed(raw) {
			pm.sendLine(p, op)
		}
	}, partial)
}

// liveWriter maps a child's redraw stream onto in-place buffer edits so a tool
// that repaints its output — docker compose's container block, a spinner, a
// vite banner after a screen clear, a clack prompt — overwrites or resets
// recent lines instead of appending a fresh copy each tick. It follows the
// cursor up and down through recent rows, erase-below, and screen clears, not
// full cursor addressing.
type liveWriter struct {
	buf *RingBuffer
	// rows above the append point the write cursor sits on; 0 = append
	up int
	// a painted line still waiting for its newline, at pendingOffset from the end
	pending       bool
	pendingOffset int
	lastPainted   string
	// bytes of the open line already applied — everything up to its last cursor
	// move, which a prompt redrawing in place may never follow with a newline
	applied int
}

// feed resolves one newline-terminated segment against the buffer — resetting
// the pane on a screen clear, overwriting a recent line on a cursor-up/CR
// redraw, otherwise appending — and returns the ops it applied so a consumer
// can mirror the same edit.
func (lw *liveWriter) feed(raw string) []LineOp {
	// a pty turns a cooked child's \n into \r\n, so a bare \n comes from a
	// raw-mode TUI (bubbletea) stepping over a row it left unchanged
	bareLineFeed := !strings.HasSuffix(raw, "\r")
	raw = strings.TrimSuffix(raw, "\r")
	ops, rest, moved := lw.applyMoves(raw)
	lw.applied = 0
	raw = stripNonColorANSI(resolveLine(rest))

	if bareLineFeed && rest == "" && lw.up > 0 && !lw.pending {
		lw.up--
		return ops
	}
	if lw.pending {
		lw.pending = false
		lw.buf.SetFromEnd(lw.pendingOffset, raw)
		if lw.up > 0 {
			lw.up--
		}
		return append(ops, OpRewrite{OffsetFromEnd: lw.pendingOffset, Text: raw})
	}
	if moved && raw == "" {
		return ops // cursor-only reposition, no reprint to commit yet
	}
	if lw.up == 0 {
		lw.buf.Write(raw)
		return append(ops, OpAppend{Text: raw})
	}
	offset := lw.up - 1
	lw.buf.SetFromEnd(offset, raw)
	lw.up--
	return append(ops, OpRewrite{OffsetFromEnd: offset, Text: raw})
}

// feedPartial paints a line the child hasn't ended yet — a prompt, typed
// echo, a progress frame — at the row feed would commit it to
func (lw *liveWriter) feedPartial(raw string) []LineOp {
	if len(raw) > maxPartialLine {
		return nil
	}
	raw = incompleteEscapeRe.ReplaceAllString(trimIncompleteRune(raw), "")
	ops, rest, moved := lw.applyMoves(raw)
	// a half-written row of a multi-row redraw frame waits for its newline
	if moved {
		return ops
	}
	text := stripNonColorANSI(resolveLine(rest))

	if lw.pending {
		if text == lw.lastPainted {
			return ops
		}
		lw.buf.SetFromEnd(lw.pendingOffset, text)
		lw.lastPainted = text
		return append(ops, OpRewrite{OffsetFromEnd: lw.pendingOffset, Text: text, Pending: true})
	}
	if text == "" {
		return ops
	}
	var op LineOp
	if lw.up == 0 {
		lw.buf.Write(text)
		lw.pendingOffset = 0
		op = OpAppend{Text: text, Pending: true}
	} else {
		lw.pendingOffset = lw.up - 1
		lw.buf.SetFromEnd(lw.pendingOffset, text)
		op = OpRewrite{OffsetFromEnd: lw.pendingOffset, Text: text, Pending: true}
	}
	lw.pending = true
	lw.lastPainted = text
	return append(ops, op)
}

// applyMoves walks the open line's cursor moves not yet applied, in order: the
// text before each move lands on the row the cursor is on, then the move
// repositions it. It returns the text after the last move and whether the open
// line has moved the cursor at all.
func (lw *liveWriter) applyMoves(raw string) (ops []LineOp, rest string, moved bool) {
	base := lw.applied
	for _, loc := range cursorMoveRe.FindAllStringSubmatchIndex(raw[base:], -1) {
		start, end := base+loc[0], base+loc[1]
		chunk := raw[lw.applied:start]
		ops = append(ops, lw.writeInPlace(stripNonColorANSI(resolveLine(chunk)), eraseLineRe.MatchString(chunk))...)
		count := ""
		if loc[2] >= 0 {
			count = raw[base+loc[2] : base+loc[3]]
		}
		ops = append(ops, lw.move(raw[start:end], count, ops)...)
		lw.applied = end
	}
	return ops, raw[lw.applied:], lw.applied > 0
}

// writeInPlace puts text on the cursor's row without ending the line, so the
// cursor stays on that row for the move that follows. A row the child erased
// and left empty is blanked — dialoguer clears its prompt row by row this way
func (lw *liveWriter) writeInPlace(text string, erased bool) []LineOp {
	if lw.pending {
		lw.pending = false
		lw.buf.SetFromEnd(lw.pendingOffset, text)
		lw.up = lw.pendingOffset + 1
		return []LineOp{OpRewrite{OffsetFromEnd: lw.pendingOffset, Text: text}}
	}
	if text == "" && (!erased || lw.up == 0) {
		return nil
	}
	if lw.up == 0 {
		lw.buf.Write(text)
		lw.up = 1
		return []LineOp{OpAppend{Text: text}}
	}
	lw.buf.SetFromEnd(lw.up-1, text)
	return []LineOp{OpRewrite{OffsetFromEnd: lw.up - 1, Text: text}}
}

func (lw *liveWriter) move(seq, count string, prior []LineOp) []LineOp {
	n := 1
	if v, err := strconv.Atoi(count); err == nil && v > 0 {
		n = v
	}
	switch seq[len(seq)-1] {
	case 'A':
		lw.up = min(lw.up+n, lw.buf.Len())
	case 'B':
		lw.up = max(lw.up-n, 0)
	case 'J':
		if seq == "\x1b[J" || seq == "\x1b[0J" {
			// rows below the cursor's go; its own row is repainted by what follows
			if lw.up > 1 {
				lw.buf.TruncateEnd(lw.up - 1)
				dropped := lw.up - 1
				lw.up = 1
				return []LineOp{OpTruncate{Count: dropped}}
			}
			return nil
		}
		return lw.clear(prior)
	default:
		return lw.clear(prior)
	}
	return nil
}

func (lw *liveWriter) clear(prior []LineOp) []LineOp {
	lw.buf.Clear()
	lw.up = 0
	// ESC[2J ESC[H is one reset, not two
	if len(prior) > 0 {
		if _, ok := prior[len(prior)-1].(OpClear); ok {
			return nil
		}
	}
	return []LineOp{OpClear{}}
}

// each read re-resolves the whole open line, so painting stops at a length no
// prompt or progress frame reaches and the line waits for its newline
const maxPartialLine = 64 * 1024

// a read can end partway through an escape sequence
var incompleteEscapeRe = regexp.MustCompile(`\x1b(\[[0-9;?]*)?$`)

// a read can end partway through a multi-byte rune
func trimIncompleteRune(s string) string {
	for i := len(s) - 1; i >= 0 && i >= len(s)-utf8.UTFMax+1; i-- {
		if utf8.RuneStart(s[i]) {
			if !utf8.FullRuneInString(s[i:]) {
				return s[:i]
			}
			return s
		}
	}
	return s
}

// erase-to-end-of-line or erase-whole-line
var eraseLineRe = regexp.MustCompile(`\x1b\[[02]?K`)

// cursorMoveRe matches the sequences that move the write cursor between rows:
// cursor-up and cursor-down (ESC[<n>A, ESC[<n>B; no count or 0 means one),
// erase-below (ESC[J, ESC[0J), and the full-screen clears a redrawing child
// emits before repainting — erase-display (ESC[2J), erase-scrollback (ESC[3J),
// and cursor-home (ESC[H and its 0/1 row;col forms). It deliberately excludes
// arbitrary cursor addressing (ESC[<row>;<col>H) and erase-above (ESC[1J).
var cursorMoveRe = regexp.MustCompile(`\x1b\[([0-9]*)[AB]|\x1b\[[023]?J|\x1b\[[01]?(?:;[01]?)?H`)

type lineCell struct {
	seq   string
	width int
}

// resolveLine replays the in-line cursor edits a child made (\r, \b, ESC[<n>D,
// ESC[K, ESC[2K, ESC[G) against a column model and returns what a terminal
// would show. A line the child erased before it ended keeps its last non-empty
// frame, so a progress line leaves its final state in the scrollback.
func resolveLine(s string) string {
	if !strings.ContainsAny(s, "\r\b\x1b") {
		return s
	}
	var (
		cells     []lineCell
		cursor    int
		lastFrame string
		state     byte
	)
	parser := ansi.NewParser()
	snapshot := func(from int) {
		for _, c := range cells[from:] {
			if c.width > 0 {
				lastFrame = joinCells(cells)
				return
			}
		}
	}
	// SGR and other zero-width cells survive an erase so the color of the text
	// before the cursor stays intact
	eraseFrom := func(from int) {
		kept := cells[:from:from]
		for _, c := range cells[from:] {
			if c.width == 0 {
				kept = append(kept, c)
			}
		}
		cells = kept
	}
	insert := func(c lineCell) {
		cells = append(cells, lineCell{})
		copy(cells[cursor+1:], cells[cursor:])
		cells[cursor] = c
		cursor++
	}

	for len(s) > 0 {
		seq, width, n, next := ansi.DecodeSequence(s, state, parser)
		state = next
		s = s[n:]
		if width > 0 {
			for cursor < len(cells) && cells[cursor].width == 0 {
				cursor++
			}
			if cursor == len(cells) {
				cells = append(cells, lineCell{seq, width})
			} else {
				cells[cursor] = lineCell{seq, width}
			}
			cursor++
			continue
		}
		switch {
		case seq == "\r":
			snapshot(0)
			cells, cursor = nil, 0
		case seq == "\b":
			cursor = backColumns(cells, cursor, 1)
		case ansi.HasCsiPrefix(seq) && isPlainCsi(parser):
			param, _ := parser.Param(0, 0)
			switch ansi.Cmd(parser.Command()).Final() {
			case 'D':
				cursor = backColumns(cells, cursor, max(param, 1))
			case 'K':
				switch param {
				case 0:
					snapshot(cursor)
					eraseFrom(cursor)
				case 2:
					snapshot(0)
					eraseFrom(0)
					cursor = 0
				default:
					insert(lineCell{seq: seq})
				}
			case 'G':
				if param <= 1 {
					cursor = 0
				} else {
					insert(lineCell{seq: seq})
				}
			default:
				insert(lineCell{seq: seq})
			}
		default:
			insert(lineCell{seq: seq})
		}
	}

	for _, c := range cells {
		if c.width > 0 {
			return joinCells(cells)
		}
	}
	if lastFrame != "" {
		return lastFrame
	}
	return joinCells(cells)
}

func isPlainCsi(p *ansi.Parser) bool {
	cmd := ansi.Cmd(p.Command())
	return cmd.Prefix() == 0 && cmd.Intermediate() == 0
}

// clamps at column 0: bun sizes its cursor-back by the frame's bytes, not its
// columns, so it routinely overshoots the start of the line
func backColumns(cells []lineCell, cursor, cols int) int {
	for cursor > 0 && cols > 0 {
		cursor--
		cols -= cells[cursor].width
	}
	return cursor
}

func joinCells(cells []lineCell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteString(c.seq)
	}
	return b.String()
}

// readLines calls emit once per '\n'-terminated line (newline excluded), plus
// any unterminated remainder at EOF. A non-nil partial gets the whole
// unterminated line so far after every read that leaves one open, until the
// line overflows maxLine. A line longer than maxLine is truncated in what's
// passed to emit but still fully drained from r, so an oversized line can
// never block the writer on the other end of a pty.
func readLines(r io.Reader, maxLine int, emit func(string), partial func(string)) {
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
		if partial != nil && len(line) > 0 && !overflow && err == nil {
			partial(string(line))
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
		if state != StateExited && state != StateSkipped {
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
