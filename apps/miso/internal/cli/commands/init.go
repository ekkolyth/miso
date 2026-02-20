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

// detect manager for init preselection
// checks package.json packageManager field and lockfiles
func DetectManagerForInit(root string) (string, error) {
	// check package.json for packageManager field
	packageJSONPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		var pkg struct {
			PackageManager string `json:"packageManager"`
		}
		if err := json.Unmarshal(data, &pkg); err == nil && pkg.PackageManager != "" {
			// extract manager name (before @)
			parts := strings.Split(pkg.PackageManager, "@")
			if len(parts) > 0 && parts[0] != "" {
				managerName := parts[0]
				// validate against registered managers
				registered := manager.GetRegisteredManagers()
				for _, reg := range registered {
					if reg == managerName {
						return managerName, nil
					}
				}
				// unsupported manager
				return "", fmt.Errorf("unsupported package manager '%s' specified in package.json. supported managers: %v", managerName, registered)
			}
		}
	}

	// check for lockfiles
	detected, err := manager.DetectManager(root)
	if err == nil {
		return detected, nil
	}

	// neither found - return empty string (no error)
	return "", nil
}

// miso init
func RunInit(root string, styles ui.Styles, logger *log.Logger) error {
	// check if miso.json exists
	_, err := config.Load(root)
	if err == nil {
		return fmt.Errorf("miso.json already exists")
	}
	if !errors.Is(err, config.ErrNotFound) {
		return err
	}

	// display ASCII art and welcome message
	printMisoWelcome(styles)

	// detect manager for preselection
	preselectedManager, err := DetectManagerForInit(root)
	if err != nil {
		return err
	}

	managerList := manager.GetRegisteredManagers()
	newCfg, err := RunInitOnboarding(root, managerList, preselectedManager, styles, logger)
	if err != nil {
		return err
	}
	if err := config.Save(root, newCfg); err != nil {
		return err
	}

	// skip manager init if package.json exists
	packageJsonPath := filepath.Join(root, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		logger.Info("package.json already exists, skipping manager init")
		return nil
	}

	// run manager init command
	if _, ok := manager.GetManager(newCfg.PackageManager); !ok {
		return fmt.Errorf("unsupported manager: %s", newCfg.PackageManager)
	}
	spec := manager.ExecSpec{
		Command: newCfg.PackageManager,
		Args:    []string{"init"},
	}
	if err := manager.Exec(spec, ""); err != nil {
		return err
	}
	return nil
}
