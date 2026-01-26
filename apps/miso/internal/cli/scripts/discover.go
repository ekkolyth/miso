package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// script info from discovery
type ScriptInfo struct {
	Path         string // absolute path to script file
	RelativePath string // relative path from scripts root (e.g., "publish/patch.sh")
	Extension    string
	Basename     string
	IsIndex      bool // true if this is an index.sh file
}

// discover scripts in folder recursively, return map of relative path -> []ScriptInfo
// supports subdirectories and index.sh files
// key format: "publish/patch" for "scripts/publish/patch.sh"
// key format: "publish" for "scripts/publish/index.sh"
func DiscoverScripts(scriptsPath string) (map[string][]ScriptInfo, error) {
	result := make(map[string][]ScriptInfo)

	// check if scripts folder exists
	info, err := os.Stat(scriptsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// folder doesn't exist - not an error, just return empty map
			return result, nil
		}
		return nil, fmt.Errorf("stat scripts folder: %w", err)
	}
	if !info.IsDir() {
		// not a directory - return empty map
		return result, nil
	}

	// recursively walk the scripts directory
	err = filepath.Walk(scriptsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip directories
		if info.IsDir() {
			return nil
		}

		name := info.Name()
		ext := filepath.Ext(name)
		basename := strings.TrimSuffix(name, ext)

		// skip hidden files and helper files (starting with _)
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}

		// check if file is executable or has common script extension
		isExecutable := info.Mode().Perm()&0111 != 0
		hasKnownExt := isKnownScriptExtension(ext)

		if !isExecutable && !hasKnownExt {
			return nil
		}

		// calculate relative path from scripts root
		relPath, err := filepath.Rel(scriptsPath, path)
		if err != nil {
			return fmt.Errorf("calculate relative path: %w", err)
		}

		// normalize path separators to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)

		// determine if this is an index file
		isIndex := basename == "index"

		// determine the key for this script
		// For index.sh files: "publish/index.sh" -> key "publish"
		// For regular files: "publish/patch.sh" -> key "publish/patch"
		var key string
		if isIndex {
			// index.sh: use parent directory as key
			// relPath is already normalized with ToSlash, so split by "/"
			parts := strings.Split(relPath, "/")
			if len(parts) == 1 {
				// scripts/index.sh -> key "index"
				key = "index"
			} else {
				// scripts/publish/index.sh -> key "publish"
				key = strings.Join(parts[:len(parts)-1], "/")
			}
		} else {
			// regular file: use relative path without extension as key
			key = strings.TrimSuffix(relPath, ext)
		}

		scriptInfo := ScriptInfo{
			Path:         path,
			RelativePath: relPath,
			Extension:    ext,
			Basename:     basename,
			IsIndex:      isIndex,
		}

		result[key] = append(result[key], scriptInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk scripts folder: %w", err)
	}

	return result, nil
}

// check if extension is a known script extension
func isKnownScriptExtension(ext string) bool {
	known := []string{".sh", ".bash", ".zsh", ".js", ".mjs", ".ts", ".py", ".rb", ".pl", ".lua", ".php"}
	ext = strings.ToLower(ext)
	for _, k := range known {
		if ext == k {
			return true
		}
	}
	return false
}

// check for conflicts (multiple scripts with same key)
// e.g., both "publish.sh" and "publish/index.sh" would conflict
func CheckConflicts(scripts map[string][]ScriptInfo) error {
	for key, infos := range scripts {
		if len(infos) > 1 {
			var paths []string
			for _, info := range infos {
				paths = append(paths, info.RelativePath)
			}
			return fmt.Errorf("multiple scripts for %q exist, exiting: %s",
				key, strings.Join(paths, ", "))
		}
	}
	return nil
}
