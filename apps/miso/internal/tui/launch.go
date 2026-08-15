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

// renders the resolved script + concurrent companions as bubbletea chrome.
// Every call site gates on SelectRenderer first, so the TTY/mode checks live
// there. Returns (true, nil) on a clean run, (true, err) when the program
// errored or a child exited non-zero, (false, nil) when nothing resolved so
// the caller falls through to normal execution
func Launch(cfg config.Config, scriptName string, root string, mgr manager.Manager, filterNames []string, scriptArgs []string) (bool, error) {
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, scriptName, root, mgr, filterNames, scriptArgs)
	if err != nil {
		return false, err
	}
	if !ran {
		return false, nil
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

	if err != nil {
		return true, err
	}
	if failed := pm.FailedCount(); failed > 0 {
		return true, fmt.Errorf("%d of %d tasks failed", failed, len(pm.Processes))
	}
	return true, nil
}

// the plain sibling of Launch: shares buildRun's discovery + process setup,
// then streams "[label] line" to stdout instead of rendering bubbletea chrome.
// Same (ran, err) contract as Launch
func LaunchPlain(cfg config.Config, scriptName string, root string, mgr manager.Manager, filterNames []string, scriptArgs []string) (bool, error) {
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, scriptName, root, mgr, filterNames, scriptArgs)
	if err != nil {
		return false, err
	}
	if !ran {
		return false, nil
	}
	markPlain(pm.Processes)
	return RunPlain(pm, os.Stdout, levels, concurrentProcs)
}

// markPlain switches every process to pipe-mode (no pty) and adds FORCE_COLOR,
// so a tool detects a non-tty — emitting plain line output instead of cursor
// redraws — yet still prints color. NO_COLOR still wins. A nil Environ is seeded
// from os.Environ so the augmented env inherits the ambient shell instead of
// collapsing to FORCE_COLOR alone. Chrome (Launch) skips this and keeps the pty.
func markPlain(procs []*Process) {
	for _, proc := range procs {
		proc.NoPTY = true
		base := proc.Environ
		if base == nil {
			base = os.Environ()
		}
		proc.Environ = forceColorEnv(base)
	}
}

// shared discovery + process setup for both the chrome (Launch) and plain
// (LaunchPlain) paths: resolves entries, applies workspace filters, adds a
// process per entry with scoped env, and pre-computes dependency levels. ran is
// false when no entries resolve — the caller falls through to normal execution
func buildRun(cfg config.Config, scriptName string, root string, mgr manager.Manager, filterNames []string, scriptArgs []string) (*ProcessManager, [][]TuiScriptEntry, []*Process, bool, error) {
	entries, err := discoverEntries(cfg, scriptName, root, scriptArgs)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if len(entries) == 0 {
		return nil, nil, nil, false, nil // fall through to normal execution
	}

	if len(filterNames) > 0 {
		filtered := filterEntriesByWorkspace(entries, filterNames)
		if len(filtered) == 0 {
			return nil, nil, nil, false, fmt.Errorf("no %q script in workspace(s) %s", scriptName, strings.Join(filterNames, ", "))
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
				return nil, nil, nil, false, spawnErr
			}
			cmd = spawnCmd
			args = append(spawnArgs, entry.Args...)
		} else {
			// Run via package manager
			if mgr == nil {
				return nil, nil, nil, false, fmt.Errorf("script %q requires a package manager but none is configured", entry.ScriptName)
			}
			spec := mgr.BuildRun(entry.ScriptName, entry.Args)
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
			return nil, nil, nil, false, fmt.Errorf("build env for %s: %w", entry.Label, envErr)
		}

		pm.Add(entry, cmd, args, dir, processEnv)
	}

	// Pre-compute dependency levels when the command declares dependsOn. Only
	// main entries participate in ordering; concurrent companions — marked at
	// discovery — start immediately.
	var levels [][]TuiScriptEntry
	var concurrentProcs []*Process
	if cfg.HasDependsOn(scriptName) {
		mainEntries, concurrentEntries := classifyEntries(entries)
		for _, e := range concurrentEntries {
			if proc := pm.findProc(e.Label); proc != nil {
				concurrentProcs = append(concurrentProcs, proc)
			}
		}

		wsInfos := buildWSInfos(mainEntries)
		graph, err := BuildDependencyGraph(wsInfos)
		if err != nil {
			return nil, nil, nil, false, fmt.Errorf("build dependency graph: %w", err)
		}
		var sortErr error
		levels, sortErr = TopoSort(mainEntries, graph)
		if sortErr != nil {
			return nil, nil, nil, false, sortErr
		}
	}

	return pm, levels, concurrentProcs, true, nil
}

// members ≥ 1: fan out over members (root is the orchestrator, never a
// fan-out member); root script is the body only when no member defines the
// name. Root task companions attach exactly once either way. Zero members /
// simple mode: root script is the body, as always.
func discoverEntries(cfg config.Config, scriptName string, root string, scriptArgs []string) ([]TuiScriptEntry, error) {
	var members []workspace.Member
	if !cfg.SimpleMode() {
		var err error
		members, err = workspace.DiscoverMembersCached(root, cfg)
		if err != nil {
			return nil, fmt.Errorf("discover members: %w", err)
		}
	}

	if len(members) == 0 {
		return discoverSingleRepo(cfg, scriptName, root, scriptArgs)
	}

	fanOut, err := discoverMemberFanOut(cfg, scriptName, root, members)
	if err != nil {
		return nil, err
	}
	// args attach only to a single unambiguous main entry — fan-out is
	// inherently one-or-more, so scriptArgs never reaches it
	if len(fanOut) > 0 {
		return appendRootCompanions(cfg, scriptName, root, fanOut, members)
	}

	// no member defines the name — root script as the body (single pane)
	rootResolved, err := ResolveSingleRepoScripts([]string{scriptName}, root, cfg)
	if err != nil {
		return nil, err
	}
	if rootScopeEmpty(cfg, scriptName, rootResolved) {
		return nil, nil // nothing anywhere — fall through to passthrough
	}
	return discoverRootScope(cfg, scriptName, root, rootResolved, scriptArgs)
}

// true when neither a direct script nor a concurrent companion resolves at
// root scope for scriptName
func rootScopeEmpty(cfg config.Config, scriptName string, rootResolved []TuiScriptEntry) bool {
	return len(rootResolved) == 0 && len(cfg.TaskConcurrent(scriptName)) == 0
}

// zero-member path: root resolves directly and root companions attach
func discoverSingleRepo(cfg config.Config, scriptName, root string, scriptArgs []string) ([]TuiScriptEntry, error) {
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
	if rootScopeEmpty(cfg, scriptName, rootResolved) {
		return nil, nil
	}
	return discoverRootScope(cfg, scriptName, root, rootResolved, scriptArgs)
}

// root task companions, resolved at root scope, appended exactly once per run
func appendRootCompanions(cfg config.Config, scriptName, root string, entries []TuiScriptEntry, members []workspace.Member) ([]TuiScriptEntry, error) {
	for _, concName := range cfg.TaskConcurrent(scriptName) {
		concEntries, err := resolveConcurrent(cfg, concName, root, nil, members)
		if err != nil {
			return nil, err
		}
		markConcurrent(concEntries)
		entries = append(entries, concEntries...)
	}
	return DeduplicateLabels(entries), nil
}

// resolves one concurrent entry and errors if it resolves to zero entries.
// A bare name is local scope: the root when local is nil, otherwise that
// member. "#name" pins resolution to the root regardless of declaring scope.
// "@member/script" resolves script inside the named member regardless of
// local scope, via the same name-first tiered matching as an explicit CLI
// @scope (workspace.ResolveScopes).
func resolveConcurrent(cfg config.Config, concName, root string, local *WorkspaceInfo, members []workspace.Member) ([]TuiScriptEntry, error) {
	var entries []TuiScriptEntry
	var err error
	var scopeDesc string

	switch {
	case strings.HasPrefix(concName, "#"):
		rootName := strings.TrimPrefix(concName, "#")
		scopeDesc = fmt.Sprintf("root (scripts folder %q or root package.json)", cfg.Scripts)
		if cfg.SimpleMode() {
			entries, err = ResolveSingleRepoScriptsFolderOnly([]string{rootName}, root, cfg)
		} else {
			entries, err = ResolveSingleRepoScripts([]string{rootName}, root, cfg)
		}
	case strings.HasPrefix(concName, "@"):
		parts := strings.SplitN(strings.TrimPrefix(concName, "@"), "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("concurrent %q: expected @member/script", concName)
		}
		memberName, script := parts[0], parts[1]
		resolved, scopeErr := workspace.ResolveScopes([]string{"@" + memberName}, members, root)
		if scopeErr != nil {
			return nil, scopeErr
		}
		member := resolved[0]
		effective := workspace.EffectiveConfig(cfg, member)
		memberInfo := WorkspaceInfo{Name: member.Name, Dir: member.Dir, ScriptsFolder: effective.Scripts, Shell: effective.Shell}
		scopeDesc = fmt.Sprintf("member %q", member.Name)
		entries, err = DiscoverTuiScripts(script, []WorkspaceInfo{memberInfo}, cfg.Scripts)
	case local == nil:
		scopeDesc = fmt.Sprintf("root (scripts folder %q or root package.json)", cfg.Scripts)
		if cfg.SimpleMode() {
			entries, err = ResolveSingleRepoScriptsFolderOnly([]string{concName}, root, cfg)
		} else {
			entries, err = ResolveSingleRepoScripts([]string{concName}, root, cfg)
		}
	default:
		scopeDesc = fmt.Sprintf("member %q", local.Name)
		entries, err = DiscoverTuiScripts(concName, []WorkspaceInfo{*local}, cfg.Scripts)
	}

	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("concurrent %q: no script found at %s", concName, scopeDesc)
	}
	return entries, nil
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

// adds concurrent companions to an already-resolved main entry. mainEntries is
// always length 1 here (ResolveSingleRepoScripts* resolves one script name) —
// that single entry gets scriptArgs; concurrent companions never do
func discoverRootScope(cfg config.Config, scriptName, root string, mainEntries []TuiScriptEntry, scriptArgs []string) ([]TuiScriptEntry, error) {
	if len(mainEntries) == 1 {
		mainEntries[0].Args = scriptArgs
	}

	concurrent := cfg.TaskConcurrent(scriptName)

	// members may already be cached from discoverEntries's own fan-out fetch;
	// this guard only spares the zero-member / simple-mode path a fetch when
	// no @-ref is present (bare names resolve at root)
	var members []workspace.Member
	if concurrentNeedsMembers(concurrent) {
		var err error
		members, err = workspace.DiscoverMembersCached(root, cfg)
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
		markConcurrent(concEntries)
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
			markConcurrent(concEntries)
			entries = append(entries, concEntries...)
		}
	}
	// dedup happens once, in appendRootCompanions, over the final merged
	// list whenever this fan-out is non-empty; an empty result needs no dedup
	return entries, nil
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

// flags every entry as a concurrent companion, stamped at discovery so the
// buildRun split classifies structurally instead of re-matching the root
// concurrent list (which misses member-injected companions)
func markConcurrent(entries []TuiScriptEntry) {
	for i := range entries {
		entries[i].IsConcurrent = true
	}
}

// partitions entries into dependency-ordered main entries and concurrent
// companions, keyed on the IsConcurrent marker — the marker is what lets a
// member fan-out companion escape the dependsOn topo sort it would deadlock
func classifyEntries(entries []TuiScriptEntry) (main, concurrent []TuiScriptEntry) {
	for _, e := range entries {
		if e.IsConcurrent {
			concurrent = append(concurrent, e)
		} else {
			main = append(main, e)
		}
	}
	return main, concurrent
}
