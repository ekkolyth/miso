package tui

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
)

// Launch starts the TUI with the given config, script name, and manager.
// Returns (true, nil) if the TUI ran successfully.
// Returns (false, nil) if the TUI was not applicable (caller should fall through to normal execution).
// Returns (false, err) on error.
func Launch(cfg config.Config, scriptName string, root string, mgr manager.Manager) (bool, error) {
	if !cfg.TuiEnabled() {
		return false, nil
	}

	entries, err := discoverEntries(cfg, scriptName, root)
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return false, nil // fall through to normal execution
	}

	pm := NewProcessManager()

	for _, entry := range entries {
		var cmd string
		var args []string
		dir := entry.WorkspaceDir

		if entry.ScriptSource == "folder" {
			// Run via shell
			shell := cfg.Shell
			if shell == "" {
				shell = "sh"
			}
			cmd = shell
			args = []string{"-e", entry.ScriptPath}
		} else {
			// Run via package manager
			spec := mgr.BuildRun(entry.ScriptName, nil)
			cmd = spec.Command
			args = spec.Args
		}

		pm.Add(entry, cmd, args, dir)
	}

	// Pre-compute dependency levels if this command has dependsOn config.
	// Only main task entries participate in dependency ordering — concurrent
	// entries are started immediately.
	var levels [][]TuiScriptEntry
	var concurrentProcs []*Process
	if cfg.HasDependsOn(scriptName) {
		// Split entries: main task entries vs concurrent entries.
		// Use prefix matching (same as DiscoverTuiScripts) to identify
		// concurrent entries — e.g., concurrent: ["services"] should match
		// entries with ScriptName "services" AND "services:worker".
		var mainEntries []TuiScriptEntry
		concurrentPrefixes := cfg.TaskConcurrent(scriptName)
		for _, e := range entries {
			isConcurrent := false
			for _, prefix := range concurrentPrefixes {
				if e.ScriptName == prefix || strings.HasPrefix(e.ScriptName, prefix+":") || strings.HasPrefix(e.ScriptName, prefix+"/") {
					isConcurrent = true
					break
				}
			}
			if isConcurrent {
				proc := pm.findProc(e.Label)
				if proc != nil {
					concurrentProcs = append(concurrentProcs, proc)
				}
			} else {
				mainEntries = append(mainEntries, e)
			}
		}

		wsInfos := buildWSInfos(mainEntries)
		graph, err := BuildDependencyGraph(wsInfos)
		if err != nil {
			return false, fmt.Errorf("build dependency graph: %w", err)
		}
		var sortErr error
		levels, sortErr = TopoSort(mainEntries, graph)
		if sortErr != nil {
			return false, sortErr
		}
	}

	var model tea.Model
	switch cfg.TuiMode {
	case "tabbed":
		model = NewTabbedModel(pm, scriptName, false)
	case "merged":
		model = NewMergedModel(pm, scriptName, false)
	default:
		return false, fmt.Errorf("unknown tui mode: %s", cfg.TuiMode)
	}

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	pm.SetProgram(p)

	// Catch OS signals — tell bubbletea to quit cleanly so it restores
	// the alt screen. Process cleanup happens after p.Run() returns below.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		p.Quit()
	}()
	defer signal.Stop(sigCh)

	// Start all processes in a goroutine — prog.Send() blocks until the
	// bubbletea event loop is running, so we can't call Start before p.Run().
	go func() {
		// Start concurrent companions immediately
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
		} else {
			for _, proc := range pm.Processes {
				if err := pm.Start(proc); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
				}
			}
		}
	}()

	_, err = p.Run()

	// Dump buffered logs to stdout so they survive the alt-screen restore.
	if !cfg.TuiCleanExit {
		DumpLogs(pm)
	}

	// Always clean up child processes when the TUI exits, regardless of how.
	pm.StopAll()

	// Print failure summary if any processes exited non-zero.
	failed := pm.FailedCount()
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "miso: %d of %d tasks failed\n", failed, len(pm.Processes))
	}

	return true, err
}

func discoverEntries(cfg config.Config, scriptName string, root string) ([]TuiScriptEntry, error) {
	if cfg.IsMonorepo() {
		wsDirs, err := config.LoadWorkspaces(root)
		if err != nil {
			return nil, fmt.Errorf("failed to load workspaces: %w", err)
		}

		var wsInfos []WorkspaceInfo
		for _, dir := range wsDirs {
			name := filepath.Base(dir)
			wsInfos = append(wsInfos, WorkspaceInfo{
				Name: name,
				Dir:  dir,
			})
		}

		entries, err := DiscoverTuiScripts(scriptName, wsInfos, cfg.Scripts)
		if err != nil {
			return nil, err
		}

		// Discover concurrent companion tasks
		for _, concName := range cfg.TaskConcurrent(scriptName) {
			concEntries, err := DiscoverTuiScripts(concName, wsInfos, cfg.Scripts)
			if err != nil {
				return nil, fmt.Errorf("discover concurrent %q: %w", concName, err)
			}
			entries = append(entries, concEntries...)
		}

		return DeduplicateLabels(entries), nil
	}

	// Single repo with concurrent config
	concurrent := cfg.TaskConcurrent(scriptName)
	if len(concurrent) > 0 {
		// Resolve the main task itself
		mainEntries, err := ResolveSingleRepoScripts([]string{scriptName}, root, cfg)
		if err != nil {
			return nil, err
		}

		// Resolve each concurrent task
		concEntries, err := ResolveSingleRepoScripts(concurrent, root, cfg)
		if err != nil {
			return nil, err
		}

		return DeduplicateLabels(append(mainEntries, concEntries...)), nil
	}

	return nil, nil
}

func buildWSInfos(entries []TuiScriptEntry) []WorkspaceInfo {
	seen := make(map[string]bool)
	var infos []WorkspaceInfo
	for _, e := range entries {
		if !seen[e.Label] {
			seen[e.Label] = true
			infos = append(infos, WorkspaceInfo{Name: e.Label, Dir: e.WorkspaceDir})
		}
	}
	return infos
}
