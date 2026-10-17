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

// every append/rewrite prints a line — the same per-line stream as before;
// clear prints nothing
func (s *stdoutSink) OnLine(label string, op LineOp) {
	var text string
	switch o := op.(type) {
	case OpAppend:
		text = o.Text
	case OpRewrite:
		text = o.Text
	default:
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.w, "[%s] %s\n", label, text)
}

func (s *stdoutSink) OnState(label string, state ProcessState, code int) {
	if state != StateExited {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.w, "[%s] exited (%d)\n", label, code)
}

// starts concurrent companions immediately, then each dependsOn task as soon
// as its own dependencies exit, so a long-running task never holds back one
// that doesn't depend on it. A task whose dependency failed or was skipped is
// skipped, and so are its dependents; unrelated tasks keep going. Without
// levels every process starts in parallel. Blocks until every task in levels
// has exited or been skipped
func startProcesses(pm *ProcessManager, levels [][]TuiScriptEntry, deps map[string][]string, concurrentProcs []*Process) {
	for _, proc := range concurrentProcs {
		startOrWarn(pm, proc)
	}
	if levels == nil {
		for _, proc := range pm.Processes {
			startOrWarn(pm, proc)
		}
		return
	}

	// closed once the task has exited or been skipped; Process.done can't
	// serve, since Start replaces it
	finished := make(map[string]chan struct{})
	for _, level := range levels {
		for _, entry := range level {
			finished[entry.Label] = make(chan struct{})
		}
	}

	var wg sync.WaitGroup
	for _, level := range levels {
		for _, entry := range level {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer close(finished[entry.Label])
				proc := pm.findProc(entry.Label)
				if proc == nil {
					return
				}
				depsSucceeded := true
				for _, dep := range deps[entry.Label] {
					depFinished, ok := finished[dep]
					if !ok {
						continue
					}
					<-depFinished
					if !exitedCleanly(pm.findProc(dep)) {
						depsSucceeded = false
					}
				}
				if !depsSucceeded {
					pm.Skip(proc)
					return
				}
				startOrWarn(pm, proc)
				pm.WaitAllExited([]*Process{proc})
			}()
		}
	}
	wg.Wait()
}

func startOrWarn(pm *ProcessManager, proc *Process) {
	if err := pm.Start(proc); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
	}
}

// a missing process never ran, so it holds nothing back
func exitedCleanly(proc *Process) bool {
	if proc == nil {
		return true
	}
	proc.mu.Lock()
	defer proc.mu.Unlock()
	return proc.State == StateExited && proc.ExitCode == 0
}

// orchestrates without chrome: streams "[label] line" to w and blocks until
// every process exits or SIGINT/SIGTERM arrives. Returns (true, nil) on a clean
// run, (true, err) when a child exited non-zero, (false, nil) when there was
// nothing to run
func RunPlain(pm *ProcessManager, w io.Writer, levels [][]TuiScriptEntry, deps map[string][]string, concurrentProcs []*Process) (bool, error) {
	if len(pm.Processes) == 0 {
		return false, nil
	}
	pm.SetSink(&stdoutSink{w: w})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	go func() {
		startProcesses(pm, levels, deps, concurrentProcs)
		pm.WaitAllExited(pm.Processes)
		close(done)
	}()

	select {
	case <-done:
	case <-sigCh:
	}

	pm.StopAll()
	if failed := pm.FailedCount(); failed > 0 {
		return true, fmt.Errorf("%d of %d tasks failed", failed, len(pm.Processes))
	}
	return true, nil
}
