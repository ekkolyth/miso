package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"github.com/ekkolyth/miso/internal/config"
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

	return formatGroupedErrors(failures)
}

// formatGroupedErrors builds a single error with all failures grouped by label.
func formatGroupedErrors(failures []entryErrors) error {
	var b strings.Builder
	b.WriteString("env validation failed:")
	for _, f := range failures {
		fmt.Fprintf(&b, "\n  %s:", f.label)
		for _, e := range f.errs {
			fmt.Fprintf(&b, "\n    - %s", e.Error())
		}
	}
	return fmt.Errorf("%s", b.String())
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
				errs = append(errs, fmt.Errorf("missing required variable: %s", key))
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
