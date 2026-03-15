package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ekkolyth/miso/internal/config"
)

// turboLineRe matches "workspace:task: output text"
// turboLineRe matches "workspace:task: output text" where task may contain
// colons (e.g. "db:studio"), so the label is everything up to the last ": ".
var turboLineRe = regexp.MustCompile(`^([a-zA-Z0-9@/_.-]+:[a-zA-Z0-9:_.-]+): (.*)$`)

// nxHeaderRe matches "> nx run workspace:task"
var nxHeaderRe = regexp.MustCompile(`^> nx run ([a-zA-Z0-9@/_.-]+:[a-zA-Z0-9_.-]+)$`)

func parseTurboLine(line string) (label, text string) {
	m := turboLineRe.FindStringSubmatch(line)
	if m == nil {
		return "", line
	}
	return m[1], m[2]
}

func parseNxHeader(line string) (label string, isHeader bool) {
	m := nxHeaderRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// DelegateLaunch spawns turbo or nx as a single process and renders its output
// in the miso TUI. Returns (true, nil) if the TUI ran successfully.
func DelegateLaunch(cfg config.Config, scriptName string, root string) (bool, error) {
	if !cfg.TuiEnabled() {
		return false, nil
	}

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
		delegateArgs = []string{"run", scriptName, "--log-order=stream"}
	case "nx":
		delegateArgs = []string{"run-many", "--target=" + scriptName}
	default:
		return false, fmt.Errorf("unsupported delegated mode: %s", mode)
	}

	// Build the command
	cmd := exec.Command(binary, delegateArgs...)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
	switch cfg.Tui {
	case "tabbed":
		model = NewTabbedModel(pm, scriptName, true)
	case "merged":
		model = NewMergedModel(pm, scriptName, true)
	default:
		return false, fmt.Errorf("unknown tui mode: %s", cfg.Tui)
	}

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	pm.SetProgram(p)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		p.Quit()
	}()
	defer signal.Stop(sigCh)

	// parseDelegatedLine extracts a workspace label and text from a line of
	// turbo/nx output. Returns the label and text if matched, or empty label
	// if the line is unmatched boilerplate (which should be discarded).
	parseLine := func(line string, currentNxLabel *string) (label, text string, skip bool) {
		switch mode {
		case "turbo":
			// Try to parse as a workspace-prefixed line first
			label, text = parseTurboLine(line)
			if label != "" {
				return label, text, false
			}
			// Unmatched line — skip turbo's own boilerplate (• prefix lines)
			return "", "", true
		case "nx":
			if hdrLabel, isHdr := parseNxHeader(line); isHdr {
				*currentNxLabel = hdrLabel
				return "", "", true
			}
			if *currentNxLabel != "" {
				label = *currentNxLabel
				text = line
			}
		}
		return label, text, false
	}

	// routeLine sends a parsed line to the correct process tab, creating it on
	// the fly if this is a new workspace label.
	routeLine := func(label, text string) {
		if label == "" {
			return // discard unmatched lines (turbo/nx boilerplate)
		}
		proc := pm.findProc(label)
		if proc == nil {
			entry := TuiScriptEntry{Label: label, ScriptName: scriptName, WorkspaceDir: root}
			proc = pm.Add(entry, "", nil, root)
			proc.State = StateRunning
			pm.sendState(proc, StateRunning, 0)
		}
		proc.Buffer.Write(text)
		pm.sendOutput(proc, text)
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

		// Parse stdout — workspace-prefixed lines become tabs
		go func() {
			scanner := bufio.NewScanner(stdout)
			currentNxLabel := ""
			for scanner.Scan() {
				line := stripNonColorANSI(scanner.Text())
				label, text, skip := parseLine(line, &currentNxLabel)
				if skip {
					continue
				}
				routeLine(label, text)
			}
		}()

		// Parse stderr — same logic so workspace errors go to the right tab
		go func() {
			scanner := bufio.NewScanner(stderr)
			currentNxLabel := ""
			for scanner.Scan() {
				line := stripNonColorANSI(scanner.Text())
				label, text, skip := parseLine(line, &currentNxLabel)
				if skip {
					continue
				}
				routeLine(label, text)
			}
		}()

		// Wait for the delegated process to exit
		exitErr := cmd.Wait()
		code := 0
		if exitErr != nil {
			if exitError, ok := exitErr.(*exec.ExitError); ok {
				code = exitError.ExitCode()
			} else {
				code = -1
			}
		}
		pm.mu.Lock()
		for _, proc := range pm.Processes {
			proc.State = StateExited
			proc.ExitCode = code
		}
		pm.mu.Unlock()
		for _, proc := range pm.Processes {
			pm.sendState(proc, StateExited, code)
		}
	}()

	_, err = p.Run()
	// Kill the delegated process group
	if cmd.Process != nil {
		pgid := cmd.Process.Pid
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}

	failed := pm.FailedCount()
	if failed > 0 {
		total := len(pm.Processes)
		fmt.Fprintf(os.Stderr, "miso: %d of %d tasks failed\n", failed, total)
	}

	return true, err
}
