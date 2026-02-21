package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
)

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
		for _, lockfile := range manager.LockfileNames() {
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

// return usage help text
func UsageText() string {
	return strings.TrimSpace(`
Miso – the agnostic package manager

Usage:
  miso init
  miso version
  miso env
  miso upgrade [--local]
  miso completion [bash|zsh|fish]
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
