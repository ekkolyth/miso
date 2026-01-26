package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// script info from discovery
type ScriptInfo struct {
	Path      string
	Extension string
	Basename  string
}

// discover scripts in folder, return map of basename -> []ScriptInfo
// detects extension conflicts (multiple files with same basename)
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

	// scan folder for executable files
	entries, err := os.ReadDir(scriptsPath)
	if err != nil {
		return nil, fmt.Errorf("read scripts folder: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// check if file is executable or has common script extension
		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		basename := strings.TrimSuffix(name, ext)

		// skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// include if executable or has known script extension
		isExecutable := info.Mode().Perm()&0111 != 0
		hasKnownExt := isKnownScriptExtension(ext)

		if isExecutable || hasKnownExt {
			scriptInfo := ScriptInfo{
				Path:      filepath.Join(scriptsPath, name),
				Extension: ext,
				Basename:  basename,
			}
			result[basename] = append(result[basename], scriptInfo)
		}
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

// check for extension conflicts (multiple files with same basename)
func CheckConflicts(scripts map[string][]ScriptInfo) error {
	for basename, infos := range scripts {
		if len(infos) > 1 {
			var paths []string
			for _, info := range infos {
				paths = append(paths, filepath.Base(info.Path))
			}
			return fmt.Errorf("multiple scripts named %q found: %s. use 'miso %s' or 'miso %s'",
				basename, strings.Join(paths, ", "), paths[0], paths[1])
		}
	}
	return nil
}
