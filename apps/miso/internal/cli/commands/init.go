package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// injectPackageManager sets the packageManager field in package.json to the
// bare manager name and writes the file back to disk.
func injectPackageManager(root string, pkg map[string]interface{}, managerName string) error {
	pkg["packageManager"] = managerName
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

		cfg := config.Config{
			Schema:  config.SchemaURL,
			Scripts: "./scripts",
			Flags:   make(map[string][]string),
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json")

	// ── Case 2: existing package.json, no packageManager; check lockfiles ───
	case pkg != nil:
		detected, _ := manager.DetectManager(root)

		managerName, err := selectManager(managerList, detected, styles)
		if err != nil {
			return err
		}

		if err := injectPackageManager(root, pkg, managerName); err != nil {
			return err
		}
		logger.Info("added packageManager to package.json", "manager", managerName)

		cfg := config.Config{
			Schema:  config.SchemaURL,
			Scripts: "./scripts",
			Flags:   make(map[string][]string),
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json")

	// ── Case 3: no package.json — new project ───────────────────────────────
	default:
		projectName, err := askProjectName(filepath.Base(root), styles)
		if err != nil {
			return err
		}

		managerName, err := selectManager(managerList, "", styles)
		if err != nil {
			return err
		}

		logger.Info("scaffolding new project", "name", projectName, "manager", managerName)

		// validate manager is supported
		if _, ok := manager.GetManager(managerName); !ok {
			return fmt.Errorf("unsupported manager: %s", managerName)
		}

		// run <manager> init to scaffold package.json (and whatever else the
		// manager sets up, e.g. tsconfig for bun)
		spec := manager.ExecSpec{
			Command: managerName,
			Args:    []string{"init"},
		}
		if err := manager.Exec(spec, root); err != nil {
			return err
		}

		// read the package.json the manager just created and inject name +
		// packageManager so they live in one place
		freshPkg, err := readPackageJSON(root)
		if err != nil {
			return err
		}
		if freshPkg == nil {
			// manager init didn't produce a package.json; create a minimal one
			freshPkg = map[string]interface{}{
				"name":    projectName,
				"version": "0.0.1",
			}
		} else {
			freshPkg["name"] = projectName
		}
		freshPkg["packageManager"] = managerName
		if err := writePackageJSON(root, freshPkg); err != nil {
			return err
		}

		cfg := config.Config{
			Schema:  config.SchemaURL,
			Scripts: "./scripts",
			Flags:   make(map[string][]string),
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json")
	}

	return nil
}
