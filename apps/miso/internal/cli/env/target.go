package env

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/workspace"
)

// order: global, then root entries scoped to the target, then member-local (member
// targets only). Later layers win on key conflict. Result is gap-filled under
// os.Environ() so the ambient shell wins.
func BuildTargetEnv(projectRoot string, cfg config.Config, target workspace.Target) ([]string, error) {
	vars := make(map[string]string)
	loaded := false

	// Injection follows orchestration, not repo mode: a turbo/nx run that turbo
	// owns goes through DelegateLaunch and never lands here, so reaching this
	// point means miso is spawning the process and owns its env.
	if len(cfg.Env) == 0 {
		searchDir := projectRoot
		if target.Kind == workspace.TargetMember && target.Dir != "" {
			searchDir = target.Dir
		}
		if path, err := discoverEnvFile(searchDir); err == nil {
			if fileVars, err := loadEnvFile(path); err == nil {
				loaded = true
				for key, value := range fileVars {
					vars[key] = value
				}
			}
		}
	}

	apply := func(entry *config.EnvEntry, baseDir string) {
		if entry.Path == "" {
			return
		}
		abs := entry.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(baseDir, entry.Path)
		}
		fileVars, err := loadEnvFile(abs)
		if err != nil {
			return
		}
		loaded = true
		for key, value := range fileVars {
			vars[key] = value
		}
	}

	for _, entry := range cfg.Env {
		if entry.Scope == "global" {
			apply(entry, projectRoot)
		}
	}
	for _, entry := range cfg.Env {
		if entry.Scope == "global" {
			continue
		}
		if entry.Scope == target.Name {
			apply(entry, projectRoot)
			continue
		}
		if target.Kind == workspace.TargetMember && target.Dir != "" && entry.Scope == filepath.Base(target.Dir) {
			apply(entry, projectRoot)
		}
	}
	if target.Kind == workspace.TargetMember && target.Dir != "" {
		if memberCfg, err := config.Load(target.Dir); err == nil {
			for _, entry := range memberCfg.Env {
				apply(entry, target.Dir)
			}
		}
	}

	startDir := projectRoot
	if target.Kind == workspace.TargetMember && target.Dir != "" {
		startDir = target.Dir
	}
	binDirs := collectBinDirs(startDir, projectRoot)

	if !loaded && len(binDirs) == 0 {
		return nil, nil
	}

	processEnv := os.Environ()
	existing := make(map[string]bool)
	for _, kv := range processEnv {
		if key, _, ok := strings.Cut(kv, "="); ok {
			existing[key] = true
		}
	}
	for key, value := range vars {
		if !existing[key] {
			processEnv = append(processEnv, key+"="+value)
		}
	}
	if len(binDirs) > 0 {
		processEnv = prependPath(processEnv, binDirs)
	}
	return processEnv, nil
}

// collectBinDirs returns existing node_modules/.bin dirs from startDir up to
// projectRoot (inclusive), nearest first.
func collectBinDirs(startDir, projectRoot string) []string {
	abs := func(p string) string {
		if a, err := filepath.Abs(p); err == nil {
			return a
		}
		return p
	}
	root := abs(projectRoot)
	dir := abs(startDir)

	var dirs []string
	seen := make(map[string]bool)
	for {
		bin := filepath.Join(dir, "node_modules", ".bin")
		if info, err := os.Stat(bin); err == nil && info.IsDir() && !seen[bin] {
			seen[bin] = true
			dirs = append(dirs, bin)
		}
		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

// prependPath puts binDirs at the front of PATH in environ (local bins win).
func prependPath(environ []string, binDirs []string) []string {
	prefix := strings.Join(binDirs, string(os.PathListSeparator))
	for i, kv := range environ {
		if key, val, ok := strings.Cut(kv, "="); ok && key == "PATH" {
			if val != "" {
				environ[i] = "PATH=" + prefix + string(os.PathListSeparator) + val
			} else {
				environ[i] = "PATH=" + prefix
			}
			return environ
		}
	}
	return append(environ, "PATH="+prefix)
}
