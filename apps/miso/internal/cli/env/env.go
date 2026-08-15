package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/ui"
	"github.com/ekkolyth/miso/internal/workspace"
)

const EnvFlag = "--env"

func HasEnvFlag(args []string) bool {
	for _, a := range args {
		if a == EnvFlag {
			return true
		}
	}
	return false
}

func StripEnvFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a != EnvFlag {
			out = append(out, a)
		}
	}
	return out
}

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
	members, err := workspace.DiscoverMembers(projectRoot, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		logger.Error("failed to discover workspaces", "error", err)
		return fmt.Errorf("discover members: %w", err)
	}

	// Member-local entries scope by location, not by a declared scope field.
	// Validation traverses them in every repo mode — a delegated runner owns
	// injection, not the gate that says the values are there.
	memberEntries, err := collectMemberEntries(projectRoot, members)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		logger.Error("failed to read a workspace config", "error", err)
		return err
	}

	if len(cfg.Env) == 0 && len(memberEntries) == 0 {
		// No config: discovery mode
		path, err := discoverEnvFile(projectRoot)
		if err != nil {
			return err
		}
		logger.Info("env loaded", "path", path)
		return nil
	}

	// Scope carries meaning only where miso resolves targets. A delegated repo
	// has no scope requirement, and "global" isn't reserved there.
	if !cfg.IsDelegated() {
		for _, member := range members {
			if member.Name == "global" {
				fmt.Fprintln(os.Stderr)
				logger.Error("member uses the reserved name \"global\"", "dir", member.Dir)
				return fmt.Errorf("member %q uses the reserved name \"global\"", member.Dir)
			}
		}
		var unscoped []string
		for _, entry := range cfg.Env {
			if entry.Scope == "" {
				unscoped = append(unscoped, entry.Path)
			}
		}
		if len(unscoped) > 0 {
			fmt.Fprintln(os.Stderr)
			logger.Error("env config invalid — every root entry needs a scope (a target name or \"global\")")
			for _, path := range unscoped {
				fmt.Fprintf(os.Stderr, "    %s\n", path)
			}
			return errors.New("env config invalid: missing scope")
		}
	}

	if err := checkScopeExclusivity(projectRoot, cfg, memberEntries, logger); err != nil {
		return err
	}

	var failures []entryErrors

	for _, owned := range memberEntries {
		errs := runEntry(owned.member.Dir, owned.entry, logger)
		if len(errs) > 0 {
			failures = append(failures, entryErrors{
				label: owned.member.Name + ": " + entryLabel(owned.entry),
				errs:  errs,
			})
		}
	}

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

// memberEntry pairs a member-local entry with the member that declared it.
type memberEntry struct {
	member workspace.Member
	entry  *config.EnvEntry
}

// collectMemberEntries flattens every member miso.json's env entries. A config
// that won't load is a failure, not an empty contribution — skipping it would
// report a clean run over requirements nothing checked.
func collectMemberEntries(projectRoot string, members []workspace.Member) ([]memberEntry, error) {
	var entries []memberEntry
	for _, member := range members {
		if member.ConfigPath == "" {
			continue
		}
		memberCfg, err := config.Load(member.Dir)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", relativeTo(projectRoot, member.ConfigPath), err)
		}
		for _, entry := range memberCfg.Env {
			entries = append(entries, memberEntry{member: member, entry: entry})
		}
	}
	return entries, nil
}

// relativeTo trims projectRoot off path for display, falling back to path itself.
func relativeTo(projectRoot, path string) string {
	if rel, err := filepath.Rel(projectRoot, path); err == nil {
		return rel
	}
	return path
}

// checkScopeExclusivity fails when a root entry scopes to a member that declares
// its own env. A scope lives in one config or the other, never split across both.
func checkScopeExclusivity(projectRoot string, cfg config.Config, memberEntries []memberEntry, logger *log.Logger) error {
	declaring := make(map[string]workspace.Member)
	for _, owned := range memberEntries {
		declaring[owned.member.Name] = owned.member
		declaring[filepath.Base(owned.member.Dir)] = owned.member
	}

	var conflicts []string
	for _, entry := range cfg.Env {
		if entry.Scope == "" || entry.Scope == "global" {
			continue
		}
		member, ok := declaring[entry.Scope]
		if !ok {
			continue
		}
		conflicts = append(conflicts, fmt.Sprintf("%s — root %s and %s", entry.Scope, config.FileName, relativeTo(projectRoot, member.ConfigPath)))
	}
	if len(conflicts) == 0 {
		return nil
	}

	fmt.Fprintln(os.Stderr)
	logger.Error("env scope declared in two places — keep it in the root config or the member's, not both")
	for _, conflict := range conflicts {
		fmt.Fprintf(os.Stderr, "    %s\n", conflict)
	}
	return errors.New("env config invalid: scope declared in two places")
}

// printGroupedErrors writes styled, grouped errors to the given writer.
func printGroupedErrors(w *os.File, failures []entryErrors) {
	warnStyle := lipgloss.NewStyle().Foreground(ui.WarningColor)

	for i, f := range failures {
		labelColor := ui.LabelColors[i%len(ui.LabelColors)]
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(labelColor)

		_, _ = fmt.Fprintf(w, "  %s\n", labelStyle.Render(f.label))
		for _, e := range f.errs {
			if ve, ok := e.(*varError); ok {
				_, _ = fmt.Fprintf(w, "    %s %s\n", warnStyle.Render(ve.name+":"), ve.msg)
			} else {
				_, _ = fmt.Fprintf(w, "    %s\n", e.Error())
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
