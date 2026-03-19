package commands

import (
	"fmt"
	"os"

	"github.com/ekkolyth/miso/internal/manager"
)

// miso upgrade
func Upgrade(local bool, args []string) error {
	var spec manager.ExecSpec
	if local {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not determine working directory: %w", err)
		}
		managerName, err := manager.DetectManager(cwd)
		if err != nil {
			return fmt.Errorf("could not detect package manager: %w", err)
		}
		driver, ok := manager.GetManager(managerName)
		if !ok {
			return fmt.Errorf("unsupported package manager: %s", managerName)
		}
		spec = driver.BuildAdd(append([]string{"@ekkolyth/miso"}, args...))
	} else {
		npmArgs := append([]string{"npm", "install", "-g", "@ekkolyth/miso"}, args...)
		spec = manager.ExecSpec{Command: "sudo", Args: npmArgs}
	}
	return manager.Exec(spec, "")
}
