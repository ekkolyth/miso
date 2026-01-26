package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
)

// load config from miso.json or return default if not found
func LoadConfig(root string) (config.Config, error) {
	cfg, err := config.Load(root)
	if errors.Is(err, config.ErrNotFound) {
		return config.Config{
			ProjectName: filepath.Base(root),
			Scripts:     map[string]string{},
		}, nil
	}
	if err != nil {
		return config.Config{}, err
	}
	cfg.EnsureDefaults()
	if cfg.ProjectName == "" {
		cfg.ProjectName = filepath.Base(root)
	}
	return cfg, nil
}

// ensure package manager is configured, return manager name and updated config
func EnsureManager(root string, cfg config.Config, onboardingFn func(string, []string) (config.Config, error)) (string, config.Config, error) {
	if cfg.PackageManager != "" {
		return cfg.PackageManager, cfg, nil
	}

	detected, err := DetectManager(root)
	if err == nil {
		cfg.PackageManager = detected
		if err := config.Save(root, cfg); err != nil {
			return "", cfg, err
		}
		return detected, cfg, nil
	}

	if errors.Is(err, ErrNoLockfile) {
		managerList := GetRegisteredManagers()
		newCfg, onboardingErr := onboardingFn(root, managerList)
		if onboardingErr != nil {
			return "", cfg, onboardingErr
		}
		if err := config.Save(root, newCfg); err != nil {
			return "", cfg, err
		}
		return newCfg.PackageManager, newCfg, nil
	}

	return "", cfg, err
}

// print error and exit with code 1
func Fail(logger *log.Logger, err error, showUsage bool) {
	fmt.Fprintf(os.Stderr, "ERROR miso: %s\n", err.Error())
	if showUsage {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, UsageText())
	}
	os.Exit(1)
}

// return miso version from package.json
func GetMisoVersion() (string, error) {
	type packageInfo struct {
		Version string `json:"version"`
	}

	// Try multiple locations for package.json
	var packageJSONPaths []string

	// 1. Current directory (for npm installs)
	cwd, _ := os.Getwd()
	packageJSONPaths = append(packageJSONPaths, filepath.Join(cwd, "package.json"))

	// 2. node_modules/@ekkolyth/miso/package.json (for local npm installs)
	packageJSONPaths = append(packageJSONPaths, filepath.Join(cwd, "node_modules", "@ekkolyth", "miso", "package.json"))

	// 3. Relative to binary location (for Go installs)
	// Try to find the project root by looking for package.json
	// Start from the binary location and walk up
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// Walk up to find package.json (max 10 levels)
		for i := 0; i < 10; i++ {
			pkgPath := filepath.Join(exeDir, "package.json")
			packageJSONPaths = append(packageJSONPaths, pkgPath)
			parent := filepath.Dir(exeDir)
			if parent == exeDir {
				break
			}
			exeDir = parent
		}
	}

	// 4. Try relative to source file location (for development)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(filename)
		// core/ -> cli/ -> internal/ -> project root
		projectRoot := filepath.Join(dir, "..", "..", "..")
		packageJSONPaths = append(packageJSONPaths, filepath.Join(projectRoot, "package.json"))
	}

	// Try each path
	for _, path := range packageJSONPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var info packageInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		if info.Version != "" {
			return info.Version, nil
		}
	}

	return "", fmt.Errorf("could not find package.json with version")
}

// return usage help text
func UsageText() string {
	return strings.TrimSpace(`
Miso – the agnostic package manager

Usage:
  miso init
  miso version
  miso update [--local]
  miso install
  miso add <pkg>
  miso remove <pkg>
  miso run <script> <args>
  miso dev <args>
  miso misox <package> [args...]
  miso <command> [args...]

  - Automatically detects bun, npm, pnpm, or yarn.
  - Generates a miso.json so you can override the manager or define scripts.
  - Unknown commands pass through to the detected package manager.
`)
}
