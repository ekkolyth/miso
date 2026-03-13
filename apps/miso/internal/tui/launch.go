package tui

import (
	"fmt"
	"os"
	"path/filepath"

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

	var model tea.Model
	switch cfg.Tui {
	case "tabbed":
		model = NewTabbedModel(pm, scriptName)
	case "merged":
		model = NewMergedModel(pm, scriptName)
	default:
		return false, fmt.Errorf("unknown tui mode: %s", cfg.Tui)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	pm.SetProgram(p)

	// Start all processes
	for _, proc := range pm.Processes {
		if err := pm.Start(proc); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
		}
	}

	_, err = p.Run()
	return true, err
}

func discoverEntries(cfg config.Config, scriptName string, root string) ([]TuiScriptEntry, error) {
	if cfg.IsMono() && len(cfg.Multi) > 0 {
		fmt.Fprintf(os.Stderr, "warning: 'multi' config is ignored in monorepo mode — workspace auto-discovery is used instead\n")
	}

	if cfg.IsMono() {
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

		return DiscoverTuiScripts(scriptName, wsInfos, cfg.Scripts)
	}

	// Single repo with multi config
	if scripts, ok := cfg.Multi[scriptName]; ok {
		return DiscoverMultiScripts(scripts, root, cfg)
	}

	return nil, nil
}
