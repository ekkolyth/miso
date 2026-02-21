package scripting

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

// discover scripts recursively
func DiscoverScripts(scriptsPath string) (map[string][]ScriptInfo, error) {
	result := make(map[string][]ScriptInfo)

	// check if scripts folder exists
	info, err := os.Stat(scriptsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("stat scripts folder: %w", err)
	}
	if !info.IsDir() {
		return result, nil
	}

	err = filepath.Walk(scriptsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		name := info.Name()
		ext := filepath.Ext(name)
		basename := strings.TrimSuffix(name, ext)

		// skip hidden and helper files
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}

		isExecutable := info.Mode().Perm()&0111 != 0
		hasKnownExt := isKnownScriptExtension(ext)

		if !isExecutable && !hasKnownExt {
			return nil
		}

		relPath, err := filepath.Rel(scriptsPath, path)
		if err != nil {
			return fmt.Errorf("calculate relative path: %w", err)
		}

		relPath = filepath.ToSlash(relPath)

		isIndex := basename == "index"

		var key string
		if isIndex {
			parts := strings.Split(relPath, "/")
			if len(parts) == 1 {
				key = "index"
			} else {
				key = strings.Join(parts[:len(parts)-1], "/")
			}
		} else {
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

// check for script conflicts
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
