package pm

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
	"github.com/ekkolyth/miso/internal/config"
)

// miso remove
func Remove(managerName string, packageNames []string, workDir string, cfg config.Config) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	args := packageNames
	if flags, ok := cfg.Flags["remove"]; ok {
		args = append(flags, packageNames...)
	}
	spec := driver.BuildRemove(args)
	if err := core.Exec(spec, managerName, workDir); err != nil {
		return err
	}
	return nil
}
