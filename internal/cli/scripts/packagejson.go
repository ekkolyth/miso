package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// read scripts from package.json
func ReadPackageJSONScripts(root string) (map[string]string, error) {
	packageJSONPath := filepath.Join(root, "package.json")
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			// package.json doesn't exist - not an error, return empty map
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("read package.json: %w", err)
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}

	if pkg.Scripts == nil {
		return make(map[string]string), nil
	}

	return pkg.Scripts, nil
}
