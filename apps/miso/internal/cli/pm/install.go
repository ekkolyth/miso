package pm

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
	"github.com/ekkolyth/miso/internal/config"
)

// miso install
func Install(managerName string, workDir string, cfg config.Config) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildInstall()
	if flags, ok := cfg.Flags["install"]; ok {
		spec.Args = append(spec.Args, flags...)
	}
	if err := core.Exec(spec, managerName, workDir); err != nil {
		return err
	}
	return nil
}
