package tui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/config"
)

// WorkspaceInfo holds the name and directory of a workspace.
type WorkspaceInfo struct {
	Name string
	Dir  string
}

// TuiScriptEntry represents a single script that can be launched from the TUI.
type TuiScriptEntry struct {
	Label        string
	ScriptName   string
	WorkspaceDir string
	ScriptSource string // "folder" or "packagejson"
	ScriptPath   string
}

// DiscoverTuiScripts finds all scripts matching the given command prefix across
// the provided workspaces. scriptsFolder is the relative path to the scripts
// directory (e.g. "./scripts"). It is used in monorepo mode.
//
// Prefix matching: command "dev" matches "dev", "dev:worker", "dev/worker", etc.
// Scripts folder takes precedence over package.json for the same script name.
// Labels: single match per workspace → workspace name; multiple matches → "workspace:scriptName".
// Results are sorted alphabetically by label.
func DiscoverTuiScripts(command string, workspaces []WorkspaceInfo, scriptsFolder string) ([]TuiScriptEntry, error) {
	if scriptsFolder == "" {
		scriptsFolder = "./scripts"
	}

	var entries []TuiScriptEntry

	for _, ws := range workspaces {
		wsEntries, err := discoverWorkspaceScripts(command, ws, scriptsFolder)
		if err != nil {
			return nil, err
		}
		entries = append(entries, wsEntries...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Label < entries[j].Label
	})

	return entries, nil
}

// discoverWorkspaceScripts finds matching scripts within a single workspace.
func discoverWorkspaceScripts(command string, ws WorkspaceInfo, scriptsFolder string) ([]TuiScriptEntry, error) {
	// resolve scripts path
	scriptsPath := scriptsFolder
	if !filepath.IsAbs(scriptsPath) {
		scriptsPath = filepath.Join(ws.Dir, scriptsPath)
	}

	// discover folder scripts
	folderScripts, err := scripting.DiscoverScripts(scriptsPath)
	if err != nil {
		return nil, err
	}

	// discover package.json scripts
	pkgScripts, err := scripting.ReadPackageJSONScripts(ws.Dir)
	if err != nil {
		return nil, err
	}

	// collect matching script names; folder takes precedence over package.json
	type match struct {
		name   string
		source string
		path   string
	}

	seen := make(map[string]bool)
	var matches []match

	// folder scripts: prefix match
	for name, infos := range folderScripts {
		if matchesPrefix(command, name) {
			seen[name] = true
			path := ""
			if len(infos) > 0 {
				path = infos[0].Path
			}
			matches = append(matches, match{name: name, source: "folder", path: path})
		}
	}

	// package.json scripts: prefix match, only if not already found in folder
	for name := range pkgScripts {
		if !seen[name] && matchesPrefix(command, name) {
			matches = append(matches, match{name: name, source: "packagejson", path: ""})
		}
	}

	if len(matches) == 0 {
		return nil, nil
	}

	// sort matches for deterministic label assignment
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].name < matches[j].name
	})

	var entries []TuiScriptEntry
	for _, m := range matches {
		var label string
		if len(matches) == 1 {
			label = ws.Name
		} else {
			label = ws.Name + ":" + m.name
		}
		entries = append(entries, TuiScriptEntry{
			Label:        label,
			ScriptName:   m.name,
			WorkspaceDir: ws.Dir,
			ScriptSource: m.source,
			ScriptPath:   m.path,
		})
	}

	return entries, nil
}

// matchesPrefix reports whether scriptName equals command or starts with command
// followed by ":" or "/".
func matchesPrefix(command, scriptName string) bool {
	if scriptName == command {
		return true
	}
	return strings.HasPrefix(scriptName, command+":") || strings.HasPrefix(scriptName, command+"/")
}

// DiscoverMultiScripts resolves each script name from the multi array against
// root using the provided config. It is used in single-repo mode with "multi"
// config. Labels are the script names from the scripts slice.
func DiscoverMultiScripts(scripts []string, root string, cfg config.Config) ([]TuiScriptEntry, error) {
	var entries []TuiScriptEntry

	for _, name := range scripts {
		resolved, err := scripting.ResolveScript(name, root, cfg)
		if err != nil {
			return nil, err
		}

		source := ""
		switch resolved.Source {
		case scripting.ScriptSourceFolder:
			source = "folder"
		case scripting.ScriptSourcePackageJSON:
			source = "packagejson"
		}

		entries = append(entries, TuiScriptEntry{
			Label:        name,
			ScriptName:   name,
			WorkspaceDir: root,
			ScriptSource: source,
			ScriptPath:   resolved.Path,
		})
	}

	return entries, nil
}
