package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

func skipScanDir(name string) bool {
	return name == "node_modules" || strings.HasPrefix(name, ".")
}

// depth 1 then depth 2, so a parent is listed before its children
func scanDirs(root string) []string {
	depthOne := childDirs(root)
	dirs := slices.Clone(depthOne)
	for _, parent := range depthOne {
		dirs = append(dirs, childDirs(parent)...)
	}
	return dirs
}

func childDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var children []string
	for _, entry := range entries {
		if !entry.IsDir() || skipScanDir(entry.Name()) {
			continue
		}
		children = append(children, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(children)
	return children
}
