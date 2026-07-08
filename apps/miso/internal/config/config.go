package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// return when miso.json not found
var ErrNotFound = errors.New("config: not found")

// default miso config filename
const FileName = "miso.json"

// SchemaURL is the canonical URL for the miso.json JSON schema (IDE autocomplete/validation)
const SchemaURL = "https://misojs.dev/miso.schema.json"

// TaskConfig holds per-task configuration for dependency ordering and concurrent companions.
type TaskConfig struct {
	DependsOn  []string `json:"dependsOn,omitempty"`
	Concurrent []string `json:"concurrent,omitempty"`
}

// persisted project metadata
type Config struct {
	Schema         string                `json:"$schema,omitempty"`
	PackageManager *bool                 `json:"packageManager,omitempty"`
	Scripts        string                `json:"scripts"`
	Shell          string                `json:"shell,omitempty"`
	Flags          map[string][]string   `json:"flags,omitempty"`
	Env            []*EnvEntry           `json:"env,omitempty"`
	Repo           string                `json:"repo,omitempty"` // "miso" (default), "turbo", or "nx"
	Tasks          map[string]TaskConfig `json:"-"`              // populated from repo object form; not serialized directly
	TuiMode        string                `json:"tui,omitempty"`  // "off" (default), "tabbed", or "merged"
	TuiCleanExit   bool                  `json:"-"`              // populated from tui object form; not serialized directly
}

// EnvEntry holds a single env file path and its variable validation rules.
// label is optional but recommended in multi-app setups.
type EnvEntry struct {
	Scope     string       `json:"scope,omitempty"`
	Label     string       `json:"label,omitempty"`
	Path      string       `json:"path,omitempty"`
	Required  EnvRequired  `json:"required,omitempty"`
	Variables EnvVariables `json:"variables,omitempty"`
}

// EnvRequired is "all" | "none" | []string
type EnvRequired struct {
	Mode string   // "all", "none", or ""
	Keys []string // when Mode is "", use Keys for specific required keys
}

// MarshalJSON emits schema-compatible JSON: string ("all"/"none") or array of keys.
func (r EnvRequired) MarshalJSON() ([]byte, error) {
	if r.Mode != "" {
		return json.Marshal(r.Mode)
	}
	if len(r.Keys) > 0 {
		return json.Marshal(r.Keys)
	}
	return []byte("null"), nil
}

// EnvVariables is either object (map) or array - mutually exclusive
type EnvVariables struct {
	Object map[string]VarConfigOrString // type validation
	Array  []string                     // presence only
}

// MarshalJSON emits schema-compatible JSON: array or object (never both).
func (v EnvVariables) MarshalJSON() ([]byte, error) {
	if len(v.Array) > 0 {
		return json.Marshal(v.Array)
	}
	if len(v.Object) > 0 {
		m := make(map[string]interface{}, len(v.Object))
		for k, val := range v.Object {
			m[k] = val
		}
		return json.Marshal(m)
	}
	return []byte("null"), nil
}

// VarConfigOrString is either a type string shorthand or full VarConfig
type VarConfigOrString struct {
	IsShorthand bool
	Type        string
	Config      VarConfig
}

// MarshalJSON emits schema-compatible JSON: string (shorthand) or VarConfig object.
func (v VarConfigOrString) MarshalJSON() ([]byte, error) {
	if v.IsShorthand {
		return json.Marshal(v.Type)
	}
	return json.Marshal(v.Config)
}

// VarConfig holds per-variable validation rules
type VarConfig struct {
	Type        string   `json:"type"`
	Optional    bool     `json:"optional"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Values      []string `json:"values,omitempty"`
	Schemes     []string `json:"schemes,omitempty"`
	TrueValues  []string `json:"trueValues,omitempty"`
	FalseValues []string `json:"falseValues,omitempty"`
}

// initialize optional maps
func (c *Config) EnsureDefaults() {
	if c.Scripts == "" {
		c.Scripts = "./scripts"
	}
	if c.Flags == nil {
		c.Flags = make(map[string][]string)
	}
}

// IsMonorepo is retained only until callers migrate to workspace discovery (Task 10).
// After the collapse the native path branches on discovered members, not this flag.
func (c Config) IsMonorepo() bool {
	return c.IsDelegated()
}

// defaults to "miso"
func (c Config) RepoMode() string {
	if c.Repo == "" {
		return "miso"
	}
	return c.Repo
}

// turbo or nx
func (c Config) IsDelegated() bool {
	return c.Repo == "turbo" || c.Repo == "nx"
}

// checks for ^ prefix in dependsOn
func (c Config) HasDependsOn(command string) bool {
	if c.Tasks == nil {
		return false
	}
	task, ok := c.Tasks[command]
	if !ok {
		return false
	}
	for _, dep := range task.DependsOn {
		if len(dep) > 0 && dep[0] == '^' {
			return true
		}
	}
	return false
}

func (c Config) TaskConcurrent(command string) []string {
	if c.Tasks == nil {
		return nil
	}
	task, ok := c.Tasks[command]
	if !ok {
		return nil
	}
	if len(task.Concurrent) == 0 {
		return nil
	}
	return task.Concurrent
}

func (c Config) TuiEnabled() bool {
	return c.TuiMode == "tabbed" || c.TuiMode == "merged"
}

func (c Config) SimpleMode() bool {
	return c.PackageManager != nil && !*c.PackageManager
}

// resolve config file path for project root
func Path(root string) string {
	return filepath.Join(root, FileName)
}

// configLoad is used for two-phase unmarshaling (env can be string, object, or array)
type configLoad struct {
	Schema         string              `json:"$schema,omitempty"`
	PackageManager *bool               `json:"packageManager,omitempty"`
	Scripts        string              `json:"scripts"`
	Shell          string              `json:"shell,omitempty"`
	Flags          map[string][]string `json:"flags,omitempty"`
	EnvRaw         json.RawMessage     `json:"env,omitempty"`
	RepoRaw        json.RawMessage     `json:"repo,omitempty"`
	TuiRaw         json.RawMessage     `json:"tui,omitempty"`
}

// read miso.json from disk
func Load(root string) (Config, error) {
	path := Path(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, ErrNotFound
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var load configLoad
	if err := json.Unmarshal(data, &load); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	tuiMode := "off"
	tuiCleanExit := false
	if len(load.TuiRaw) > 0 {
		var err error
		tuiMode, tuiCleanExit, err = parseTuiField(load.TuiRaw)
		if err != nil {
			return Config{}, fmt.Errorf("parse tui config: %w", err)
		}
	}

	cfg := Config{
		Schema:         load.Schema,
		PackageManager: load.PackageManager,
		Scripts:        load.Scripts,
		Shell:          load.Shell,
		Flags:          load.Flags,
		TuiMode:        tuiMode,
		TuiCleanExit:   tuiCleanExit,
	}

	if len(load.RepoRaw) > 0 {
		repo, tasks, err := parseRepoField(load.RepoRaw)
		if err != nil {
			return Config{}, fmt.Errorf("parse repo config: %w", err)
		}
		cfg.Repo = repo
		cfg.Tasks = tasks
	}

	if len(load.EnvRaw) > 0 {
		entries, err := parseEnvField(load.EnvRaw)
		if err != nil {
			return Config{}, fmt.Errorf("parse env config: %w", err)
		}
		cfg.Env = entries
	}

	cfg.EnsureDefaults()
	return cfg, nil
}

// parseEnvField handles the three accepted shapes for the "env" field:
//
//  1. string  — legacy bare path: normalized to [{path: s}]
//  2. object  — legacy single-entry object: normalized to [{...}]
//  3. array   — canonical multi-entry form
func parseEnvField(raw json.RawMessage) ([]*EnvEntry, error) {
	// 1. bare string (legacy)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []*EnvEntry{{Path: s}}, nil
	}

	// 2. array (canonical)
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		entries := make([]*EnvEntry, 0, len(arr))
		for i, item := range arr {
			entry, err := parseEnvEntry(item)
			if err != nil {
				return nil, fmt.Errorf("env[%d]: %w", i, err)
			}
			entries = append(entries, entry)
		}
		return entries, nil
	}

	// 3. single object (legacy)
	entry, err := parseEnvEntry(raw)
	if err != nil {
		return nil, err
	}
	return []*EnvEntry{entry}, nil
}

// parseRepoField handles the two accepted shapes for the "repo" field:
//  1. string — "miso", "turbo", "nx"
//  2. object — { "mode": "turbo", "tasks": { ... } }
func parseRepoField(raw json.RawMessage) (string, map[string]TaskConfig, error) {
	var mode string
	var tasks map[string]TaskConfig

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		mode = s
	} else {
		var obj struct {
			Mode  string                `json:"mode"`
			Tasks map[string]TaskConfig `json:"tasks,omitempty"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return "", nil, fmt.Errorf("repo: expected string or object, got %s", string(raw))
		}
		mode = obj.Mode
		tasks = obj.Tasks
	}

	switch mode {
	case "", "miso", "turbo", "nx":
		return mode, tasks, nil
	default:
		return "", nil, fmt.Errorf("repo: unknown mode %q (use miso, turbo, or nx)", mode)
	}
}

// parseTuiField handles the two accepted shapes for the "tui" field:
//  1. string — "off", "tabbed", "merged"
//  2. object — { "mode": "tabbed", "cleanExit": true }
func parseTuiField(raw json.RawMessage) (string, bool, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			s = "off"
		}
		return s, false, nil
	}

	var obj struct {
		Mode      string `json:"mode"`
		CleanExit bool   `json:"cleanExit"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", false, fmt.Errorf("tui: expected string or object, got %s", string(raw))
	}

	mode := obj.Mode
	if mode == "" {
		mode = "off"
	}

	return mode, obj.CleanExit, nil
}

// parseEnvEntry decodes a single EnvEntry from its raw JSON representation.
// The legacy object form supported "path" as an array; we accept that here
// and use the first element (later elements were loaded together anyway).
func parseEnvEntry(raw json.RawMessage) (*EnvEntry, error) {
	var obj struct {
		Scope     string          `json:"scope,omitempty"`
		Label     string          `json:"label,omitempty"`
		Path      json.RawMessage `json:"path,omitempty"`
		Required  json.RawMessage `json:"required,omitempty"`
		Variables json.RawMessage `json:"variables,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	entry := &EnvEntry{Scope: obj.Scope, Label: obj.Label}

	// path: accept string or legacy []string
	if len(obj.Path) > 0 {
		var ps string
		if err := json.Unmarshal(obj.Path, &ps); err == nil {
			entry.Path = ps
		} else {
			var pa []string
			if err := json.Unmarshal(obj.Path, &pa); err != nil {
				return nil, fmt.Errorf("env.path: expected string or array of strings")
			}
			if len(pa) > 0 {
				entry.Path = pa[0]
			}
		}
	}

	// required
	if len(obj.Required) > 0 {
		var rs string
		if err := json.Unmarshal(obj.Required, &rs); err == nil {
			entry.Required.Mode = rs
		} else {
			var rk []string
			if err := json.Unmarshal(obj.Required, &rk); err == nil {
				entry.Required.Keys = rk
			} else {
				return nil, fmt.Errorf("env.required: invalid value %s (expected string or array of strings)", string(obj.Required))
			}
		}
	}

	// variables
	if len(obj.Variables) > 0 {
		vars, err := parseEnvVariables(obj.Variables)
		if err != nil {
			return nil, err
		}
		entry.Variables = vars
	}

	return entry, nil
}

// parseEnvVariables handles variables as object or array
func parseEnvVariables(raw json.RawMessage) (EnvVariables, error) {
	// try array first
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return EnvVariables{Array: arr}, nil
	}

	// object - values can be string (type) or VarConfig
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return EnvVariables{}, err
	}

	result := EnvVariables{Object: make(map[string]VarConfigOrString)}
	for k, v := range obj {
		// try string (type shorthand)
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			result.Object[k] = VarConfigOrString{IsShorthand: true, Type: s}
			continue
		}
		// full VarConfig
		var cfg VarConfig
		if err := json.Unmarshal(v, &cfg); err != nil {
			return EnvVariables{}, fmt.Errorf("variable %q: %w", k, err)
		}
		result.Object[k] = VarConfigOrString{Config: cfg}
	}
	return result, nil
}

// save miso.json to disk, create parent dirs if needed
func Save(root string, cfg Config) error {
	cfg.EnsureDefaults()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	if err := os.WriteFile(Path(root), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// expands globs; returns only paths that exist on disk
func LoadWorkspaces(root string) ([]string, error) {
	pkgPath := filepath.Join(root, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read package.json: %w", err)
	}

	var pkg struct {
		Workspaces []string `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}

	if len(pkg.Workspaces) == 0 {
		return nil, nil
	}

	var result []string
	for _, pattern := range pkg.Workspaces {
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(root, pattern)
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("expand workspace glob %q: %w", pattern, err)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			if info.IsDir() {
				result = append(result, match)
			}
		}
	}

	return result, nil
}

// FindWorkspace matches a workspace identifier against discovered workspace paths.
// The identifier is matched against three candidates per workspace, in order:
//  1. Directory basename (e.g. "api" for /root/packages/api)
//  2. Relative path from root (e.g. "packages/api")
//  3. The "name" field in the workspace's package.json (e.g. "@myorg/api")
//
// If the identifier matches more than one workspace, an error is returned listing
// the conflicting paths. If no match is found, an error is returned listing available
// workspace basenames.
func FindWorkspace(name string, workspaces []string, root string) (string, error) {
	var matches []string

	for _, ws := range workspaces {
		if matchesWorkspace(name, ws, root) {
			matches = append(matches, ws)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("workspace %q not found (available: %s)", name, joinWorkspaceNames(workspaces))
	default:
		var paths []string
		for _, m := range matches {
			rel, err := filepath.Rel(root, m)
			if err != nil {
				paths = append(paths, m)
			} else {
				paths = append(paths, rel)
			}
		}
		return "", fmt.Errorf("workspace %q is ambiguous — matches multiple workspaces: %s (use a more specific identifier)", name, joinStrings(paths))
	}
}

// matchesWorkspace returns true if name matches the given workspace path
// by basename, relative path from root, or package.json name field.
func matchesWorkspace(name string, ws string, root string) bool {
	// 1. basename
	if filepath.Base(ws) == name {
		return true
	}

	// 2. relative path from root
	rel, err := filepath.Rel(root, ws)
	if err == nil && filepath.ToSlash(rel) == filepath.ToSlash(name) {
		return true
	}

	// 3. package.json name field
	pkgName := readPackageJSONName(ws)
	if pkgName != "" && pkgName == name {
		return true
	}

	return false
}

// readPackageJSONName reads the "name" field from a workspace's package.json.
// Returns empty string if the file is missing or the field is absent.
func readPackageJSONName(wsDir string) string {
	data, err := os.ReadFile(filepath.Join(wsDir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Name
}

// joinWorkspaceNames returns a comma-separated list of workspace basenames.
// Defined here (config package) — also duplicated in scripting package; do NOT import across packages.
func joinWorkspaceNames(workspaces []string) string {
	names := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		names = append(names, filepath.Base(ws))
	}
	return joinStrings(names)
}

// joinStrings joins a slice of strings with ", ".
// Defined here (config package) — also duplicated in scripting package; do NOT import across packages.
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
