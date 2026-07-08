package scripting

import (
	"fmt"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/workspace"
)

// ResolveWorkspaceScript resolves a script by name within a specific workspace.
// workspaceName is matched against members by basename, relative path,
// or package.json name field. scriptName is the script to find.
// Returns the resolved script, the workspace directory as the working directory,
// and the member's effective shell (member miso.json overrides root; empty means
// the caller's fallback applies).
func ResolveWorkspaceScript(workspaceName string, scriptName string, root string, cfg config.Config) (ResolvedScript, string, string, error) {
	members, err := workspace.DiscoverMembers(root, cfg)
	if err != nil {
		return ResolvedScript{}, "", "", fmt.Errorf("discover members: %w", err)
	}

	if len(members) == 0 {
		return ResolvedScript{}, "", "", fmt.Errorf("no workspaces found — is this a monorepo?")
	}

	member, err := workspace.Find(workspaceName, members, root)
	if err != nil {
		return ResolvedScript{}, "", "", err
	}
	workspaceDir := member.Dir

	// member miso.json overlays root for scripts folder and shell
	effective := workspace.EffectiveConfig(cfg, member)

	// determine the scripts folder for this workspace
	scriptsPath := effective.Scripts
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
		return ResolvedScript{}, "", "", fmt.Errorf("discover workspace scripts: %w", err)
	}

	scripts, ok := discovered[scriptName]
	if ok {
		if len(scripts) > 1 {
			var paths []string
			for _, s := range scripts {
				paths = append(paths, s.RelativePath)
			}
			return ResolvedScript{}, "", "", fmt.Errorf("multiple scripts for %q exist in workspace %q: %s",
				scriptName, workspaceName, joinStrings(paths))
		}
		return ResolvedScript{
			Source: ScriptSourceFolder,
			Path:   scripts[0].Path,
		}, workspaceDir, effective.Shell, nil
	}

	// 2. fall back to workspace package.json scripts
	pkgScripts, err := ReadPackageJSONScripts(workspaceDir)
	if err != nil {
		return ResolvedScript{}, "", "", fmt.Errorf("read workspace package.json scripts: %w", err)
	}
	if command, ok := pkgScripts[scriptName]; ok {
		return ResolvedScript{
			Source: ScriptSourcePackageJSON,
			Path:   command,
		}, workspaceDir, effective.Shell, nil
	}

	// not found in either
	return ResolvedScript{Source: ScriptSourceNone}, workspaceDir, effective.Shell, nil
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
