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

	// check for explicit extension (e.g., "jump.sh")
	ext := filepath.Ext(name)
	if ext != "" {
		basename := strings.TrimSuffix(name, ext)
		if scripts, ok := discovered[basename]; ok {
			// find exact match
			for _, script := range scripts {
				if filepath.Ext(script.Path) == ext {
					return ResolvedScript{
						Source: ScriptSourceFolder,
						Path:   script.Path,
					}, nil
				}
			}
		}
		// check if it's in discovered scripts
		for _, scripts := range discovered {
			for _, script := range scripts {
				if filepath.Base(script.Path) == name {
					return ResolvedScript{
						Source: ScriptSourceFolder,
						Path:   script.Path,
					}, nil
				}
			}
		}
	}

	// check by basename (without extension)
	basename := strings.TrimSuffix(name, ext)
	if scripts, ok := discovered[basename]; ok {
		if len(scripts) > 1 {
			// conflict - return error
			var paths []string
			for _, script := range scripts {
				paths = append(paths, filepath.Base(script.Path))
			}
			return ResolvedScript{}, fmt.Errorf("multiple scripts named %q found: %s. use 'miso %s' or 'miso %s'",
				basename, strings.Join(paths, ", "), paths[0], paths[1])
		}
		if len(scripts) == 1 {
			return ResolvedScript{
				Source: ScriptSourceFolder,
				Path:   scripts[0].Path,
			}, nil
		}
	}

	// check package.json scripts
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
