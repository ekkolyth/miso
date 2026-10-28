package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolve returns the package manager for dir: the packageManager field in
// package.json when set, otherwise the one its lockfile names.
func Resolve(dir string) (string, error) {
	data, readErr := os.ReadFile(filepath.Join(dir, "package.json"))
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return "", fmt.Errorf("read package.json: %w", readErr)
	}
	if readErr == nil {
		var pkg struct {
			PackageManager string `json:"packageManager"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			return "", fmt.Errorf("parse package.json: %w", err)
		}
		name := strings.SplitN(pkg.PackageManager, "@", 2)[0]
		if name != "" {
			if _, ok := GetManager(name); !ok {
				return "", fmt.Errorf(
					"unsupported package manager %q in package.json (supported: %s)",
					name,
					strings.Join(GetRegisteredManagers(), ", "),
				)
			}
			return name, nil
		}
	}

	return DetectManager(dir)
}
