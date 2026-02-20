package cli

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/manager"
)

// miso misox <package>
func RunMisox(managerName string, packageName string, args []string, workDir string) error {
	driver, ok := manager.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildMisox(packageName, args)
	return manager.Exec(spec, managerName, workDir)
}
