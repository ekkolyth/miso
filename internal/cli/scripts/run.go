package scripts

import (
	"fmt"

	"github.com/ekkolyth/miso/internal/cli/core"
	"github.com/ekkolyth/miso/internal/config"
)

// miso run <script>
func Run(managerName string, scriptName string, scriptArgs []string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildRun(scriptName, scriptArgs)
	if err := core.Exec(spec, managerName); err != nil {
		return err
	}
	return nil
}

// run multiple scripts sequentially
func RunMultiple(managerName string, scriptNames []string, scriptArgs []string) error {
	driver, ok := core.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	for _, scriptName := range scriptNames {
		spec := driver.BuildRun(scriptName, scriptArgs)
		if err := core.Exec(spec, managerName); err != nil {
			return err
		}
	}
	return nil
}

// run custom script from miso.json
func RunOverride(scriptName string, scriptArgs []string, cfg config.Config) error {
	command := cfg.Scripts[scriptName]
	if command == "" {
		return fmt.Errorf("script %q not defined in config", scriptName)
	}
	if err := core.ExecScript(command, scriptArgs); err != nil {
		return err
	}
	return nil
}
