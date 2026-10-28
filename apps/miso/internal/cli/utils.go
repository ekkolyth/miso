package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/cli/env"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/ui"
	"github.com/ekkolyth/miso/internal/workspace"
)

// find project root by walking up directories
// priority: miso.json > lockfile > node_modules. A member's own miso.json
// resolves to the workspace root that lists it, when that root has a miso.json
// and isn't in standalone mode
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
			return enclosingWorkspaceRoot(current), nil
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

// the nearest ancestor with a miso.json whose workspace has dir as a member,
// else dir. Only the member's own directory counts: a project nested deeper
// inside a member (a test fixture, say) keeps its own root
func enclosingWorkspaceRoot(dir string) string {
	for ancestor := filepath.Dir(dir); ancestor != filepath.Dir(ancestor); ancestor = filepath.Dir(ancestor) {
		// standalone mode has no workspace discovery, so it never claims a member
		if ancestorCfg, err := config.Load(ancestor); err != nil || ancestorCfg.StandaloneMode() {
			continue
		}
		members, err := workspace.DiscoverMembers(ancestor, config.Config{})
		if err != nil {
			continue
		}
		for _, member := range members {
			if filepath.Clean(member.Dir) == dir {
				return ancestor
			}
		}
	}
	return dir
}

// load config from miso.json
func LoadConfig(projectRoot string) (config.Config, error) {
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return config.Config{}, err
	}
	cfg.EnsureDefaults()
	return cfg, nil
}

// ensure package manager is configured, return manager name and config.
// Resolution order:
//  1. package.json packageManager field
//  2. lockfile detection
func EnsureManager(root string, cfg config.Config) (string, config.Config, error) {
	name, err := manager.Resolve(root)
	if errors.Is(err, manager.ErrNoLockfile) || errors.Is(err, manager.ErrMultipleLockfiles) {
		return "", cfg, errors.New("could not determine package manager: add a packageManager field to package.json, or run 'miso init'")
	}
	if err != nil {
		return "", cfg, err
	}
	return name, cfg, nil
}

// print styled error and exit with code 1
func Fail(_ *log.Logger, err error, showUsage bool) {
	s := ui.Default()
	var envErr *env.ValidationError
	if errors.As(err, &envErr) {
		// blank line sets the block apart from `miso env`'s passing-entry lines
		fmt.Fprintln(os.Stderr)
	}
	fmt.Fprintf(os.Stderr, "%s %s %s\n", s.Error.Render("ERROR"), s.Muted.Render("miso:"), err.Error())
	if envErr != nil {
		envErr.Render(os.Stderr)
	}
	if showUsage {
		// UsageText owns its own leading/trailing spacing
		fmt.Fprint(os.Stderr, UsageText())
	}
	os.Exit(1)
}

// return styled usage help text
func UsageText() string {
	s := ui.Default()
	var b strings.Builder
	b.WriteString("\n") // one line above the whole thing
	b.WriteString(s.Heading.Render("Miso") + " – the agnostic package manager\n\n")
	b.WriteString(s.Label.Render("Usage:") + "\n")
	for _, sub := range []string{
		"init",
		"version",
		"env",
		"upgrade [--local]",
		"completion [bash|zsh|fish]",
		"install",
		"add <pkg>",
		"remove <pkg>",
		"run <script> <args>",
		"dev <args>",
		"<command> [args...]",
	} {
		b.WriteString("  " + s.Flavor.Render("miso") + " " + sub + "\n")
	}
	b.WriteString("\n")
	for _, note := range []string{
		"Miso automatically detects your package manager (bun, pnpm, npm, yarn)",
		"Configure miso for your repo using miso.json",
	} {
		b.WriteString("  " + s.Muted.Render("- "+note) + "\n")
	}
	b.WriteString("\n\n") // two line breaks under the grey text
	return b.String()
}
