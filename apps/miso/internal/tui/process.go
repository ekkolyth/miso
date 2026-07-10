package tui

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ekkolyth/miso/internal/proc"
)

type ProcessState int

const (
	StateStarting ProcessState = iota
	StateRunning
	StateExited
)

type ProcessOutputMsg struct {
	Label string
	Line  string
}

type ProcessStateMsg struct {
	Label string
	State ProcessState
	Code  int
}

// streams wired to a spawned process — unix: pty master is both reader and
// stdin writer; windows: separate stdout/stderr pipes + stdin pipe
type spawnResult struct {
	readers []io.Reader
	stdin   io.Writer
	resize  func(rows, cols int)
	closer  func()
}

// Process holds the runtime state for a single managed process.
type Process struct {
	Entry     TuiScriptEntry
	Command   string
	Args      []string
	Dir       string   // working directory for the process
	Environ   []string // environment variables for the process (nil = inherit)
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

// ProcessManager owns the set of managed processes and dispatches tea messages.
type ProcessManager struct {
	Processes []*Process
	mu        sync.Mutex
	program   *tea.Program
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{}
}

// SetProgram registers the bubbletea program used for sending messages.
func (pm *ProcessManager) SetProgram(p *tea.Program) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.program = p
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
	p.mu.Unlock()

	// A pty (unix) or pipes (Windows) — gives children a TTY + live stdin.
	res, err := spawnProcess(cmd, 0, 0)
	if err != nil {
		p.mu.Lock()
		p.State = StateExited
		p.ExitCode = -1
		close(p.done)
		p.mu.Unlock()
		pm.sendState(p, StateExited, -1)
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
		wg.Wait()
		exitErr := cmd.Wait()
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
		close(p.done)
		p.mu.Unlock()

		pm.sendState(p, StateExited, code)
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
		go func(proc *Process) {
			defer wg.Done()
			pm.Stop(proc)
		}(p)
	}
	wg.Wait()
}

// captureOutput reads lines from r, strips non-color ANSI sequences, writes
// them to the process buffer, and sends ProcessOutputMsg messages.
func (pm *ProcessManager) captureOutput(p *Process, r io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()

	// Retain up to 1MB per line; anything beyond is dropped from the stored line
	// but still consumed, so a child that writes a huge unbroken line (electron/
	// vite minified bundles) can't stall the pty and freeze the TUI.
	const maxLine = 1024 * 1024
	readLines(r, maxLine, func(raw string) {
		line := strings.TrimSuffix(raw, "\r")
		line = stripNonColorANSI(line)
		p.Buffer.Write(line)
		pm.sendOutput(p, line)
	})
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

// sendOutput dispatches a ProcessOutputMsg to the registered bubbletea program.
func (pm *ProcessManager) sendOutput(p *Process, line string) {
	pm.mu.Lock()
	prog := pm.program
	pm.mu.Unlock()

	if prog != nil {
		prog.Send(ProcessOutputMsg{Label: p.Entry.Label, Line: line})
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

// sendState dispatches a ProcessStateMsg to the registered bubbletea program.
func (pm *ProcessManager) sendState(p *Process, state ProcessState, code int) {
	pm.mu.Lock()
	prog := pm.program
	pm.mu.Unlock()

	if prog != nil {
		prog.Send(ProcessStateMsg{Label: p.Entry.Label, State: state, Code: code})
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
