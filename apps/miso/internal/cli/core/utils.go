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

var lockfiles = []string{
	"bun.lockb",
	"bun.lock",
	"package-lock.json",
	"pnpm-lock.yaml",
	"yarn.lock",
}

// find project root by walking up directories
// priority: miso.json > lockfile > node_modules
func FindProjectRoot(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	current := absPath
	root := filepath.VolumeName(absPath) + string(filepath.Separator)
	if root == string(filepath.Separator) {
		root = "/"
	}

	var foundLockfile bool
	var foundNodeModules bool
	var projectRoot string

	for {
		// check for miso.json (highest priority)
		misoPath := filepath.Join(current, config.FileName)
		if st, err := os.Stat(misoPath); err == nil && !st.IsDir() {
			return current, nil
		}

		// check for lockfiles
		for _, lockfile := range lockfiles {
			lockPath := filepath.Join(current, lockfile)
			if st, err := os.Stat(lockPath); err == nil && !st.IsDir() {
				foundLockfile = true
				if projectRoot == "" {
					projectRoot = current
				}
				break
			}
		}

		// check for node_modules
		nodeModulesPath := filepath.Join(current, "node_modules")
		if st, err := os.Stat(nodeModulesPath); err == nil && st.IsDir() {
			foundNodeModules = true
			if projectRoot == "" {
				projectRoot = current
			}
		}

		// stop at filesystem root
		if current == root || current == filepath.Dir(current) {
			break
		}

		current = filepath.Dir(current)
	}

	// if we found lockfile or node_modules but no miso.json
	if foundLockfile || foundNodeModules {
		return "", errors.New("miso is not currently initialized in this project. run 'miso init' from the project root to get started")
	}

	// nothing found
	return "", errors.New("no project found. miso was not able to locate a miso.json, node_modules folder, or lockfile in the parent tree. run 'miso init' from the project root to get started")
}

// load config from miso.json
func LoadConfig(projectRoot string) (config.Config, error) {
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return config.Config{}, err
	}
	cfg.EnsureDefaults()
	if cfg.ProjectName == "" {
		cfg.ProjectName = filepath.Base(projectRoot)
	}
	return cfg, nil
}

// ensure package manager is configured, return manager name and config
func EnsureManager(root string, cfg config.Config) (string, config.Config, error) {
	if cfg.PackageManager == "" {
		return "", cfg, errors.New("package manager not configured in miso.json")
	}
	return cfg.PackageManager, cfg, nil
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
  miso upgrade [--local]
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
