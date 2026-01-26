package pm

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
)

// miso add
func Add(managerName string, packageNames []string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildAdd(packageNames)
	if err := core.Exec(spec, managerName); err != nil {
		return err
	}
	return nil
}
