package scripting

import (
	"fmt"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/config"
)

// ResolveWorkspaceScript resolves a script by name within a specific workspace.
// workspaceName is the short name (e.g. "onboarding"), scriptName is the script
// to find (e.g. "build"). Returns the resolved script and the workspace directory
// that should be used as the working directory when executing.
func ResolveWorkspaceScript(workspaceName string, scriptName string, root string, cfg config.Config) (ResolvedScript, string, error) {
	// load workspaces from package.json
	workspaces, err := config.LoadWorkspaces(root)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("load workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		return ResolvedScript{}, "", fmt.Errorf("no workspaces found in package.json — is this a monorepo?")
	}

	// find the workspace directory by short name
	workspaceDir, ok := config.FindWorkspace(workspaceName, workspaces)
	if !ok {
		return ResolvedScript{}, "", fmt.Errorf("workspace %q not found (available: %s)", workspaceName, joinWorkspaceNames(workspaces))
	}

	// determine the scripts folder for this workspace
	scriptsPath := cfg.Scripts
	if scriptsPath == "" {
		scriptsPath = "./scripts"
	}

	// resolve scripts path relative to workspace dir (not project root)
	if !filepath.IsAbs(scriptsPath) {
		scriptsPath = filepath.Join(workspaceDir, scriptsPath)
	}

	// discover scripts in the workspace scripts folder
	discovered, err := DiscoverScripts(scriptsPath)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("discover workspace scripts: %w", err)
	}

	scripts, ok := discovered[scriptName]
	if !ok {
		return ResolvedScript{Source: ScriptSourceNone}, workspaceDir, nil
	}

	if len(scripts) > 1 {
		var paths []string
		for _, s := range scripts {
			paths = append(paths, s.RelativePath)
		}
		return ResolvedScript{}, "", fmt.Errorf("multiple scripts for %q exist in workspace %q: %s",
			scriptName, workspaceName, joinStrings(paths))
	}

	return ResolvedScript{
		Source: ScriptSourceFolder,
		Path:   scripts[0].Path,
	}, workspaceDir, nil
}

// WorkspaceFromCWD checks whether the current working directory is inside a
// known workspace and returns the workspace directory if so. This enables
// automatic scoping: running "miso build" from inside apps/onboarding will
// resolve against that workspace's scripts folder.
func WorkspaceFromCWD(cwd string, workspaces []string) (string, bool) {
	for _, ws := range workspaces {
		absWs, err := filepath.Abs(ws)
		if err != nil {
			continue
		}
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			continue
		}
		// check if cwd is the workspace dir or nested inside it
		rel, err := filepath.Rel(absWs, absCwd)
		if err != nil {
			continue
		}
		// rel will not start with ".." if cwd is inside ws
		if rel == "." || (len(rel) > 0 && rel[0] != '.') {
			return absWs, true
		}
	}
	return "", false
}

// joinWorkspaceNames returns a comma-separated list of workspace short names.
func joinWorkspaceNames(workspaces []string) string {
	names := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		names = append(names, filepath.Base(ws))
	}
	return joinStrings(names)
}

// joinStrings joins a slice of strings with ", ".
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
