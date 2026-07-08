package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ekkolyth/miso/internal/manager"
)

// true when the binary was launched under the misox name (standalone npx stand-in)
func InvokedAsMisox(argv0 string) bool {
	base := filepath.Base(argv0)
	return base == "misox" || strings.HasPrefix(base, "misox-")
}

// misox <package>
func RunMisox(managerName string, packageName string, args []string, workDir string) error {
	driver, ok := manager.GetManager(managerName)
	if !ok {
		return fmt.Errorf("unsupported manager: %s", managerName)
	}
	spec := driver.BuildMisox(packageName, args)
	return manager.Exec(spec, workDir)
}
