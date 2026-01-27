package core

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ekkolyth/miso/internal/config"
)

// miso version
func RunVersion(root string, cfg config.Config) error {
	misoVersion, err := GetMisoVersion()
	if err != nil {
		// If we can't get version, show unknown but don't fail
		misoVersion = "unknown"
	}
	fmt.Fprintf(os.Stdout, "miso %s\n", misoVersion)

	// get manager version if available
	var managerName string
	if cfg.PackageManager != "" {
		managerName = cfg.PackageManager
	} else {
		// detect manager from lockfile
		detected, err := DetectManager(root)
		if err == nil {
			managerName = detected
		}
	}

	// run manager version if available
	if managerName != "" {
		driver, ok := GetManager(managerName)
		if ok {
			spec := driver.BuildVersion()
			// run silently
			cmd := exec.Command(spec.Command, spec.Args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
	}
	return nil
}
