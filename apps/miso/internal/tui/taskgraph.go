package tui

import (
	"fmt"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/workspace"
)

// one node per script per workspace, so a shared dependency runs once and
// survives label rewriting
func entryKey(e TuiScriptEntry) string {
	return e.WorkspaceDir + "\x00" + e.ScriptName
}

type taskGraph struct {
	cfg      config.Config
	root     string
	members  []workspace.Member
	byName   map[string]workspace.Member
	infos    map[string]WorkspaceInfo
	effects  map[string]config.Config
	upstream map[string][]string
}

// adds every task the run's dependsOn lists name, transitively, and returns
// the label-keyed graph TopoSort orders the main entries by. The graph is nil
// when no task in the run declares dependsOn
func expandDependencies(cfg config.Config, root string, scriptName string, entries []TuiScriptEntry) ([]TuiScriptEntry, map[string][]string, error) {
	g := &taskGraph{
		cfg:     cfg,
		root:    root,
		byName:  make(map[string]workspace.Member),
		infos:   make(map[string]WorkspaceInfo),
		effects: make(map[string]config.Config),
	}
	if !cfg.SimpleMode() {
		members, err := workspace.DiscoverMembersCached(root, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("discover members: %w", err)
		}
		g.members = members
		for _, member := range members {
			g.byName[member.Name] = member
		}
	}

	out := append([]TuiScriptEntry(nil), entries...)
	index := make(map[string]int)
	companions := make(map[string]bool)
	var queue []int
	for i, entry := range out {
		key := entryKey(entry)
		if entry.IsConcurrent {
			companions[key] = true
			continue
		}
		index[key] = i
		queue = append(queue, i)
	}

	edges := make(map[string][]string)
	declared := false
	added := false
	addDep := func(dep resolvedDep) (string, error) {
		depKey := entryKey(dep.entry)
		if companions[depKey] {
			return "", fmt.Errorf("repo.tasks.%s: %q is listed in both concurrent and dependsOn", dep.declaredBy, dep.ref)
		}
		if _, ok := index[depKey]; !ok {
			entry := dep.entry
			entry.Args = nil
			entry.IsConcurrent = false
			index[depKey] = len(out)
			queue = append(queue, len(out))
			out = append(out, entry)
			added = true
		}
		return depKey, nil
	}

	// a task with no script anywhere still runs what it depends on
	if !hasMainEntry(out, scriptName) {
		deps, hasRefs, err := g.dependenciesOf(TuiScriptEntry{ScriptName: scriptName, WorkspaceDir: root})
		if err != nil {
			return nil, nil, err
		}
		declared = hasRefs
		for _, dep := range deps {
			if _, err := addDep(dep); err != nil {
				return nil, nil, err
			}
		}
	}

	for len(queue) > 0 {
		node := out[queue[0]]
		queue = queue[1:]

		deps, hasRefs, err := g.dependenciesOf(node)
		if err != nil {
			return nil, nil, err
		}
		declared = declared || hasRefs
		nodeKey := entryKey(node)
		edges[nodeKey] = nil
		for _, dep := range deps {
			depKey, err := addDep(dep)
			if err != nil {
				return nil, nil, err
			}
			edges[nodeKey] = append(edges[nodeKey], depKey)
		}
	}

	if !declared {
		return entries, nil, nil
	}
	if added {
		// labels were deduplicated against the smaller list; redo it over the
		// full run so a member running two scripts gets distinct labels
		for i := range out {
			out[i].Label = out[i].WorkspaceName
			if out[i].Label == "" {
				out[i].Label = out[i].ScriptName
			}
		}
		out = DeduplicateLabels(out)
	}

	graph := make(map[string][]string, len(edges))
	for nodeKey, depKeys := range edges {
		label := out[index[nodeKey]].Label
		graph[label] = nil
		for _, depKey := range depKeys {
			graph[label] = append(graph[label], out[index[depKey]].Label)
		}
	}
	return out, graph, nil
}

func hasMainEntry(entries []TuiScriptEntry, scriptName string) bool {
	for _, entry := range entries {
		if !entry.IsConcurrent && entry.ScriptName == scriptName {
			return true
		}
	}
	return false
}

type resolvedDep struct {
	entry TuiScriptEntry
	ref   string
	// the task whose dependsOn lists ref, which differs from the node when
	// ref came through a scriptless task
	declaredBy string
}

// a bare name runs in the node's own workspace — the member it belongs to, or
// the root — whichever list declared it, and a workspace without that script
// is skipped; "#name" and "@member/script" resolve as in concurrent, and "^name"
// runs in the node's upstream members. A named task with no script but its own
// dependsOn stands in for what it depends on
func (g *taskGraph) dependenciesOf(node TuiScriptEntry) ([]resolvedDep, bool, error) {
	return g.collectDependencies(node, map[string]bool{})
}

// visiting holds the scriptless tasks on the current path, which TopoSort
// never sees, so a cycle through them would otherwise recurse forever
func (g *taskGraph) collectDependencies(node TuiScriptEntry, visiting map[string]bool) ([]resolvedDep, bool, error) {
	member := g.memberOf(node)

	var deps []resolvedDep
	hasRefs := false
	resolveList := func(refs []string) error {
		for _, ref := range refs {
			hasRefs = true
			entries, scriptless, err := g.resolveRef(ref, member)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				deps = append(deps, resolvedDep{entry: entry, ref: ref, declaredBy: node.ScriptName})
			}
			for _, task := range scriptless {
				key := entryKey(task)
				if visiting[key] {
					return fmt.Errorf("circular dependency detected")
				}
				visiting[key] = true
				inner, _, err := g.collectDependencies(task, visiting)
				delete(visiting, key)
				if err != nil {
					return err
				}
				deps = append(deps, inner...)
			}
		}
		return nil
	}

	if err := resolveList(g.cfg.Tasks[node.ScriptName].DependsOn); err != nil {
		return nil, false, err
	}
	if member != nil {
		if err := resolveList(g.effective(*member).Tasks[node.ScriptName].DependsOn); err != nil {
			return nil, false, err
		}
	}
	return deps, hasRefs, nil
}

func (g *taskGraph) memberOf(node TuiScriptEntry) *workspace.Member {
	if node.WorkspaceName == "" {
		return nil
	}
	if m, ok := g.byName[node.WorkspaceName]; ok {
		return &m
	}
	return nil
}

// a node for a task with no script, in the given workspace (nil is the root)
func (g *taskGraph) taskNode(name string, member *workspace.Member) TuiScriptEntry {
	if member == nil {
		return TuiScriptEntry{ScriptName: name, WorkspaceDir: g.root}
	}
	return TuiScriptEntry{ScriptName: name, WorkspaceName: member.Name, WorkspaceDir: member.Dir}
}

func (g *taskGraph) declaresDependsOn(node TuiScriptEntry) bool {
	if len(g.cfg.Tasks[node.ScriptName].DependsOn) > 0 {
		return true
	}
	member := g.memberOf(node)
	return member != nil && len(g.effective(*member).Tasks[node.ScriptName].DependsOn) > 0
}

// scripts the ref runs, plus the scriptless tasks it names that declare
// dependsOn of their own
func (g *taskGraph) resolveRef(ref string, member *workspace.Member) ([]TuiScriptEntry, []TuiScriptEntry, error) {
	if strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "@") {
		entries, err := resolveConcurrent(g.cfg, ref, g.root, nil, g.members, "dependsOn")
		if err == nil {
			return entries, nil, nil
		}
		if task, ok := g.pinnedTask(ref); ok && g.declaresDependsOn(task) {
			return nil, []TuiScriptEntry{task}, nil
		}
		return nil, nil, err
	}
	name, isCaret := strings.CutPrefix(ref, "^")
	if !isCaret {
		entries, err := g.resolveInWorkspace(name, member)
		if err != nil || len(entries) > 0 {
			return entries, nil, err
		}
		return nil, g.scriptlessIn(name, member), nil
	}
	if member == nil {
		return nil, nil, nil
	}
	upstream, err := g.upstreamOf(member.Name)
	if err != nil {
		return nil, nil, err
	}
	var entries, scriptless []TuiScriptEntry
	for _, upName := range upstream {
		upMember := g.byName[upName]
		// an upstream member without the script is skipped, not an error
		found, err := DiscoverTuiScripts(name, []WorkspaceInfo{g.memberInfo(upMember)}, g.cfg.Scripts)
		if err != nil {
			return nil, nil, err
		}
		if len(found) == 0 {
			scriptless = append(scriptless, g.scriptlessIn(name, &upMember)...)
		}
		entries = append(entries, found...)
	}
	return entries, scriptless, nil
}

func (g *taskGraph) scriptlessIn(name string, member *workspace.Member) []TuiScriptEntry {
	task := g.taskNode(name, member)
	if !g.declaresDependsOn(task) {
		return nil
	}
	return []TuiScriptEntry{task}
}

// the workspace a "#name" or "@member/name" ref pins, resolved as concurrent
// would; false when the ref names no workspace
func (g *taskGraph) pinnedTask(ref string) (TuiScriptEntry, bool) {
	if name, ok := strings.CutPrefix(ref, "#"); ok {
		return g.taskNode(name, nil), true
	}
	parts := strings.SplitN(strings.TrimPrefix(ref, "@"), "/", 2)
	if len(parts) != 2 {
		return TuiScriptEntry{}, false
	}
	resolved, err := workspace.ResolveScopes([]string{"@" + parts[0]}, g.members, g.root)
	if err != nil || len(resolved) == 0 {
		return TuiScriptEntry{}, false
	}
	return g.taskNode(parts[1], &resolved[0]), true
}

// empty when the workspace has no script by that name
func (g *taskGraph) resolveInWorkspace(name string, member *workspace.Member) ([]TuiScriptEntry, error) {
	if member != nil {
		return DiscoverTuiScripts(name, []WorkspaceInfo{g.memberInfo(*member)}, g.cfg.Scripts)
	}
	if g.cfg.SimpleMode() {
		return ResolveSingleRepoScriptsFolderOnly([]string{name}, g.root, g.cfg)
	}
	return ResolveSingleRepoScripts([]string{name}, g.root, g.cfg)
}

// built over every member, not only those in the run, so "^name" can reach a
// member the fan-out never touched
func (g *taskGraph) upstreamOf(memberName string) ([]string, error) {
	if g.upstream == nil {
		infos := make([]WorkspaceInfo, 0, len(g.members))
		for _, member := range g.members {
			infos = append(infos, g.memberInfo(member))
		}
		graph, err := BuildDependencyGraph(infos)
		if err != nil {
			return nil, fmt.Errorf("build dependency graph: %w", err)
		}
		g.upstream = graph
	}
	return g.upstream[memberName], nil
}

// memoized — EffectiveConfig reads the member's miso.json each call
func (g *taskGraph) effective(member workspace.Member) config.Config {
	if cfg, ok := g.effects[member.Name]; ok {
		return cfg
	}
	cfg := workspace.EffectiveConfig(g.cfg, member)
	g.effects[member.Name] = cfg
	return cfg
}

func (g *taskGraph) memberInfo(member workspace.Member) WorkspaceInfo {
	if info, ok := g.infos[member.Name]; ok {
		return info
	}
	effective := g.effective(member)
	info := WorkspaceInfo{Name: member.Name, Dir: member.Dir, ScriptsFolder: effective.Scripts, Shell: effective.Shell}
	g.infos[member.Name] = info
	return info
}
