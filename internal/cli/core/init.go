package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/ui"
)

// miso init
func RunInit(root string, styles ui.Styles, logger *log.Logger) error {
	// Check if miso.json already exists - if so, exit early
	_, err := config.Load(root)
	if err == nil {
		return fmt.Errorf("miso.json already exists")
	}
	if !errors.Is(err, config.ErrNotFound) {
		return err
	}

	managerList := GetRegisteredManagers()
	newCfg, err := RunInitOnboarding(root, managerList, styles, logger)
	if err != nil {
		return err
	}
	if err := config.Save(root, newCfg); err != nil {
		return err
	}

	// Check if package.json already exists - if so, skip running init
	packageJsonPath := filepath.Join(root, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		logger.Info("package.json already exists, skipping manager init")
		return nil
	}

	// Run the package manager's init command
	if _, ok := GetManager(newCfg.PackageManager); !ok {
		return fmt.Errorf("unsupported manager: %s", newCfg.PackageManager)
	}
	spec := ExecSpec{
		Command: newCfg.PackageManager,
		Args:    []string{"init"},
	}
	if err := Exec(spec, newCfg.PackageManager); err != nil {
		return err
	}
	return nil
}
