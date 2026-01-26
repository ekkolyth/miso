package pm

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
	"github.com/ekkolyth/miso/internal/config"
)

// miso run <script>
func Run(managerName string, scriptName string, scriptArgs []string, workDir string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildRun(scriptName, scriptArgs)
	if err := core.Exec(spec, managerName, workDir); err != nil {
		return err
	}
	return nil
}

// run multiple scripts sequentially
func RunMultiple(managerName string, scriptNames []string, scriptArgs []string, workDir string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	for _, scriptName := range scriptNames {
		spec := driver.BuildRun(scriptName, scriptArgs)
		if err := core.Exec(spec, managerName, workDir); err != nil {
			return err
		}
	}
	return nil
}

// miso dev
func Dev(managerName string, scriptArgs []string, workDir string, cfg config.Config) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	args := scriptArgs
	if flags, ok := cfg.Flags["dev"]; ok {
		args = append(flags, scriptArgs...)
	}
	spec := driver.BuildRun("dev", args)
	if err := core.Exec(spec, managerName, workDir); err != nil {
		return err
	}
	return nil
}
