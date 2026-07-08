package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
)

type Member struct {
	Name       string
	Dir        string
	ConfigPath string
}

// DiscoverMembers resolves the PM-native workspace member list. pnpm-workspace.yaml
// wins when present; otherwise package.json "workspaces". Independent of repo mode.
func DiscoverMembers(root string, cfg config.Config) ([]Member, error) {
	patterns, err := workspacePatterns(root)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return nil, nil
	}

	var includes, excludes []string
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			excludes = append(excludes, strings.TrimPrefix(pattern, "!"))
		} else {
			includes = append(includes, pattern)
		}
	}

	seen := make(map[string]bool)
	var members []Member
	for _, pattern := range includes {
		glob := pattern
		if !filepath.IsAbs(glob) {
			glob = filepath.Join(root, pattern)
		}
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, fmt.Errorf("expand workspace glob %q: %w", pattern, err)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			if seen[match] || excluded(match, root, excludes) {
				continue
			}
			seen[match] = true
			members = append(members, newMember(match))
		}
	}
	return members, nil
}

// workspacePatterns prefers pnpm-workspace.yaml; falls back to package.json workspaces.
func workspacePatterns(root string) ([]string, error) {
	if patterns, err := ReadPnpmWorkspace(root); err != nil {
		return nil, err
	} else if len(patterns) > 0 {
		return patterns, nil
	}
	return readPackageJSONWorkspaces(root)
}

func readPackageJSONWorkspaces(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	var pkg struct {
		Workspaces []string `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	return pkg.Workspaces, nil
}

// excluded reports whether dir matches any "!"-exclusion (filepath.Match; no "**").
func excluded(dir, root string, excludes []string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, ex := range excludes {
		ex = strings.TrimSuffix(strings.TrimPrefix(ex, "./"), "/")
		if ok, _ := filepath.Match(ex, rel); ok {
			return true
		}
		if ok, _ := filepath.Match(ex, filepath.Base(dir)); ok {
			return true
		}
	}
	return false
}

func newMember(dir string) Member {
	name := readPackageJSONName(dir)
	if name == "" {
		name = filepath.Base(dir)
	}
	configPath := filepath.Join(dir, config.FileName)
	if _, err := os.Stat(configPath); err != nil {
		configPath = ""
	}
	return Member{Name: name, Dir: dir, ConfigPath: configPath}
}

func readPackageJSONName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Name
}
