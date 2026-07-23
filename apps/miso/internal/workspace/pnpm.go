package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type pnpmWorkspaceFile struct {
	Packages []string `yaml:"packages"`
}

// the raw package patterns from pnpm-workspace.yaml.
// "!"-prefixed patterns (pnpm exclusions) are returned verbatim. nil when absent.
func ReadPnpmWorkspace(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pnpm-workspace.yaml: %w", err)
	}
	var file pnpmWorkspaceFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse pnpm-workspace.yaml: %w", err)
	}
	return file.Packages, nil
}
