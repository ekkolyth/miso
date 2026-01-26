package scripts

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
)

func Dev(managerName string, scriptArgs []string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildRun("dev", scriptArgs)
	if err := core.Exec(spec, managerName); err != nil {
		return err
	}
	return nil
}
