package scripting

import (
	"errors"
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

// ErrAmbiguousScript marks a resolution that matched more than one definition:
// two folder scripts sharing a key, or a folder script AND a package.json script
// of the same name. miso refuses to guess which one you meant.
var ErrAmbiguousScript = errors.New("ambiguous script")

// scriptsFolder resolves the scripts directory for root (defaults to ./scripts).
func scriptsFolder(cfg config.Config, root string) string {
	path := cfg.Scripts
	if path == "" {
		path = "./scripts"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return path
}

// matchFolderScript finds the single folder script for name — an explicit
// extension match first, then a bare-key match. Returns nil when none matches,
// and ErrAmbiguousScript when two files share the key.
func matchFolderScript(discovered map[string][]ScriptInfo, name, ext, key string) (*ScriptInfo, error) {
	if ext != "" {
		if scripts, ok := discovered[key]; ok {
			for i := range scripts {
				if scripts[i].Extension == ext {
					if len(scripts) > 1 {
						return nil, multipleScriptsError(key, scripts)
					}
					return &scripts[i], nil
				}
			}
		}
	}
	if scripts, ok := discovered[key]; ok {
		if len(scripts) > 1 {
			return nil, multipleScriptsError(key, scripts)
		}
		return &scripts[0], nil
	}
	return nil, nil
}

func multipleScriptsError(key string, scripts []ScriptInfo) error {
	paths := make([]string, 0, len(scripts))
	for _, s := range scripts {
		paths = append(paths, s.RelativePath)
	}
	return fmt.Errorf("%w: multiple scripts for %q exist: %s", ErrAmbiguousScript, key, strings.Join(paths, ", "))
}

// ResolveScript resolves a script by name from the scripts folder or package.json.
//
// A name may resolve from at most ONE source. miso deliberately does not pick a
// winner between a folder script and a package.json script of the same name —
// they invoke different things (a package.json script named after a turbo/nx
// task is how you get delegation + the TUI chrome, since miso controls that
// invocation; a folder script runs literally). Silently preferring one hides
// intent, so an overlap is a hard ErrAmbiguousScript telling you to rename one.
func ResolveScript(name string, root string, cfg config.Config) (ResolvedScript, error) {
	discovered, err := DiscoverScripts(scriptsFolder(cfg, root))
	if err != nil {
		return ResolvedScript{}, fmt.Errorf("discover scripts: %w", err)
	}

	name = filepath.ToSlash(name)
	ext := filepath.Ext(name)
	key := strings.TrimSuffix(name, ext)

	folder, err := matchFolderScript(discovered, name, ext, key)
	if err != nil {
		return ResolvedScript{}, err
	}

	// package.json scripts apply only to simple (non-path) names
	var pkgCommand string
	var pkgFound bool
	if !strings.Contains(name, "/") {
		pkgScripts, err := ReadPackageJSONScripts(root)
		if err != nil {
			return ResolvedScript{}, fmt.Errorf("read package.json scripts: %w", err)
		}
		pkgCommand, pkgFound = pkgScripts[name]
	}

	if folder != nil && pkgFound {
		return ResolvedScript{}, fmt.Errorf(
			"%w: %q is defined both as a scripts-folder script (%s) and a package.json script — rename one so the invocation is unambiguous",
			ErrAmbiguousScript, name, folder.RelativePath)
	}

	if folder != nil {
		return ResolvedScript{Source: ScriptSourceFolder, Path: folder.Path}, nil
	}
	if pkgFound {
		return ResolvedScript{Source: ScriptSourcePackageJSON, Path: pkgCommand}, nil
	}
	return ResolvedScript{Source: ScriptSourceNone}, nil
}

// ResolveScriptFolderOnly resolves a script from the folder only — no
// package.json fallback. Used in simple mode, where package.json is ignored.
func ResolveScriptFolderOnly(name string, root string, cfg config.Config) (ResolvedScript, error) {
	discovered, err := DiscoverScripts(scriptsFolder(cfg, root))
	if err != nil {
		return ResolvedScript{}, fmt.Errorf("discover scripts: %w", err)
	}

	name = filepath.ToSlash(name)
	ext := filepath.Ext(name)
	key := strings.TrimSuffix(name, ext)

	folder, err := matchFolderScript(discovered, name, ext, key)
	if err != nil {
		return ResolvedScript{}, err
	}
	if folder != nil {
		return ResolvedScript{Source: ScriptSourceFolder, Path: folder.Path}, nil
	}
	return ResolvedScript{Source: ScriptSourceNone}, nil
}

// check if script exists in any source
func HasScript(name string, root string, cfg config.Config) (bool, error) {
	resolved, err := ResolveScript(name, root, cfg)
	if err != nil {
		if errors.Is(err, ErrAmbiguousScript) {
			return true, err
		}
		return false, err
	}
	return resolved.Source != ScriptSourceNone, nil
}
