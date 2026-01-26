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

	// Try to get manager version if available, but don't trigger onboarding
	var managerName string
	if cfg.PackageManager != "" {
		managerName = cfg.PackageManager
	} else {
		// Try to detect manager from lockfile, but don't trigger onboarding
		detected, err := DetectManager(root)
		if err == nil {
			managerName = detected
		}
	}

	// If we have a manager, try to run its version command
	if managerName != "" {
		driver, ok := GetManager(managerName)
		if ok {
			spec := driver.BuildVersion()
			// Run version command silently (no logging)
			cmd := exec.Command(spec.Command, spec.Args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run() // Silently ignore errors
		}
	}
	return nil
}
