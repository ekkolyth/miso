package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/ui"
)

const EnvFlag = "--env"

// HasEnvFlag returns true if --env is in the given args
func HasEnvFlag(args []string) bool {
	for _, a := range args {
		if a == EnvFlag {
			return true
		}
	}
	return false
}

// StripEnvFlag returns a copy of args with --env removed
func StripEnvFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a != EnvFlag {
			out = append(out, a)
		}
	}
	return out
}

// StripEnvFromFlags returns a copy of cfg with --env removed from all flag slices
func StripEnvFromFlags(cfg config.Config) config.Config {
	if cfg.Flags == nil {
		return cfg
	}
	stripped := make(map[string][]string)
	for k, v := range cfg.Flags {
		stripped[k] = StripEnvFlag(v)
	}
	cfg.Flags = stripped
	return cfg
}

// discoveryOrder is the default order to look for .env files when no env config is set
var discoveryOrder = []string{
	".env.local",
	".env.production",
	".env.development",
	".env",
}

// entryErrors holds errors for a single entry, preserving order.
type entryErrors struct {
	label string
	errs  []error
}

// Run executes the miso env command: for each EnvEntry, resolve its path, load the file,
// validate variables, and report results. When no env config is present, falls back to
// discovery mode and reports which file was found.
func Run(projectRoot string, cfg config.Config, logger *log.Logger) error {
	if len(cfg.Env) == 0 {
		// No config: discovery mode
		path, err := discoverEnvFile(projectRoot)
		if err != nil {
			return err
		}
		logger.Info("env loaded", "path", path)
		return nil
	}

	var failures []entryErrors

	for _, entry := range cfg.Env {
		errs := runEntry(projectRoot, entry, logger)
		if len(errs) > 0 {
			failures = append(failures, entryErrors{
				label: entryLabel(entry),
				errs:  errs,
			})
		}
	}

	if len(failures) == 0 {
		return nil
	}

	// Print styled error block to stderr (blank line separates from INFO lines)
	fmt.Fprintln(os.Stderr)
	logger.Error("env validation failed")
	printGroupedErrors(os.Stderr, failures)

	return errors.New("env validation failed")
}

// printGroupedErrors writes styled, grouped errors to the given writer.
func printGroupedErrors(w *os.File, failures []entryErrors) {
	warnStyle := lipgloss.NewStyle().Foreground(ui.WarningColor)

	for i, f := range failures {
		labelColor := ui.LabelColors[i%len(ui.LabelColors)]
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(labelColor)

		fmt.Fprintf(w, "  %s\n", labelStyle.Render(f.label))
		for _, e := range f.errs {
			if ve, ok := e.(*varError); ok {
				fmt.Fprintf(w, "    %s %s\n", warnStyle.Render(ve.name+":"), ve.msg)
			} else {
				fmt.Fprintf(w, "    %s\n", e.Error())
			}
		}
	}
}

// runEntry resolves, loads, and validates a single EnvEntry.
// Returns a slice of validation errors (nil if all passed).
func runEntry(projectRoot string, entry *config.EnvEntry, logger *log.Logger) []error {
	label := entryLabel(entry)

	// Resolve path
	absPath, err := resolveEntryPath(projectRoot, entry)
	if err != nil {
		return []error{err}
	}

	// Load env file
	envMap, err := loadEnvFile(absPath)
	if err != nil {
		return []error{err}
	}

	// No variables defined: just report the load
	if len(entry.Variables.Object) == 0 && len(entry.Variables.Array) == 0 {
		logger.Info("env loaded", "label", label, "path", absPath)
		return nil
	}

	// Presence-only (array) mode
	if len(entry.Variables.Array) > 0 {
		var errs []error
		for _, key := range entry.Variables.Array {
			if _, ok := envMap[key]; !ok {
				errs = append(errs, &varError{name: key, msg: "missing required variable"})
			}
		}
		if len(errs) > 0 {
			return errs
		}
		logger.Info("env validation passed", "label", label, "variables", len(entry.Variables.Array))
		return nil
	}

	// Object mode — full type validation
	if errs := validateVariables(envMap, entry.Variables.Object, entry.Required); len(errs) > 0 {
		return errs
	}
	logger.Info("env validation passed", "label", label, "variables", len(entry.Variables.Object))
	return nil
}

// entryLabel returns a human-readable identifier for an entry used in log/error output.
func entryLabel(entry *config.EnvEntry) string {
	if entry.Label != "" {
		return entry.Label
	}
	if entry.Path != "" {
		return entry.Path
	}
	return "env"
}

// resolveEntryPath returns the absolute path for the entry's configured path.
// Unlike the old multi-path form, each entry has exactly one path; if it is
// empty we fall back to discovery so that label-only entries still work.
func resolveEntryPath(projectRoot string, entry *config.EnvEntry) (string, error) {
	if entry.Path != "" {
		abs := filepath.Join(projectRoot, entry.Path)
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("env file not found: %s", abs)
			}
			return "", fmt.Errorf("env file %s: %w", abs, err)
		}
		return abs, nil
	}

	// No path on this entry: fall back to discovery
	return discoverEnvFile(projectRoot)
}

// discoverEnvFile walks discoveryOrder and returns the first file that exists.
func discoverEnvFile(projectRoot string) (string, error) {
	for _, name := range discoveryOrder {
		p := filepath.Join(projectRoot, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	tried := ""
	for i, n := range discoveryOrder {
		if i > 0 {
			tried += ", "
		}
		tried += filepath.Join(projectRoot, n)
	}
	return "", fmt.Errorf("no .env file found (tried: %s)", tried)
}

// loadEnvFile reads a single .env file and returns its key-value map.
func loadEnvFile(path string) (map[string]string, error) {
	envMap, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}
	return envMap, nil
}

// BuildProcessEnv builds a merged environment for spawning scripts.
// It loads .env files and merges them with os.Environ(). Process env wins —
// .env values only fill in vars that aren't already set.
// Returns nil if no env files are found/configured (caller should use default inherited env).
func BuildProcessEnv(projectRoot string, cfg config.Config, workspaceDir string) ([]string, error) {
	fileVars := make(map[string]string)
	loaded := false

	if len(cfg.Env) > 0 {
		for _, entry := range cfg.Env {
			absPath := filepath.Join(projectRoot, entry.Path)
			if entry.Path == "" {
				searchDir := projectRoot
				if cfg.IsMonorepo() && workspaceDir != projectRoot {
					searchDir = workspaceDir
				}
				discovered, err := discoverEnvFile(searchDir)
				if err != nil {
					continue
				}
				absPath = discovered
			}

			// Monorepo scoping: only load entries whose path falls under workspaceDir
			if cfg.IsMonorepo() && workspaceDir != projectRoot {
				if !strings.HasPrefix(absPath, workspaceDir+string(filepath.Separator)) && absPath != workspaceDir {
					continue
				}
			}

			envMap, err := loadEnvFile(absPath)
			if err != nil {
				continue
			}
			loaded = true
			for k, v := range envMap {
				fileVars[k] = v
			}
		}
	} else {
		searchDir := projectRoot
		if cfg.IsMonorepo() && workspaceDir != projectRoot {
			searchDir = workspaceDir
		}
		path, err := discoverEnvFile(searchDir)
		if err != nil {
			return nil, nil
		}
		envMap, err := loadEnvFile(path)
		if err != nil {
			return nil, nil
		}
		loaded = true
		for k, v := range envMap {
			fileVars[k] = v
		}
	}

	if !loaded {
		return nil, nil
	}

	processEnv := os.Environ()
	existing := make(map[string]bool)
	for _, e := range processEnv {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) >= 1 {
			existing[parts[0]] = true
		}
	}

	for k, v := range fileVars {
		if !existing[k] {
			processEnv = append(processEnv, k+"="+v)
		}
	}

	return processEnv, nil
}
