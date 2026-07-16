package tui

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// streams process output as "[label] line" to w
type stdoutSink struct {
	w  io.Writer
	mu sync.Mutex
}

func (s *stdoutSink) OnOutput(label, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.w, "[%s] %s\n", label, line)
}

func (s *stdoutSink) OnState(label string, state ProcessState, code int) {
	if state != StateExited {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.w, "[%s] exited (%d)\n", label, code)
}

// starts concurrent companions immediately, then runs dependsOn levels
// sequentially (blocking per level) or the remaining processes in parallel.
// Mirrors the goroutine formerly inline in Launch.
func startProcesses(pm *ProcessManager, levels [][]TuiScriptEntry, concurrentProcs []*Process) {
	for _, proc := range concurrentProcs {
		if err := pm.Start(proc); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
		}
	}
	if levels != nil {
		for _, level := range levels {
			var levelProcs []*Process
			for _, entry := range level {
				proc := pm.findProc(entry.Label)
				if proc == nil {
					continue
				}
				if err := pm.Start(proc); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
				}
				levelProcs = append(levelProcs, proc)
			}
			pm.WaitAllExited(levelProcs)
			for _, proc := range levelProcs {
				if proc.ExitCode != 0 {
					return
				}
			}
		}
		return
	}
	for _, proc := range pm.Processes {
		if err := pm.Start(proc); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
		}
	}
}

// RunPlain orchestrates without chrome: streams "[label] line" to w and blocks
// until every process exits or SIGINT/SIGTERM arrives. Returns (true, nil) once
// it has run.
func RunPlain(pm *ProcessManager, w io.Writer, levels [][]TuiScriptEntry, concurrentProcs []*Process) (bool, error) {
	if len(pm.Processes) == 0 {
		return false, nil
	}
	pm.SetSink(&stdoutSink{w: w})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	go func() {
		startProcesses(pm, levels, concurrentProcs)
		pm.WaitAllExited(pm.Processes)
		close(done)
	}()

	select {
	case <-done:
	case <-sigCh:
	}

	pm.StopAll()
	if failed := pm.FailedCount(); failed > 0 {
		fmt.Fprintf(os.Stderr, "miso: %d of %d tasks failed\n", failed, len(pm.Processes))
	}
	return true, nil
}
