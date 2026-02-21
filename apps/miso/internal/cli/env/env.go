package env

import (
	"fmt"
	"os"
	"path/filepath"

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

// discoveryOrder is the default order to look for .env files when path is not set
var discoveryOrder = []string{
	".env.local",
	".env.production",
	".env.development",
	".env",
}

// Run executes the miso env command: resolve paths, load .env, validate, output
func Run(projectRoot string, cfg config.Config, logger *log.Logger) error {
	// Resolve paths
	paths, loadedPath, err := resolvePaths(projectRoot, cfg.Env)
	if err != nil {
		return err
	}

	// Load env files
	envMap, err := loadEnvFiles(projectRoot, paths)
	if err != nil {
		return err
	}

	// When no env config or no variables: just report where we loaded from
	if cfg.Env == nil || (len(cfg.Env.Variables.Object) == 0 && len(cfg.Env.Variables.Array) == 0) {
		logger.Info("env loaded at", "path", loadedPath)
		return nil
	}

	// Validate
	if len(cfg.Env.Variables.Array) > 0 {
		// Presence-only mode
		for _, key := range cfg.Env.Variables.Array {
			if _, ok := envMap[key]; !ok {
				return fmt.Errorf("missing required variable: %s", key)
			}
		}
		logger.Info("env validation passed", "variables", len(cfg.Env.Variables.Array))
		return nil
	}

	// Object mode - type validation
	if err := validateVariables(envMap, cfg.Env.Variables.Object, cfg.Env.Required); err != nil {
		return err
	}
	logger.Info("env validation passed", "variables", len(cfg.Env.Variables.Object))
	return nil
}

func resolvePaths(projectRoot string, envCfg *config.EnvConfig) ([]string, string, error) {
	if envCfg != nil && len(envCfg.Path) > 0 {
		// Use configured paths - fail fast on missing
		paths := make([]string, len(envCfg.Path))
		for i, p := range envCfg.Path {
			paths[i] = filepath.Join(projectRoot, p)
			if _, err := os.Stat(paths[i]); err != nil {
				if os.IsNotExist(err) {
					return nil, "", fmt.Errorf("env file not found: %s", paths[i])
				}
				return nil, "", fmt.Errorf("env file %s: %w", paths[i], err)
			}
		}
		return paths, paths[0], nil
	}

	// Discovery order
	for _, name := range discoveryOrder {
		p := filepath.Join(projectRoot, name)
		if _, err := os.Stat(p); err == nil {
			return []string{p}, p, nil
		}
	}

	tried := ""
	for i, n := range discoveryOrder {
		if i > 0 {
			tried += ", "
		}
		tried += filepath.Join(projectRoot, n)
	}
	return nil, "", fmt.Errorf("no .env file found (tried: %s)", tried)
}

func loadEnvFiles(projectRoot string, paths []string) (map[string]string, error) {
	envMap, err := godotenv.Read(paths...)
	if err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}
	return envMap, nil
}
