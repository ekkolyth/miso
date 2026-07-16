package tui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/ekkolyth/miso/internal/cli/env"
	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/workspace"
)

// resolves the interpreter for a folder script, returning the command plus
// its full argument list (interpreter args + script path).
func folderSpawn(scriptPath, shell, managerName string) (string, []string, error) {
	interp, interpArgs, err := scripting.ResolveInterpreter(scriptPath, shell, managerName)
	if err != nil {
		return "", nil, err
	}
	return interp, append(interpArgs, scriptPath), nil
}

// filterEntriesByWorkspace keeps only entries whose WorkspaceName is in names.
// A nil/empty names keeps all entries (no scope requested).
func filterEntriesByWorkspace(entries []TuiScriptEntry, names []string) []TuiScriptEntry {
	if len(names) == 0 {
		return entries
	}
	keep := make(map[string]bool, len(names))
	for _, name := range names {
		keep[name] = true
	}
	var out []TuiScriptEntry
	for _, entry := range entries {
		if keep[entry.WorkspaceName] {
			out = append(out, entry)
		}
	}
	return out
}

// Launch starts the TUI with the given config, script name, and manager.
// Returns (true, nil) if the TUI ran successfully.
// Returns (false, nil) if the TUI was not applicable (caller should fall through to normal execution).
// Returns (false, err) on error.
func Launch(cfg config.Config, scriptName string, root string, mgr manager.Manager, filterNames []string) (bool, error) {
	if !cfg.TuiEnabled() {
		return false, nil
	}

	if !hasInteractiveTTY() {
		if len(filterNames) == 0 {
			fmt.Fprintln(os.Stderr, "miso: no interactive terminal — running plain")
		}
		return false, nil
	}

	entries, err := discoverEntries(cfg, scriptName, root)
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return false, nil // fall through to normal execution
	}

	if len(filterNames) > 0 {
		filtered := filterEntriesByWorkspace(entries, filterNames)
		if len(filtered) == 0 {
			return false, fmt.Errorf("no %q script in workspace(s) %s", scriptName, strings.Join(filterNames, ", "))
		}
		entries = filtered
	}

	pm := NewProcessManager()

	managerName := ""
	if mgr != nil {
		managerName = mgr.Name()
	} else if detected, err := manager.DetectManager(root); err == nil {
		managerName = detected
	}

	for _, entry := range entries {
		var cmd string
		var args []string
		dir := entry.WorkspaceDir

		if entry.ScriptSource == "folder" {
			// member's effective shell wins, then root, then sh (see ResolveInterpreter)
			shell := entry.Shell
			if shell == "" {
				shell = cfg.Shell
			}
			spawnCmd, spawnArgs, spawnErr := folderSpawn(entry.ScriptPath, shell, managerName)
			if spawnErr != nil {
				return false, spawnErr
			}
			cmd = spawnCmd
			args = spawnArgs
		} else {
			// Run via package manager
			if mgr == nil {
				return false, fmt.Errorf("script %q requires a package manager but none is configured", entry.ScriptName)
			}
			spec := mgr.BuildRun(entry.ScriptName, nil)
			cmd = spec.Command
			args = spec.Args
		}

		// Build env scoped to this member target
		target := workspace.Target{Kind: workspace.TargetScript, Name: entry.ScriptName}
		if entry.WorkspaceName != "" {
			target = workspace.Target{Kind: workspace.TargetMember, Name: entry.WorkspaceName, Dir: dir}
		}
		processEnv, envErr := env.BuildTargetEnv(root, cfg, target)
		if envErr != nil {
			return false, fmt.Errorf("build env for %s: %w", entry.Label, envErr)
		}

		pm.Add(entry, cmd, args, dir, processEnv)
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

	p := tea.NewProgram(model)
	pm.SetSink(programSink{prog: p})

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
		startProcesses(pm, levels, concurrentProcs)
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

// root wins over members — fan out only when the root doesn't have the script
func discoverEntries(cfg config.Config, scriptName string, root string) ([]TuiScriptEntry, error) {
	var rootResolved []TuiScriptEntry
	var err error
	if cfg.SimpleMode() {
		rootResolved, err = ResolveSingleRepoScriptsFolderOnly([]string{scriptName}, root, cfg)
	} else {
		rootResolved, err = ResolveSingleRepoScripts([]string{scriptName}, root, cfg)
	}
	if err != nil {
		return nil, err
	}
	if len(rootResolved) > 0 {
		return discoverRootScope(cfg, scriptName, root, rootResolved)
	}

	members, err := workspace.DiscoverMembers(root, cfg)
	if err != nil {
		return nil, fmt.Errorf("discover members: %w", err)
	}
	// simple mode does not support monorepo workspace discovery
	if len(members) == 0 || cfg.SimpleMode() {
		return nil, nil
	}
	return discoverMemberFanOut(cfg, scriptName, root, members)
}

// resolves one concurrent entry. A bare name is local scope: the root when
// local is nil, otherwise that member. "@member/script" resolves script inside
// the named member regardless of local scope, via the same name-first tiered
// matching as an explicit CLI @scope (workspace.ResolveScopes).
func resolveConcurrent(cfg config.Config, concName, root string, local *WorkspaceInfo, members []workspace.Member) ([]TuiScriptEntry, error) {
	if strings.HasPrefix(concName, "@") {
		parts := strings.SplitN(strings.TrimPrefix(concName, "@"), "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("concurrent %q: expected @member/script", concName)
		}
		memberName, script := parts[0], parts[1]
		resolved, err := workspace.ResolveScopes([]string{"@" + memberName}, members, root)
		if err != nil {
			return nil, err
		}
		member := resolved[0]
		effective := workspace.EffectiveConfig(cfg, member)
		ws := WorkspaceInfo{Name: member.Name, Dir: member.Dir, ScriptsFolder: effective.Scripts, Shell: effective.Shell}
		return DiscoverTuiScripts(script, []WorkspaceInfo{ws}, cfg.Scripts)
	}
	if local == nil {
		if cfg.SimpleMode() {
			return ResolveSingleRepoScriptsFolderOnly([]string{concName}, root, cfg)
		}
		return ResolveSingleRepoScripts([]string{concName}, root, cfg)
	}
	return DiscoverTuiScripts(concName, []WorkspaceInfo{*local}, cfg.Scripts)
}

// only "@"-prefixed entries need the member list
func concurrentNeedsMembers(concurrent []string) bool {
	for _, name := range concurrent {
		if strings.HasPrefix(name, "@") {
			return true
		}
	}
	return false
}

// adds concurrent companions to an already-resolved main entry
func discoverRootScope(cfg config.Config, scriptName, root string, mainEntries []TuiScriptEntry) ([]TuiScriptEntry, error) {
	concurrent := cfg.TaskConcurrent(scriptName)

	// a broken workspace file must not block a script with no @-ref — only
	// fetch members when one is actually present (bare names resolve at root)
	var members []workspace.Member
	if concurrentNeedsMembers(concurrent) {
		var err error
		members, err = workspace.DiscoverMembers(root, cfg)
		if err != nil {
			return nil, err
		}
	}

	entries := mainEntries
	for _, concName := range concurrent {
		concEntries, err := resolveConcurrent(cfg, concName, root, nil, members)
		if err != nil {
			return nil, err
		}
		entries = append(entries, concEntries...)
	}
	return DeduplicateLabels(entries), nil
}

// one process per member that defines the script, plus each member's own
// concurrent companions resolved within that same member
func discoverMemberFanOut(cfg config.Config, scriptName, root string, members []workspace.Member) ([]TuiScriptEntry, error) {
	var entries []TuiScriptEntry
	for _, member := range members {
		effective := workspace.EffectiveConfig(cfg, member)
		ws := WorkspaceInfo{
			Name:          member.Name,
			Dir:           member.Dir,
			ScriptsFolder: effective.Scripts,
			Shell:         effective.Shell,
		}
		memberEntries, err := DiscoverTuiScripts(scriptName, []WorkspaceInfo{ws}, cfg.Scripts)
		if err != nil {
			return nil, err
		}
		if len(memberEntries) == 0 {
			continue
		}
		entries = append(entries, memberEntries...)

		for _, concName := range effective.TaskConcurrent(scriptName) {
			concEntries, err := resolveConcurrent(cfg, concName, root, &ws, members)
			if err != nil {
				return nil, err
			}
			entries = append(entries, concEntries...)
		}
	}
	return DeduplicateLabels(entries), nil
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
