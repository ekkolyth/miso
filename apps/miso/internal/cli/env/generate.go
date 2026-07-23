package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/huh/v2"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	"github.com/joho/godotenv"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/workspace"
)

// GenerateOptions toggles the populate/override layers on top of the keys-only
// template, or -x/--example which fills type-valid fakes into a `.env.example`.
type GenerateOptions struct {
	Populate bool
	Override bool
	Example  bool
}

// ParseGenerateFlags reads the tokens after `miso env`. -p/-o require -g; -x
// implies generation and can't combine with -p/-o.
func ParseGenerateFlags(args []string) (bool, GenerateOptions, error) {
	var generate bool
	var opts GenerateOptions
	for _, arg := range args {
		switch arg {
		case "-g", "--generate":
			generate = true
		case "-p", "--populate":
			opts.Populate = true
		case "-o", "--override":
			opts.Override = true
		case "-x", "--example":
			opts.Example = true
			generate = true
		default:
			return false, GenerateOptions{}, fmt.Errorf("unknown env flag: %s", arg)
		}
	}
	if (opts.Populate || opts.Override) && !generate {
		return false, GenerateOptions{}, errors.New("-p/--populate and -o/--override require -g/--generate")
	}
	if opts.Example && (opts.Populate || opts.Override) {
		return false, GenerateOptions{}, errors.New("-x/--example fills fake values and can't combine with -p/--populate or -o/--override")
	}
	return generate, opts, nil
}

// declared variable names (array preserved, object unordered)
func variableKeys(vars config.EnvVariables) []string {
	if len(vars.Array) > 0 {
		return append([]string(nil), vars.Array...)
	}
	keys := make([]string, 0, len(vars.Object))
	for name := range vars.Object {
		keys = append(keys, name)
	}
	return keys
}

// header, then one KEY=value line per key, in order
func renderEnvFile(header string, keys []string, values map[string]string) string {
	var out strings.Builder
	out.WriteString(header + "\n")
	for _, key := range keys {
		out.WriteString(key)
		out.WriteString("=")
		out.WriteString(values[key])
		out.WriteString("\n")
	}
	return out.String()
}

// empty map for a missing file; parse errors on an existing file surface
func readEnvSoft(path string) (map[string]string, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	return godotenv.Read(path)
}

// destination dir for a scope's .env.generated: a member scope → member dir;
// "global" or any root-level scope (a root script/task name) → the repo root.
func scopeDir(scope, projectRoot string, members []workspace.Member) string {
	if scope == "global" {
		return projectRoot
	}
	if member, err := workspace.Find(scope, members, projectRoot); err == nil {
		return member.Dir
	}
	return projectRoot
}

// scopedEntry pairs an env entry with the directory its path/override resolve from.
type scopedEntry struct {
	entry   *config.EnvEntry
	baseDir string
}

// collectScopes groups declared entries by scope. Root entries use their declared
// scope and resolve paths from the repo root; member-local entries take the
// member's name as scope and resolve from the member dir.
func collectScopes(projectRoot string, cfg config.Config, members []workspace.Member) map[string][]scopedEntry {
	scopes := map[string][]scopedEntry{}
	for _, entry := range cfg.Env {
		if entry.Scope == "" {
			continue
		}
		scopes[entry.Scope] = append(scopes[entry.Scope], scopedEntry{entry: entry, baseDir: projectRoot})
	}
	for _, member := range members {
		if member.ConfigPath == "" {
			continue
		}
		memberCfg, err := config.Load(member.Dir)
		if err != nil {
			continue
		}
		for _, entry := range memberCfg.Env {
			scopes[member.Name] = append(scopes[member.Name], scopedEntry{entry: entry, baseDir: member.Dir})
		}
	}
	return scopes
}

// buildScopeEnv returns the sorted declared key set and a values map for a scope:
// empty values for keys-only, baseline values with -p, override values winning with -o.
func buildScopeEnv(entries []scopedEntry, opts GenerateOptions) ([]string, map[string]string, error) {
	keySet := map[string]bool{}
	for _, scoped := range entries {
		for _, key := range variableKeys(scoped.entry.Variables) {
			keySet[key] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = ""
	}

	layer := func(source func(scopedEntry) string) error {
		for _, scoped := range entries {
			rel := source(scoped)
			if rel == "" {
				continue
			}
			found, err := readEnvSoft(filepath.Join(scoped.baseDir, rel))
			if err != nil {
				return err
			}
			for _, key := range keys {
				if val, ok := found[key]; ok {
					values[key] = val
				}
			}
		}
		return nil
	}

	if opts.Populate {
		if err := layer(func(scoped scopedEntry) string { return scoped.entry.Path }); err != nil {
			return nil, nil, err
		}
	}
	if opts.Override {
		if err := layer(func(scoped scopedEntry) string { return scoped.entry.Override }); err != nil {
			return nil, nil, err
		}
	}
	return keys, values, nil
}

// validateEntryValues checks a built values map against an entry's declared rules,
// reusing the same validators as `miso env`. An empty value for a required key fails.
func validateEntryValues(values map[string]string, entry *config.EnvEntry) []error {
	if len(entry.Variables.Array) > 0 {
		var errs []error
		for _, key := range entry.Variables.Array {
			if val, ok := values[key]; !ok || val == "" {
				errs = append(errs, &varError{name: key, msg: "missing required variable"})
			}
		}
		return errs
	}
	if len(entry.Variables.Object) > 0 {
		return validateVariables(values, entry.Variables.Object, entry.Required)
	}
	return nil
}

// miso env entry point: parse flags, then generate or validate
func Command(projectRoot string, cfg config.Config, logger *log.Logger, args []string) error {
	generate, opts, err := ParseGenerateFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		logger.Error(err.Error())
		return err
	}
	if generate {
		return Generate(projectRoot, cfg, logger, opts)
	}
	return Run(projectRoot, cfg, logger)
}

// one .env.generated per destination dir — root-level scopes (global + any
// non-member scope) merge into the root file. With -p/-o it validates the
// resolved values first and writes nothing if any scope fails.
func Generate(projectRoot string, cfg config.Config, logger *log.Logger, opts GenerateOptions) error {
	members, err := workspace.DiscoverMembers(projectRoot, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		logger.Error("failed to discover workspaces", "error", err)
		return fmt.Errorf("discover members: %w", err)
	}
	scopes := collectScopes(projectRoot, cfg, members)
	if len(scopes) == 0 {
		logger.Info("no scoped env entries to generate")
		return nil
	}

	scopeNames := make([]string, 0, len(scopes))
	for name := range scopes {
		scopeNames = append(scopeNames, name)
	}
	sort.Strings(scopeNames)

	header := "# generated by miso env"
	if opts.Example {
		header = "# generated by miso env --example (fake values — replace before use)"
	}

	type output struct {
		dir  string
		keys []string
		text string
	}
	var outputs []output
	var failures []entryErrors

	// group scopes by the directory their file lands in; global + any root-level
	// scope share the repo root, so their keys merge into a single file.
	dirEntries := map[string][]scopedEntry{}
	var dirOrder []string
	for _, scope := range scopeNames {
		dir := scopeDir(scope, projectRoot, members)
		if _, seen := dirEntries[dir]; !seen {
			dirOrder = append(dirOrder, dir)
		}
		dirEntries[dir] = append(dirEntries[dir], scopes[scope]...)
	}
	sort.Strings(dirOrder)

	for _, dir := range dirOrder {
		entries := dirEntries[dir]
		var keys []string
		var values map[string]string
		if opts.Example {
			keys, values = buildExampleEnv(entries)
		} else {
			keys, values, err = buildScopeEnv(entries, opts)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				logger.Error("env generate: failed to build env", "dir", dir, "error", err)
				return err
			}
			if opts.Populate || opts.Override {
				for _, scoped := range entries {
					if errs := validateEntryValues(values, scoped.entry); len(errs) > 0 {
						failures = append(failures, entryErrors{
							label: entryLabel(scoped.entry),
							errs:  errs,
						})
					}
				}
			}
		}
		outputs = append(outputs, output{dir: dir, keys: keys, text: renderEnvFile(header, keys, values)})
	}

	if len(failures) > 0 {
		fmt.Fprintln(os.Stderr)
		logger.Error("env generation failed validation")
		printGroupedErrors(os.Stderr, failures)
		return errors.New("env generation failed validation")
	}

	filename := ".env.generated"
	if opts.Example {
		filename = ".env.example"
	}
	for _, out := range outputs {
		path := filepath.Join(out.dir, filename)
		if opts.Example {
			if _, statErr := os.Stat(path); statErr == nil && !confirmOverwrite(path) {
				logger.Info("env example skipped (exists)", "path", path)
				continue
			}
		}
		if err := os.WriteFile(path, []byte(out.text), 0o644); err != nil {
			fmt.Fprintln(os.Stderr)
			logger.Error("env generate: failed to write file", "path", path, "error", err)
			return fmt.Errorf("write %s: %w", path, err)
		}
		logger.Info("env generated", "path", path, "variables", len(out.keys))
	}
	return nil
}

// buildExampleEnv returns the sorted declared key set and a type-valid fake for
// each var — enough to satisfy `miso env` validation.
func buildExampleEnv(entries []scopedEntry) ([]string, map[string]string) {
	keySet := map[string]bool{}
	values := map[string]string{}
	for _, scoped := range entries {
		vars := scoped.entry.Variables
		for _, key := range vars.Array {
			keySet[key] = true
			values[key] = "changeme"
		}
		for key, vc := range vars.Object {
			keySet[key] = true
			values[key] = fakeValue(resolveVarConfig(vc))
		}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, values
}

// confirmOverwrite asks before clobbering an existing file; a non-interactive
// terminal refuses rather than overwrite silently.
func confirmOverwrite(path string) bool {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return false
	}
	overwrite := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("%s already exists — overwrite?", path)).
			Value(&overwrite),
	)).WithTheme(huh.ThemeFunc(huh.ThemeCharm))
	if err := form.Run(); err != nil {
		return false
	}
	return overwrite
}
