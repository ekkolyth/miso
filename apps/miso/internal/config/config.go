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

// persisted project metadata
type Config struct {
	Schema         string              `json:"$schema,omitempty"`
	PackageManager string              `json:"package-manager"`
	ProjectName    string              `json:"name"`
	Scripts        string              `json:"scripts"`
	Shell          string              `json:"shell,omitempty"`
	Flags          map[string][]string `json:"flags,omitempty"`
	Env            *EnvConfig          `json:"env,omitempty"`
}

// EnvConfig holds env file path and variable validation rules.
type EnvConfig struct {
	Path      []string     `json:"path,omitempty"`
	Required  EnvRequired `json:"required,omitempty"`
	Variables EnvVariables `json:"variables,omitempty"`
}

// EnvRequired is "all" | "none" | []string
type EnvRequired struct {
	Mode string   // "all", "none", or ""
	Keys []string // when Mode is "", use Keys for specific required keys
}

// EnvVariables is either object (map) or array - mutually exclusive
type EnvVariables struct {
	Object map[string]VarConfigOrString // type validation
	Array  []string                     // presence only
}

// VarConfigOrString is either a type string shorthand or full VarConfig
type VarConfigOrString struct {
	IsShorthand bool
	Type        string
	Config      VarConfig
}

// VarConfig holds per-variable validation rules
type VarConfig struct {
	Type        string    `json:"type"`
	Optional    bool      `json:"optional"`
	Min         *float64  `json:"min,omitempty"`
	Max         *float64  `json:"max,omitempty"`
	Pattern     string    `json:"pattern,omitempty"`
	Values      []string  `json:"values,omitempty"`
	Schemes     []string  `json:"schemes,omitempty"`
	TrueValues  []string  `json:"trueValues,omitempty"`
	FalseValues []string  `json:"falseValues,omitempty"`
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

// resolve config file path for project root
func Path(root string) string {
	return filepath.Join(root, FileName)
}

// configLoad is used for two-phase unmarshaling (env can be string or object)
type configLoad struct {
	Schema         string              `json:"$schema,omitempty"`
	PackageManager string              `json:"package-manager"`
	ProjectName    string              `json:"name"`
	Scripts        string              `json:"scripts"`
	Shell          string              `json:"shell,omitempty"`
	Flags          map[string][]string  `json:"flags,omitempty"`
	EnvRaw         json.RawMessage     `json:"env,omitempty"`
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

	cfg := Config{
		Schema:         load.Schema,
		PackageManager: load.PackageManager,
		ProjectName:    load.ProjectName,
		Scripts:        load.Scripts,
		Shell:          load.Shell,
		Flags:          load.Flags,
	}

	if len(load.EnvRaw) > 0 {
		envCfg, err := parseEnvConfig(load.EnvRaw)
		if err != nil {
			return Config{}, fmt.Errorf("parse env config: %w", err)
		}
		cfg.Env = envCfg
	}

	cfg.EnsureDefaults()
	return cfg, nil
}

// parseEnvConfig handles env as string or object
func parseEnvConfig(raw json.RawMessage) (*EnvConfig, error) {
	// try string first (simple path)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return &EnvConfig{Path: []string{s}}, nil
	}

	// object - need custom Variables parsing
	var env struct {
		Path      []string         `json:"path,omitempty"`
		Required  json.RawMessage  `json:"required,omitempty"`
		Variables json.RawMessage  `json:"variables,omitempty"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}

	ec := &EnvConfig{Path: env.Path}

	// parse required
	if len(env.Required) > 0 {
		var s string
		if err := json.Unmarshal(env.Required, &s); err == nil {
			ec.Required.Mode = s
		} else {
			var keys []string
			if err := json.Unmarshal(env.Required, &keys); err == nil {
				ec.Required.Keys = keys
			} else {
				return nil, fmt.Errorf("env.required: invalid value %s (expected string or array of strings)", string(env.Required))
			}
		}
	}

	// parse variables (object or array)
	if len(env.Variables) > 0 {
		vars, err := parseEnvVariables(env.Variables)
		if err != nil {
			return nil, err
		}
		ec.Variables = vars
	}

	return ec, nil
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
