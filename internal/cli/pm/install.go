package pm

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
)

// miso install
func Install(managerName string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildInstall()
	if err := core.Exec(spec, managerName); err != nil {
		return err
	}
	return nil
}
