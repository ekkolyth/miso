package scripting

import (
	"fmt"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/config"
)

// ResolveWorkspaceScript resolves a script by name within a specific workspace.
// workspaceName is matched against workspace paths by basename, relative path,
// or package.json name field. scriptName is the script to find.
// Returns the resolved script and the workspace directory as the working directory.
func ResolveWorkspaceScript(workspaceName string, scriptName string, root string, cfg config.Config) (ResolvedScript, string, error) {
	// load workspaces from package.json
	workspaces, err := config.LoadWorkspaces(root)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("load workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		return ResolvedScript{}, "", fmt.Errorf("no workspaces found in package.json — is this a monorepo?")
	}

	// find the workspace directory — matches by basename, relative path, or package.json name
	workspaceDir, err := config.FindWorkspace(workspaceName, workspaces, root)
	if err != nil {
		return ResolvedScript{}, "", err
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

	// 1. check workspace scripts/ folder
	discovered, err := DiscoverScripts(scriptsPath)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("discover workspace scripts: %w", err)
	}

	scripts, ok := discovered[scriptName]
	if ok {
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

	// 2. fall back to workspace package.json scripts
	pkgScripts, err := ReadPackageJSONScripts(workspaceDir)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("read workspace package.json scripts: %w", err)
	}
	if command, ok := pkgScripts[scriptName]; ok {
		return ResolvedScript{
			Source: ScriptSourcePackageJSON,
			Path:   command,
		}, workspaceDir, nil
	}

	// not found in either
	return ResolvedScript{Source: ScriptSourceNone}, workspaceDir, nil
}

// WorkspaceFromCWD checks whether the current working directory is inside a
// known workspace and returns the workspace directory if so.
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
		rel, err := filepath.Rel(absWs, absCwd)
		if err != nil {
			continue
		}
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
