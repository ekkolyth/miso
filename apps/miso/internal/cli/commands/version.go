package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
)

// return miso version from package.json
func GetMisoVersion() (string, error) {
	type packageInfo struct {
		Version string `json:"version"`
	}

	// Walk up from the binary's own location to find miso's package.json.
	// We never look in CWD node_modules — miso version must always reflect
	// the version of the running binary, not whatever happens to be installed
	// as a local dependency in the current project.
	var packageJSONPaths []string

	var startDir string
	if exe, err := os.Executable(); err == nil {
		startDir, _ = filepath.Abs(filepath.Dir(exe))
	} else {
		return "", fmt.Errorf("could not determine binary location")
	}
	for i := 0; i < 10; i++ {
		pkgPath := filepath.Join(startDir, "package.json")
		packageJSONPaths = append(packageJSONPaths, pkgPath)
		parent := filepath.Dir(startDir)
		if parent == startDir {
			break
		}
		startDir = parent
	}

	// try each path in order
	for _, path := range packageJSONPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var info packageInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		if info.Version != "" {
			return info.Version, nil
		}
	}

	return "", fmt.Errorf("could not find package.json with version")
}

// miso version
func RunVersion(root string, cfg config.Config) error {
	misoVersion, err := GetMisoVersion()
	if err != nil {
		// If we can't get version, show unknown but don't fail
		misoVersion = "unknown"
	}
	fmt.Fprintf(os.Stdout, "miso %s\n", misoVersion)

	// get manager version if available
	managerName, err := manager.DetectManager(root)
	if err != nil {
		managerName = ""
	}

	// run manager version if available
	if managerName != "" {
		driver, ok := manager.GetManager(managerName)
		if ok {
			spec := driver.BuildVersion()
			if err := manager.Exec(spec, root); err != nil {
				return err
			}
		}
	}
	return nil
}
