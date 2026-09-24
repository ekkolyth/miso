package tui

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/config"
)

type WorkspaceInfo struct {
	Name string
	Dir  string
	// per-member scripts folder; empty falls back to the default passed to DiscoverTuiScripts
	ScriptsFolder string
	// per-member effective shell; empty falls back at spawn time
	Shell string
}

type TuiScriptEntry struct {
	Label         string
	WorkspaceName string
	ScriptName    string
	WorkspaceDir  string
	ScriptSource  string // "folder" or "packagejson"
	ScriptPath    string
	// effective shell for spawning folder scripts; empty falls back to cfg.Shell then sh
	Shell string
	// user-supplied invocation args; set only on the single main entry when the
	// run resolves unambiguously — nil for concurrent companions and fan-out members
	Args []string
	// concurrent companion — started immediately, exempt from dependsOn
	// ordering; stamped at discovery so the buildRun split never sorts a
	// member-injected companion into the dependency levels
	IsConcurrent bool
}

// DiscoverTuiScripts finds the script named exactly command in each of the
// provided workspaces — "dev" never matches "dev:worker" or "dev/worker".
// scriptsFolder is the relative path to the scripts directory (e.g. "./scripts").
// It is used in monorepo mode.
//
// A name defined in both the scripts folder and package.json within the same
// workspace is ambiguous and returns scripting.ErrAmbiguousScript.
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
	// member's own scripts folder wins over the passed default
	if ws.ScriptsFolder != "" {
		scriptsFolder = ws.ScriptsFolder
	}

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

	infos, inFolder := folderScripts[command]
	_, inPkg := pkgScripts[command]
	if inFolder && inPkg {
		return nil, fmt.Errorf("%w: %q in %s is defined in both scripts/ and package.json — rename one",
			scripting.ErrAmbiguousScript, command, ws.Name)
	}
	if !inFolder && !inPkg {
		return nil, nil
	}

	entry := TuiScriptEntry{
		Label:         ws.Name,
		WorkspaceName: ws.Name,
		ScriptName:    command,
		WorkspaceDir:  ws.Dir,
		ScriptSource:  "packagejson",
		Shell:         ws.Shell,
	}
	if inFolder {
		entry.ScriptSource = "folder"
		if len(infos) > 0 {
			entry.ScriptPath = infos[0].Path
		}
	}
	return []TuiScriptEntry{entry}, nil
}

// DeduplicateLabels ensures all labels in the merged entry list are unique.
// For any label that appears more than once, it rewrites to "label/scriptName".
func DeduplicateLabels(entries []TuiScriptEntry) []TuiScriptEntry {
	// Count label occurrences
	counts := make(map[string]int)
	for _, e := range entries {
		counts[e.Label]++
	}

	// Rewrite duplicates
	for i := range entries {
		if counts[entries[i].Label] > 1 {
			entries[i].Label = entries[i].Label + "/" + entries[i].ScriptName
		}
	}

	return entries
}

// ResolveSingleRepoScriptsFolderOnly is like ResolveSingleRepoScripts but only
// checks the scripts folder, ignoring package.json. Used in simple mode.
func ResolveSingleRepoScriptsFolderOnly(scripts []string, root string, cfg config.Config) ([]TuiScriptEntry, error) {
	var entries []TuiScriptEntry

	for _, name := range scripts {
		resolved, err := scripting.ResolveScriptFolderOnly(name, root, cfg)
		if err != nil {
			return nil, err
		}
		if resolved.Source == scripting.ScriptSourceNone {
			continue
		}

		entries = append(entries, TuiScriptEntry{
			Label:        name,
			ScriptName:   name,
			WorkspaceDir: root,
			ScriptSource: "folder",
			ScriptPath:   resolved.Path,
		})
	}

	return entries, nil
}

// ResolveSingleRepoScripts resolves a list of script names against the project
// root for single-repo concurrent discovery. Scripts that cannot be found are
// silently skipped. Labels are the script names.
func ResolveSingleRepoScripts(scripts []string, root string, cfg config.Config) ([]TuiScriptEntry, error) {
	var entries []TuiScriptEntry

	for _, name := range scripts {
		resolved, err := scripting.ResolveScript(name, root, cfg)
		if err != nil {
			return nil, err
		}
		if resolved.Source == scripting.ScriptSourceNone {
			continue // silently skip unresolvable scripts
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
