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

	if !loaded {
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
	return processEnv, nil
}
