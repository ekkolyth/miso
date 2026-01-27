package core

import "fmt"

// miso misox <package>
func RunMisox(managerName string, packageName string, args []string, workDir string) error {
	driver, ok := GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildMisox(packageName, args)
	return Exec(spec, managerName, workDir)
}
