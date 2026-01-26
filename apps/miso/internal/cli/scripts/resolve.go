package scripts

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
)

// script source type
type ScriptSource int

const (
	ScriptSourceNone ScriptSource = iota
	ScriptSourceFolder
	ScriptSourcePackageJSON
)

// resolved script info
type ResolvedScript struct {
	Source ScriptSource
	Path   string // file path for folder scripts, command for package.json scripts
}

// resolve script by name, checking folder then package.json
// supports path-based commands (e.g., "publish/patch") and index.sh resolution
// supports explicit extension (e.g., "jump.sh")
func ResolveScript(name string, root string, cfg config.Config) (ResolvedScript, error) {
	scriptsPath := cfg.Scripts
	if scriptsPath == "" {
		scriptsPath = "./scripts"
	}

	// resolve scripts path relative to root
	if !filepath.IsAbs(scriptsPath) {
		scriptsPath = filepath.Join(root, scriptsPath)
	}

	// check scripts folder first
	discovered, err := DiscoverScripts(scriptsPath)
	if err != nil {
		return ResolvedScript{}, fmt.Errorf("discover scripts: %w", err)
	}

	// normalize name to use forward slashes for path matching
	name = filepath.ToSlash(name)

	// check for explicit extension (e.g., "jump.sh" or "publish/patch.sh")
	ext := filepath.Ext(name)
	if ext != "" {
		// remove extension to get key
		key := strings.TrimSuffix(name, ext)
		if scripts, ok := discovered[key]; ok {
			// find exact match by extension
			for _, script := range scripts {
				if script.Extension == ext {
					// check for conflicts
					if len(scripts) > 1 {
						var paths []string
						for _, s := range scripts {
							paths = append(paths, s.RelativePath)
						}
						return ResolvedScript{}, fmt.Errorf("multiple scripts for %q exist, exiting: %s",
							key, strings.Join(paths, ", "))
					}
					return ResolvedScript{
						Source: ScriptSourceFolder,
						Path:   script.Path,
					}, nil
				}
			}
		}
	}

	// check by key (without extension) - handles path-based commands
	// This handles both regular files (e.g., "publish/patch.sh" -> key "publish/patch")
	// and index files (e.g., "publish/index.sh" -> key "publish")
	key := strings.TrimSuffix(name, ext)
	if scripts, ok := discovered[key]; ok {
		if len(scripts) > 1 {
			// conflict - return error (e.g., both "publish.sh" and "publish/index.sh" exist)
			var paths []string
			for _, script := range scripts {
				paths = append(paths, script.RelativePath)
			}
			return ResolvedScript{}, fmt.Errorf("multiple scripts for %q exist, exiting: %s",
				key, strings.Join(paths, ", "))
		}
		if len(scripts) == 1 {
			return ResolvedScript{
				Source: ScriptSourceFolder,
				Path:   scripts[0].Path,
			}, nil
		}
	}

	// check package.json scripts (only for simple names, not path-based)
	// Path-based commands like "publish/patch" should not match package.json scripts
	if !strings.Contains(name, "/") {
		pkgScripts, err := ReadPackageJSONScripts(root)
		if err != nil {
			return ResolvedScript{}, fmt.Errorf("read package.json scripts: %w", err)
		}

		if command, ok := pkgScripts[name]; ok {
			return ResolvedScript{
				Source: ScriptSourcePackageJSON,
				Path:   command,
			}, nil
		}
	}

	// not found
	return ResolvedScript{Source: ScriptSourceNone}, nil
}

// check if script exists in any source
func HasScript(name string, root string, cfg config.Config) (bool, error) {
	resolved, err := ResolveScript(name, root, cfg)
	if err != nil {
		// if error is a conflict, script exists but has conflict
		if strings.Contains(err.Error(), "multiple scripts") {
			return true, err
		}
		return false, err
	}
	return resolved.Source != ScriptSourceNone, nil
}
