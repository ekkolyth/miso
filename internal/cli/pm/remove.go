package pm

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
)

// miso remove
func Remove(managerName string, packageNames []string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildRemove(packageNames)
	if err := core.Exec(spec, managerName); err != nil {
		return err
	}
	return nil
}
