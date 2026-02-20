package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
)

// return miso version from package.json
func GetMisoVersion() (string, error) {
	type packageInfo struct {
		Version string `json:"version"`
	}

	// search multiple package.json locations
	var packageJSONPaths []string

	// current directory
	cwd, _ := os.Getwd()
	packageJSONPaths = append(packageJSONPaths, filepath.Join(cwd, "package.json"))

	// node_modules/@ekkolyth/miso/package.json
	packageJSONPaths = append(packageJSONPaths, filepath.Join(cwd, "node_modules", "@ekkolyth", "miso", "package.json"))

	// walk up from binary location (or cwd if Executable fails)
	var startDir string
	if exe, err := os.Executable(); err == nil {
		startDir, _ = filepath.Abs(filepath.Dir(exe))
	} else {
		startDir, _ = filepath.Abs(cwd)
	}
	for i := 0; i < 10; i++ {
		pkgPath := filepath.Join(startDir, "package.json")
		if _, err := os.Stat(pkgPath); err == nil {
			packageJSONPaths = append(packageJSONPaths, pkgPath)
		}
		parent := filepath.Dir(startDir)
		if parent == startDir {
			break
		}
		startDir = parent
	}

	// try each path
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
	var managerName string
	if cfg.PackageManager != "" {
		managerName = cfg.PackageManager
	} else {
		// detect manager from lockfile
		detected, err := manager.DetectManager(root)
		if err == nil {
			managerName = detected
		}
	}

	// run manager version if available
	if managerName != "" {
		driver, ok := manager.GetManager(managerName)
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
