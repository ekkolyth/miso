package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/ui"
	"github.com/ekkolyth/miso/internal/workspace"
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

	apps, err := workspace.Detect(root)
	if err != nil {
		return err
	}

	if len(apps) > 0 {
		selected, err := askApps(apps, styles)
		if err != nil {
			return err
		}
		if len(selected) > 0 {
			chosen, err := askAppManagers(selected, styles)
			if err != nil {
				return err
			}
			if _, err := writePackageManagers(selected, chosen); err != nil {
				return err
			}
			written, err := writeConfigs(root, selected)
			for _, path := range written {
				logger.Info("created miso.json", "path", path)
			}
			return err
		}
	}

	var initChoice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(styles.Heading.Render(fallbackTitle(len(apps) > 0))).
				Description(styles.Muted.Render("Press ↑/↓ to move, Enter to confirm • ctrl+c to bail")).
				Options(fallbackOptions(fileExists(filepath.Join(root, "package.json")))...).
				Value(&initChoice),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeCharm))

	if err := form.Run(); err != nil {
		return err
	}

	if initChoice == "standalone" {
		f := false
		cfg := config.Config{
			Schema:         config.SchemaURL,
			PackageManager: &f,
			Scripts:        "./scripts",
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json (standalone mode)")

		scriptsDir := filepath.Join(root, "scripts")
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			return fmt.Errorf("create scripts directory: %w", err)
		}
		logger.Info("created scripts directory")
		return nil
	}

	// "new" — create a new JS project
	repoType, err := askRepoType(styles)
	if err != nil {
		return err
	}

	var workspacePatterns []string
	var orchestration string
	if repoType == "mono" {
		workspacePatterns, err = askWorkspacePatterns(styles)
		if err != nil {
			return err
		}
		orchestration, err = askOrchestration(styles)
		if err != nil {
			return err
		}
	}

	projectName, err := askProjectName(filepath.Base(root), styles)
	if err != nil {
		return err
	}

	managerName, err := selectManager(manager.GetRegisteredManagers(), "", "Pick your package manager", styles)
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
	}
	cfg.Repo = RepoFieldForOrchestration(orchestration)
	if err := config.Save(root, cfg); err != nil {
		return err
	}
	logger.Info("created miso.json")

	return nil
}

func fallbackTitle(foundAny bool) string {
	if foundAny {
		return "No apps selected. How do you want to set up miso?"
	}
	return "miso didn't find any apps here. How do you want to set up miso?"
}

// "Create new project" would run over an existing package.json
func fallbackOptions(hasPackageJSON bool) []huh.Option[string] {
	standalone := huh.NewOption("Run in standalone mode (scripts only, no package manager)", "standalone")
	if hasPackageJSON {
		return []huh.Option[string]{standalone}
	}
	return []huh.Option[string]{huh.NewOption("Create new project", "new"), standalone}
}

func appLine(app workspace.DetectedApp, styles ui.Styles) string {
	line := app.RelPath + " — " + app.Label
	switch {
	case app.Badge != "":
		line += " " + ui.Label(ui.LabelColors[1], app.Badge)
	case app.NoManager:
		line += " " + styles.Muted.Render("no manager found")
	}
	return line
}

// lists workspace apps, which always run, and asks which Non-Workspace apps to
// include; keeps Detect's order
func askApps(apps []workspace.DetectedApp, styles ui.Styles) ([]workspace.DetectedApp, error) {
	var workspaceLines []string
	var otherOptions []huh.Option[int]
	for index, app := range apps {
		if app.InWorkspace {
			workspaceLines = append(workspaceLines, "  "+appLine(app, styles))
		} else {
			otherOptions = append(otherOptions, huh.NewOption(appLine(app, styles), index).Selected(true))
		}
	}

	fields := []huh.Field{
		huh.NewNote().Title(styles.Heading.Render(fmt.Sprintf("miso found %d apps:", len(apps)))),
	}
	if len(workspaceLines) > 0 {
		fields = append(fields, huh.NewNote().
			Title("Workspace (always run):").
			Description(strings.Join(workspaceLines, "\n")))
	}
	var picked []int
	if len(otherOptions) > 0 {
		fields = append(fields, huh.NewMultiSelect[int]().
			Title("Non-Workspace — pick the ones to run with miso:").
			Description(styles.Muted.Render("space to toggle • ctrl+a to toggle all • Enter to confirm • ctrl+c to bail")).
			Options(otherOptions...).
			Value(&picked))
	} else {
		fields = append(fields, huh.NewNote().
			Description(styles.Muted.Render("Press Enter to set up these apps • ctrl+c to bail")))
	}

	form := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeFunc(huh.ThemeCharm))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return selectedApps(apps, picked), nil
}

// every workspace app plus the picked Non-Workspace ones (indexes into apps),
// in apps' order
func selectedApps(apps []workspace.DetectedApp, picked []int) []workspace.DetectedApp {
	selected := make([]workspace.DetectedApp, 0, len(apps))
	for index, app := range apps {
		if app.InWorkspace || slices.Contains(picked, index) {
			selected = append(selected, app)
		}
	}
	return selected
}

// asks for a package manager for each selected app that has none
func askAppManagers(apps []workspace.DetectedApp, styles ui.Styles) (map[string]string, error) {
	chosen := map[string]string{}
	for _, app := range apps {
		if !app.NoManager {
			continue
		}
		name, err := selectManager(manager.GetRegisteredManagers(), "", "Pick a package manager for "+app.RelPath, styles)
		if err != nil {
			return nil, err
		}
		chosen[app.RelPath] = name
	}
	return chosen, nil
}

// sets packageManager in the package.json of each NoManager app that has a
// choice in chosen (keyed by RelPath); the only package.json init touches
func writePackageManagers(apps []workspace.DetectedApp, chosen map[string]string) ([]string, error) {
	var written []string
	for _, app := range apps {
		name, ok := chosen[app.RelPath]
		if !app.NoManager || !ok {
			continue
		}
		pkg, err := readPackageJSON(app.Dir)
		if err != nil {
			return written, fmt.Errorf("%s: %w", app.RelPath, err)
		}
		if pkg == nil {
			continue
		}
		pkg["packageManager"] = manager.ResolveManagerVersion(name)
		if err := writePackageJSON(app.Dir, pkg); err != nil {
			return written, fmt.Errorf("%s: %w", app.RelPath, err)
		}
		written = append(written, app.RelPath+"/package.json")
	}
	return written, nil
}

// members first and the root last, so a failure partway never leaves a root
// miso.json over a half-configured tree
func writeConfigs(root string, apps []workspace.DetectedApp) ([]string, error) {
	var written []string
	for _, app := range apps {
		if app.RelPath == "." || app.HasConfig {
			continue
		}
		jsApp := app.Label == "Typescript" || app.Label == "Javascript"
		if jsApp && app.InWorkspace {
			continue
		}
		cfg := config.Config{Schema: config.SchemaURL, Scripts: "./scripts"}
		if !app.HasPackageJSON {
			noManager := false
			cfg.PackageManager = &noManager
		}
		if err := config.Save(app.Dir, cfg); err != nil {
			return written, err
		}
		written = append(written, app.RelPath+"/"+config.FileName)
	}

	cfg := config.Config{
		Schema:  config.SchemaURL,
		Scripts: "./scripts",
		Flags:   map[string][]string{},
	}
	if !fileExists(filepath.Join(root, "package.json")) {
		noManager := false
		cfg.PackageManager = &noManager
	}
	switch {
	case fileExists(filepath.Join(root, "turbo.json")):
		cfg.Repo = "turbo"
	case fileExists(filepath.Join(root, "nx.json")):
		cfg.Repo = "nx"
	}
	if err := config.Save(root, cfg); err != nil {
		return written, err
	}
	return append(written, config.FileName), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	).WithTheme(huh.ThemeFunc(huh.ThemeCharm))

	if err := form.Run(); err != nil {
		return "", err
	}
	return repoType, nil
}

// RepoFieldForOrchestration maps a monorepo's chosen orchestration engine to
// the miso.json "repo" field: empty (omitted, defaults to miso) for miso or
// an unset choice, the engine name for turbo/nx.
func RepoFieldForOrchestration(orchestration string) string {
	if orchestration == "turbo" || orchestration == "nx" {
		return orchestration
	}
	return ""
}

// askOrchestration prompts a monorepo for its task-orchestration engine: miso
// itself (default), or delegating to an existing turbo/nx setup.
func askOrchestration(styles ui.Styles) (string, error) {
	var orchestration string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(styles.Heading.Render("How should tasks be orchestrated?")).
				Description(styles.Muted.Render("Press ↑/↓ to move, Enter to confirm • ctrl+c to bail")).
				Options(
					huh.NewOption("miso (default)", "miso"),
					huh.NewOption("turbo", "turbo"),
					huh.NewOption("nx", "nx"),
				).
				Value(&orchestration),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeCharm))

	if err := form.Run(); err != nil {
		return "", err
	}
	return orchestration, nil
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
	).WithTheme(huh.ThemeFunc(huh.ThemeCharm))

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
