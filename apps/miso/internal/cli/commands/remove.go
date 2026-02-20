package commands

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
)

// miso remove
func Remove(managerName string, packageNames []string, workDir string, cfg config.Config) error {
	driver, ok := manager.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	args := packageNames
	if flags, ok := cfg.Flags["remove"]; ok {
		args = append(flags, packageNames...)
	}
	spec := driver.BuildRemove(args)
	return manager.Exec(spec, workDir)
}
