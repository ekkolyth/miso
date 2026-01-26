package pm

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
	"github.com/ekkolyth/miso/internal/config"
)

// miso add
func Add(managerName string, packageNames []string, workDir string, cfg config.Config) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	args := packageNames
	if flags, ok := cfg.Flags["add"]; ok {
		args = append(flags, packageNames...)
	}
	spec := driver.BuildAdd(args)
	if err := core.Exec(spec, managerName, workDir); err != nil {
		return err
	}
	return nil
}
