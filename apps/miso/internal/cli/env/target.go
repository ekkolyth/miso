package env

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/workspace"
)

// order: global, then root entries scoped to the target, then member-local (member
// targets only). Later layers win on key conflict. Result is gap-filled under
// os.Environ() so the ambient shell wins.
// targetEntries returns the declared entries that apply to target, in layer
// order: global first, then whichever config names the target — a root-scoped
// entry or the member's own, never both (checkScopeExclusivity enforces that).
func targetEntries(projectRoot string, cfg config.Config, target workspace.Target) ([]scopedEntry, error) {
	var applied []scopedEntry
	for _, entry := range cfg.Env {
		if entry.Scope == "global" {
			applied = append(applied, scopedEntry{entry: entry, baseDir: projectRoot})
		}
	}
	for _, entry := range cfg.Env {
		if entry.Scope == "global" {
			continue
		}
		if entry.Scope == target.Name {
			applied = append(applied, scopedEntry{entry: entry, baseDir: projectRoot})
			continue
		}
		if target.Kind == workspace.TargetMember && target.Dir != "" && entry.Scope == filepath.Base(target.Dir) {
			applied = append(applied, scopedEntry{entry: entry, baseDir: projectRoot})
		}
	}
	if target.Kind == workspace.TargetMember && target.Dir != "" {
		memberCfg, err := config.Load(target.Dir)
		switch {
		case errors.Is(err, config.ErrNotFound):
		case err != nil:
			return nil, fmt.Errorf("load %s: %w", relativeTo(projectRoot, config.Path(target.Dir)), err)
		default:
			for _, entry := range memberCfg.Env {
				applied = append(applied, scopedEntry{entry: entry, baseDir: target.Dir, memberName: target.Name})
			}
		}
	}
	return applied, nil
}

// TargetSummary counts the declared scopes and variables that apply to target —
// what `--env` checked on its behalf. Zero scopes means nothing was declared for
// it, so callers have nothing to report.
func TargetSummary(projectRoot string, cfg config.Config, target workspace.Target) (scopes int, variables int) {
	// a broken member config already failed BuildTargetEnv before anything reports
	entries, _ := targetEntries(projectRoot, cfg, target)
	for _, applied := range entries {
		scopes++
		variables += len(applied.entry.Variables.Object) + len(applied.entry.Variables.Array)
	}
	return scopes, variables
}

// ValidatedLine reports what --env checked on this target's behalf, for a
// runner to print above the process's own output. Empty when nothing was
// declared for it — there's no result to report.
func ValidatedLine(projectRoot string, cfg config.Config, target workspace.Target) string {
	scopes, variables := TargetSummary(projectRoot, cfg, target)
	if scopes == 0 {
		return ""
	}
	return fmt.Sprintf("env validated — %s, %s", plural(scopes, "scope"), plural(variables, "variable"))
}

func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// a file that won't load fails the run, labelled as `miso env` labels it
func BuildTargetEnv(projectRoot string, cfg config.Config, target workspace.Target) ([]string, error) {
	entries, err := targetEntries(projectRoot, cfg, target)
	if err != nil {
		return nil, err
	}
	vars := make(map[string]string)
	loaded := false
	var discoveryFailures, memberFailures []entryErrors
	rootFailures := make(map[int]entryErrors)

	// Injection follows orchestration, not repo mode: a turbo/nx run that turbo
	// owns goes through DelegateLaunch and never lands here, so reaching this
	// point means miso is spawning the process and owns its env.
	if len(cfg.Env) == 0 {
		searchDir := projectRoot
		if target.Kind == workspace.TargetMember && target.Dir != "" {
			searchDir = target.Dir
		}
		if path, err := discoverEnvFile(searchDir); err == nil && !declaresFile(entries, path) {
			fileVars, err := loadEnvFile(path)
			if err != nil {
				discoveryFailures = append(discoveryFailures, entryErrors{label: relativeTo(projectRoot, path), errs: []error{err}})
			} else {
				loaded = true
				for key, value := range fileVars {
					vars[key] = value
				}
			}
		}
	}

	for _, applied := range entries {
		if applied.entry.Path == "" {
			continue
		}
		abs := applied.entry.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(applied.baseDir, applied.entry.Path)
		}
		var fileVars map[string]string
		err := checkEnvFile(abs)
		if err == nil {
			fileVars, err = loadEnvFile(abs)
		}
		if err != nil {
			if applied.memberName != "" {
				memberFailures = append(memberFailures, entryErrors{label: applied.memberName + ": " + entryLabel(applied.entry), errs: []error{err}})
			} else {
				rootFailures[slices.Index(cfg.Env, applied.entry)] = entryErrors{label: entryLabel(applied.entry), errs: []error{err}}
			}
			continue
		}
		loaded = true
		for key, value := range fileVars {
			vars[key] = value
		}
	}
	// ordered as `miso env` reports them: member entries, then root entries as declared
	failures := append(discoveryFailures, memberFailures...)
	for _, index := range slices.Sorted(maps.Keys(rootFailures)) {
		failures = append(failures, rootFailures[index])
	}
	if len(failures) > 0 {
		return nil, &ValidationError{summary: "env validation failed", failures: failures}
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

// a member entry naming the discovered file loads it already
func declaresFile(entries []scopedEntry, path string) bool {
	for _, applied := range entries {
		if applied.entry.Path != "" && resolveAgainst(applied.baseDir, applied.entry.Path) == filepath.Clean(path) {
			return true
		}
	}
	return false
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
