package tui

import (
	"bufio"
	"os/exec"
	"regexp"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ProcessState represents the lifecycle state of a managed process.
type ProcessState int

const (
	StateStarting ProcessState = iota
	StateRunning
	StateExited
)

// ProcessOutputMsg is a bubbletea message carrying a single captured output line.
type ProcessOutputMsg struct {
	Label string
	Line  string
}

// ProcessStateMsg is a bubbletea message indicating a process state change.
type ProcessStateMsg struct {
	Label string
	State ProcessState
	Code  int
}

// Process holds the runtime state for a single managed process.
type Process struct {
	Entry    TuiScriptEntry
	Command  string
	Args     []string
	Dir      string   // working directory for the process
	Environ  []string // environment variables for the process (nil = inherit)
	State    ProcessState
	ExitCode int
	Buffer   *RingBuffer

	cmd  *exec.Cmd
	done chan struct{}
	mu   sync.Mutex
}

// ProcessManager owns the set of managed processes and dispatches tea messages.
type ProcessManager struct {
	Processes []*Process
	mu        sync.Mutex
	program   *tea.Program
}

// NewProcessManager creates an empty ProcessManager.
func NewProcessManager() *ProcessManager {
	return &ProcessManager{}
}

// SetProgram registers the bubbletea program used for sending messages.
func (pm *ProcessManager) SetProgram(p *tea.Program) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.program = p
}

// Add creates a new Process entry and appends it to the manager.
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
	// Create a new process group so we can kill the entire tree on stop.
	p.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	p.mu.Unlock()

	cmd := p.cmd

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		p.mu.Lock()
		p.State = StateExited
		p.ExitCode = -1
		close(p.done)
		p.mu.Unlock()
		pm.sendState(p, StateExited, -1)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		p.mu.Lock()
		p.State = StateExited
		p.ExitCode = -1
		close(p.done)
		p.mu.Unlock()
		pm.sendState(p, StateExited, -1)
		return err
	}

	if err := cmd.Start(); err != nil {
		p.mu.Lock()
		p.State = StateExited
		p.ExitCode = -1
		close(p.done)
		p.mu.Unlock()
		pm.sendState(p, StateExited, -1)
		return err
	}

	p.mu.Lock()
	p.State = StateRunning
	p.mu.Unlock()
	pm.sendState(p, StateRunning, 0)

	var wg sync.WaitGroup
	wg.Add(2)

	go pm.captureOutput(p, stdoutPipe, &wg)
	go pm.captureOutput(p, stderrPipe, &wg)

	go func() {
		// Wait for both capture goroutines to finish draining the pipes.
		wg.Wait()
		// Now it is safe to call cmd.Wait() — the pipes are drained.
		exitErr := cmd.Wait()

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
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	select {
	case <-done:
		// Process group exited cleanly after SIGTERM.
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		// Wait for the done channel to be closed by the Start() goroutine.
		<-done
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
func (pm *ProcessManager) captureOutput(p *Process, r interface{ Read([]byte) (int, error) }, wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(r)
	const maxTokenSize = 1024 * 1024 // 1 MB
	buf := make([]byte, maxTokenSize)
	scanner.Buffer(buf, maxTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		line = stripNonColorANSI(line)
		p.Buffer.Write(line)
		pm.sendOutput(p, line)
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

// AllExited returns true if every process has reached StateExited.
func (pm *ProcessManager) AllExited() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if len(pm.Processes) == 0 {
		return false
	}
	for _, p := range pm.Processes {
		if p.State != StateExited {
			return false
		}
	}
	return true
}

// FailedCount returns the number of processes that exited with non-zero code.
func (pm *ProcessManager) FailedCount() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	count := 0
	for _, p := range pm.Processes {
		if p.State == StateExited && p.ExitCode != 0 {
			count++
		}
	}
	return count
}

// WaitAllExited blocks until every process in procs has reached StateExited.
func (pm *ProcessManager) WaitAllExited(procs []*Process) {
	for _, p := range procs {
		<-p.done
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
