package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/ui"
)

// readPackageJSON reads and parses package.json from root, returns nil if not found.
func readPackageJSON(root string) (map[string]interface{}, error) {
	path := filepath.Join(root, "package.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	return pkg, nil
}

// mergeWorkspaces returns a deduplicated slice that contains all patterns
// already declared in pkg["workspaces"] followed by any patterns in incoming
// that were not already present.  If pkg["workspaces"] is absent or empty the
// function simply returns incoming unchanged.
func mergeWorkspaces(pkg map[string]interface{}, incoming []string) []string {
	raw, ok := pkg["workspaces"]
	if !ok {
		return incoming
	}

	// package.json workspaces is always []interface{} after JSON round-trip.
	existing, ok := raw.([]interface{})
	if !ok || len(existing) == 0 {
		return incoming
	}

	seen := make(map[string]bool, len(existing))
	merged := make([]string, 0, len(existing)+len(incoming))

	for _, v := range existing {
		if s, ok := v.(string); ok && s != "" {
			seen[s] = true
			merged = append(merged, s)
		}
	}
	for _, p := range incoming {
		if !seen[p] {
			merged = append(merged, p)
		}
	}
	return merged
}

// writePackageJSON marshals and writes pkg back to package.json preserving indent.
func writePackageJSON(root string, pkg map[string]interface{}) error {
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode package.json: %w", err)
	}
	path := filepath.Join(root, "package.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write package.json: %w", err)
	}
	return nil
}

// packageManagerFromPackageJSON returns the bare manager name (e.g. "bun") from
// the packageManager field (e.g. "bun@1.2.3"), or "" if not set.
func packageManagerFromPackageJSON(pkg map[string]interface{}) string {
	raw, ok := pkg["packageManager"]
	if !ok {
		return ""
	}
	s, ok := raw.(string)
	if !ok || s == "" {
		return ""
	}
	return strings.SplitN(s, "@", 2)[0]
}

// injectPackageManager sets the packageManager field in package.json to
// "name@version" (e.g. "bun@1.2.3") as required by the Corepack spec and
// writes the file back to disk. If the installed version cannot be determined
// (binary not found, unexpected output, etc.) it falls back to the bare
// managerName so that init never fails solely due to a missing version.
func injectPackageManager(root string, pkg map[string]interface{}, managerName string) error {
	pkg["packageManager"] = manager.ResolveManagerVersion(managerName)
	return writePackageJSON(root, pkg)
}

// miso init
func RunInit(root string, styles ui.Styles, logger *log.Logger) error {
	// refuse if miso.json already exists
	_, err := config.Load(root)
	if err == nil {
		return fmt.Errorf("miso.json already exists")
	}
	if !errors.Is(err, config.ErrNotFound) {
		return err
	}

	printMisoWelcome(styles)

	pkg, err := readPackageJSON(root)
	if err != nil {
		return err
	}

	managerList := manager.GetRegisteredManagers()

	switch {
	// ── Case 1: existing package.json with packageManager field ─────────────
	case pkg != nil && packageManagerFromPackageJSON(pkg) != "":
		managerName := packageManagerFromPackageJSON(pkg)

		// validate it's a supported manager
		if _, ok := manager.GetManager(managerName); !ok {
			return fmt.Errorf(
				"unsupported package manager %q in package.json (supported: %s)",
				managerName,
				strings.Join(managerList, ", "),
			)
		}

		logger.Info("using package manager from package.json", "manager", managerName)

		// Ask repo type (moved here from before switch)
		repoType, err := askRepoType(styles)
		if err != nil {
			return err
		}
		var workspacePatterns []string
		if repoType == "mono" {
			workspacePatterns, err = askWorkspacePatterns(styles)
			if err != nil {
				return err
			}
		}

		if repoType == "mono" {
			merged := mergeWorkspaces(pkg, workspacePatterns)
			pkg["workspaces"] = merged
			if err := writePackageJSON(root, pkg); err != nil {
				return err
			}
			logger.Info("added workspaces to package.json")
			if err := scaffoldWorkspaceDirs(root, merged, logger); err != nil {
				return err
			}
		}

		cfg := config.Config{
			Schema:  config.SchemaURL,
			Scripts: "./scripts",
			Flags:   make(map[string][]string),
			Repo:    repoType,
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json")

	// ── Case 2: existing package.json, no packageManager; check lockfiles ───
	case pkg != nil:
		detected, _ := manager.DetectManager(root)

		// Ask repo type (moved here from before switch)
		repoType, err := askRepoType(styles)
		if err != nil {
			return err
		}
		var workspacePatterns []string
		if repoType == "mono" {
			workspacePatterns, err = askWorkspacePatterns(styles)
			if err != nil {
				return err
			}
		}

		managerName, err := selectManager(managerList, detected, styles)
		if err != nil {
			return err
		}

		if err := injectPackageManager(root, pkg, managerName); err != nil {
			return err
		}
		logger.Info("added packageManager to package.json", "manager", managerName)

		if repoType == "mono" {
			// re-read after injectPackageManager wrote it
			pkg, err = readPackageJSON(root)
			if err != nil {
				return err
			}
			merged := mergeWorkspaces(pkg, workspacePatterns)
			pkg["workspaces"] = merged
			if err := writePackageJSON(root, pkg); err != nil {
				return err
			}
			logger.Info("added workspaces to package.json")
			if err := scaffoldWorkspaceDirs(root, merged, logger); err != nil {
				return err
			}
		}

		cfg := config.Config{
			Schema:  config.SchemaURL,
			Scripts: "./scripts",
			Flags:   make(map[string][]string),
			Repo:    repoType,
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json")

	// ── Case 3: no package.json — offer new project or simple mode ───────
	default:
		var initChoice string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(styles.Heading.Render("miso could not detect an existing javascript project.")).
					Description(styles.Muted.Render("Press ↑/↓ to move, Enter to confirm • ctrl+c to bail")).
					Options(
						huh.NewOption("Create new project", "new"),
						huh.NewOption("Run in simple mode (scripts only, no package manager)", "simple"),
					).
					Value(&initChoice),
			),
		).WithTheme(huh.ThemeCharm())

		if err := form.Run(); err != nil {
			return err
		}

		if initChoice == "simple" {
			// Simple mode: scaffold miso.json with packageManager: false and create scripts dir
			f := false
			cfg := config.Config{
				Schema:         config.SchemaURL,
				PackageManager: &f,
				Scripts:        "./scripts",
			}
			if err := config.Save(root, cfg); err != nil {
				return err
			}
			logger.Info("created miso.json (simple mode)")

			scriptsDir := filepath.Join(root, "scripts")
			if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
				return fmt.Errorf("create scripts directory: %w", err)
			}
			logger.Info("created scripts directory")
			return nil
		}

		// "new" — create a new JS project
		// Now ask repo type (only relevant for JS projects)
		repoType, err := askRepoType(styles)
		if err != nil {
			return err
		}

		var workspacePatterns []string
		if repoType == "mono" {
			workspacePatterns, err = askWorkspacePatterns(styles)
			if err != nil {
				return err
			}
		}

		projectName, err := askProjectName(filepath.Base(root), styles)
		if err != nil {
			return err
		}

		managerName, err := selectManager(managerList, "", styles)
		if err != nil {
			return err
		}

		logger.Info("scaffolding new project", "name", projectName, "manager", managerName)

		if _, ok := manager.GetManager(managerName); !ok {
			return fmt.Errorf("unsupported manager: %s", managerName)
		}

		spec := manager.ExecSpec{
			Command: managerName,
			Args:    []string{"init"},
		}
		if err := manager.Exec(spec, root); err != nil {
			return err
		}

		freshPkg, err := readPackageJSON(root)
		if err != nil {
			return err
		}
		if freshPkg == nil {
			freshPkg = map[string]interface{}{
				"name":    projectName,
				"version": "0.0.1",
			}
		} else {
			freshPkg["name"] = projectName
		}
		freshPkg["packageManager"] = manager.ResolveManagerVersion(managerName)

		if repoType == "mono" {
			freshPkg["workspaces"] = workspacePatterns
		}

		if err := writePackageJSON(root, freshPkg); err != nil {
			return err
		}

		if repoType == "mono" {
			logger.Info("added workspaces to package.json")
			if err := scaffoldWorkspaceDirs(root, workspacePatterns, logger); err != nil {
				return err
			}
		}

		cfg := config.Config{
			Schema:  config.SchemaURL,
			Scripts: "./scripts",
			Flags:   make(map[string][]string),
			Repo:    repoType,
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json")
	}

	return nil
}

// askRepoType prompts the user to choose between single project or monorepo.
func askRepoType(styles ui.Styles) (string, error) {
	var repoType string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(styles.Heading.Render("What type of repository is this?")).
				Description(styles.Muted.Render("Press ↑/↓ to move, Enter to confirm • ctrl+c to bail")).
				Options(
					huh.NewOption("Single project", "single"),
					huh.NewOption("Monorepo", "mono"),
				).
				Value(&repoType),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return "", err
	}
	return repoType, nil
}

// askWorkspacePatterns prompts the user to enter workspace glob patterns.
func askWorkspacePatterns(styles ui.Styles) ([]string, error) {
	var raw string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(styles.Heading.Render("Workspace patterns")).
				Description(styles.Muted.Render("Comma-separated globs, e.g. apps/*, packages/*")).
				Placeholder("apps/*, packages/*").
				Value(&raw),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return nil, err
	}

	if strings.TrimSpace(raw) == "" {
		raw = "apps/*, packages/*"
	}

	parts := strings.Split(raw, ",")
	patterns := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no workspace patterns provided")
	}
	return patterns, nil
}

// scaffoldWorkspaceDirs creates the workspace directories derived from glob
// patterns (only the non-wildcard prefix, e.g. "apps/*" → "apps/").
func scaffoldWorkspaceDirs(root string, patterns []string, logger *log.Logger) error {
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		// strip wildcard segments to get the base directory
		parts := strings.Split(filepath.ToSlash(pattern), "/")
		var baseParts []string
		for _, p := range parts {
			if strings.ContainsAny(p, "*?[") {
				break
			}
			baseParts = append(baseParts, p)
		}
		if len(baseParts) == 0 {
			continue
		}
		dir := filepath.Join(append([]string{root}, baseParts...)...)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create workspace dir %s: %w", dir, err)
		}
		logger.Info("created workspace directory", "dir", strings.Join(baseParts, "/"))
	}
	return nil
}
