package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/proc"
	"github.com/ekkolyth/miso/internal/turbo"
)

// nxHeaderRe matches "> nx run workspace:task"
var nxHeaderRe = regexp.MustCompile(`^> nx run ([a-zA-Z0-9@/_.-]+:[a-zA-Z0-9_.-]+)$`)

func parseNxHeader(line string) (label string, isHeader bool) {
	m := nxHeaderRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// metaTabLabel is the synthetic tab carrying turbo's own (non-task) output; it
// is pinned below the real workspace tabs.
const metaTabLabel = "turbo"

// FORCE_COLOR=1 unless the caller already set FORCE_COLOR or opted out via
// NO_COLOR (NO_COLOR wins, matching turbo/nx and the no-color.org convention).
func delegatedColorEnv(base []string) []string {
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if key == "NO_COLOR" || key == "FORCE_COLOR" {
			return base
		}
	}
	return append(base, "FORCE_COLOR=1")
}

// delegateFilterArgs builds the scope-filter flags for a delegated run. turbo
// takes one --filter=<name> per member; nx takes a single comma-joined
// --projects=<a>,<b>.
func delegateFilterArgs(mode string, filters []string) []string {
	if len(filters) == 0 {
		return nil
	}
	switch mode {
	case "nx":
		return []string{"--projects=" + strings.Join(filters, ",")}
	default: // turbo
		out := make([]string, 0, len(filters))
		for _, name := range filters {
			out = append(out, "--filter="+name)
		}
		return out
	}
}

// DelegateLaunch spawns turbo or nx: chrome (the miso TUI) with an
// interactive TTY and tui != off, plain with inherited stdio otherwise —
// forwarding SIGINT/SIGTERM either way. A resolved delegate task always runs
// one of the two, never falling through to package-manager execution, so a
// nonzero delegate exit surfaces as a non-nil error.
//
// The delegated process is deliberately NOT given a pseudo-terminal (unlike the
// direct-spawn ProcessManager.Start path). turbo and nx orchestrate their own
// child processes — and turbo runs its own TTY-aware UI — so miso supplies only
// the chrome (sidebar/tabs + log rendering) and leaves process lifecycle and
// stdin to them. A pty here would collide with turbo's own terminal UI.
func DelegateLaunch(cfg config.Config, scriptName string, root string, extraArgs []string, filters []string) (bool, error) {
	mode := cfg.RepoMode()

	// Verify the binary exists
	binary := mode // "turbo" or "nx"
	if _, err := exec.LookPath(binary); err != nil {
		return false, fmt.Errorf("%s not found in PATH — install it or change repo mode", binary)
	}

	// Build the args list separately (without argv[0]) so pm.Restart works correctly.
	var delegateArgs []string
	switch mode {
	case "turbo":
		delegateArgs = append([]string{"run", scriptName, "--log-order=stream"}, delegateFilterArgs(mode, filters)...)
		delegateArgs = append(delegateArgs, extraArgs...)
	case "nx":
		delegateArgs = append([]string{"run-many", "--target=" + scriptName}, delegateFilterArgs(mode, filters)...)
	default:
		return false, fmt.Errorf("unsupported delegated mode: %s", mode)
	}

	// miso pipes turbo/nx output, so they'd see a non-tty and strip color.
	// FORCE_COLOR mirrors what a real terminal gives them: turbo/nx run their
	// normal color path and forward FORCE_COLOR to tasks, matching a direct
	// `turbo run`. NO_COLOR still wins per the shared convention.
	env := delegatedColorEnv(os.Environ())

	hasTTY := hasInteractiveTTY()
	if SelectRenderer(cfg, hasTTY) == RendererPlain {
		if !hasTTY && len(filters) == 0 {
			fmt.Fprintln(os.Stderr, "miso: no interactive terminal — running plain")
		}
		return delegateRunPlain(mode, binary, delegateArgs, root, env)
	}

	// Build the command
	cmd := exec.Command(binary, delegateArgs...)
	cmd.Dir = root
	cmd.Env = env
	proc.SetGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("pipe stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return false, fmt.Errorf("pipe stderr: %w", err)
	}

	// Create process manager — no pre-created meta-tab; tabs are discovered
	// dynamically as workspace-prefixed output arrives from turbo/nx.
	pm := NewProcessManager()

	var model tea.Model
	switch cfg.TuiMode {
	case "tabbed":
		model = NewTabbedModel(pm, scriptName, true)
	case "merged":
		model = NewMergedModel(pm, scriptName, true)
	default:
		return false, fmt.Errorf("unknown tui mode: %s", cfg.TuiMode)
	}

	p := tea.NewProgram(model)
	pm.SetSink(programSink{prog: p})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		p.Quit()
	}()
	defer signal.Stop(sigCh)

	// routeBasic handles the simple (label, text) routing used by nx and as a fallback.
	routeBasic := func(label, text string) {
		if label == "" {
			return
		}
		process := pm.findProc(label)
		if process == nil {
			entry := TuiScriptEntry{Label: label, ScriptName: scriptName, WorkspaceDir: root}
			process = pm.Add(entry, "", nil, root, nil)
			process.State = StateRunning
			pm.sendState(process, StateRunning, 0)
			pm.PinLast(metaTabLabel)
		}
		process.Buffer.Write(text)
		pm.sendOutput(process, text)
	}

	// routeTurbo handles turbo output with per-task exit codes and cache metadata.
	// Lines with no task prefix (turbo's own orchestration, warnings, errors) are
	// surfaced under a "turbo" tab instead of dropped — otherwise a failed or
	// task-less run leaves the TUI blank and quits with no explanation.
	routeTurbo := func(meta turbo.LineMeta, raw string) {
		if meta.Label == "" {
			if strings.TrimSpace(raw) != "" {
				routeBasic(metaTabLabel, raw)
			}
			return
		}
		process := pm.findProc(meta.Label)
		if process == nil {
			entry := TuiScriptEntry{Label: meta.Label, ScriptName: scriptName, WorkspaceDir: root}
			process = pm.Add(entry, "", nil, root, nil)
			process.State = StateRunning
			pm.sendState(process, StateRunning, 0)
			pm.PinLast(metaTabLabel)
		}
		if meta.IsExit {
			process.mu.Lock()
			process.State = StateExited
			process.ExitCode = meta.ExitCode
			process.mu.Unlock()
			pm.sendState(process, StateExited, meta.ExitCode)
			return
		}
		process.Buffer.Write(meta.Text)
		pm.sendOutput(process, meta.Text)
	}

	// Start the delegated process and output parsing in a goroutine.
	// prog.Send() blocks until bubbletea's event loop is running, so all
	// sends must happen after p.Run() begins — otherwise we deadlock.
	go func() {
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "miso: failed to start %s: %v\n", mode, err)
			p.Quit()
			return
		}

		var scanWg sync.WaitGroup
		scanWg.Add(2)

		scanPipe := func(r io.Reader) {
			defer scanWg.Done()
			scanner := bufio.NewScanner(r)
			currentNxLabel := ""
			for scanner.Scan() {
				line := stripNonColorANSI(scanner.Text())
				switch mode {
				case "turbo":
					routeTurbo(turbo.ParseLine(line), line)
				case "nx":
					if hdrLabel, isHdr := parseNxHeader(line); isHdr {
						currentNxLabel = hdrLabel
						continue
					}
					if currentNxLabel != "" {
						routeBasic(currentNxLabel, line)
					}
				}
			}
		}

		go scanPipe(stdout)
		go scanPipe(stderr)

		// Drain scanners before waiting for process exit
		scanWg.Wait()

		exitErr := cmd.Wait()
		code := 0
		if exitErr != nil {
			if exitError, ok := exitErr.(*exec.ExitError); ok {
				code = exitError.ExitCode()
			} else {
				code = -1
			}
		}

		// Fallback: assign exit code only to processes that didn't get individual codes
		pm.mu.Lock()
		procs := make([]*Process, len(pm.Processes))
		copy(procs, pm.Processes)
		nProcs := len(procs)
		pm.mu.Unlock()

		for _, process := range procs {
			process.mu.Lock()
			if process.State != StateExited {
				process.State = StateExited
				process.ExitCode = code
			}
			process.mu.Unlock()
			pm.sendState(process, StateExited, process.ExitCode)
		}

		// If turbo exited before producing any workspace output (e.g. config error),
		// no processes were created so the TUI will hang. Quit immediately.
		if nProcs == 0 {
			p.Quit()
		}
	}()

	_, err = p.Run()

	// Dump buffered logs to stdout so they survive the alt-screen restore.
	if !cfg.TuiCleanExit {
		DumpLogs(pm)
	}

	// Kill the delegated process group
	if cmd.Process != nil {
		pgid := cmd.Process.Pid
		_ = proc.KillGroup(pgid, syscall.SIGTERM)
	}

	failed := pm.FailedCount()
	if failed > 0 {
		total := len(pm.Processes)
		fmt.Fprintf(os.Stderr, "miso: %d of %d tasks failed\n", failed, total)
	}

	return true, err
}

// delegateRunPlain execs the delegate binary directly with inherited stdio —
// no pty, no ProcessManager, no bubbletea. turbo/nx render their own output
// straight to the terminal (or pipe). SIGINT/SIGTERM stop miso itself, so they
// are forwarded to the delegate's process group the same way ExecScriptFile
// forwards them to a spawned script: SIGTERM first, SIGKILL after a grace
// period if it hasn't exited. A signal-triggered stop is not itself reported
// as an error — only the delegate's own nonzero exit is.
func delegateRunPlain(mode, binary string, delegateArgs []string, root string, env []string) (bool, error) {
	cmd := exec.Command(binary, delegateArgs...)
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	proc.SetGroup(cmd)

	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("start %s: %w", mode, err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-sigCh:
		pgid := cmd.Process.Pid
		_ = proc.KillGroup(pgid, syscall.SIGTERM)
		killTimer := time.AfterFunc(2*time.Second, func() { _ = proc.KillGroup(pgid, syscall.SIGKILL) })
		<-done
		killTimer.Stop() // delegate already exited; don't SIGKILL a recycled pid
		return true, nil
	case err := <-done:
		return true, err
	}
}
