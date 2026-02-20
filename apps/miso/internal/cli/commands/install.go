package commands

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
)

// miso install
func Install(managerName string, workDir string, cfg config.Config) error {
	driver, ok := manager.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildInstall()
	if flags, ok := cfg.Flags["install"]; ok {
		spec.Args = append(spec.Args, flags...)
	}
	return manager.Exec(spec, managerName, workDir)
}
