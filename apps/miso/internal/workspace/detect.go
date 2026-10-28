package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager"
)

type DetectedApp struct {
	Dir            string
	RelPath        string
	Label          string
	Badge          string
	NoManager      bool
	HasPackageJSON bool
	InWorkspace    bool
	HasConfig      bool
}

// workspace apps first, then the rest, each sorted by RelPath
func Detect(root string) ([]DetectedApp, error) {
	globDirs, excludes, err := globMemberDirs(root)
	if err != nil {
		return nil, err
	}
	patterns, err := workspacePatterns(root)
	if err != nil {
		return nil, err
	}
	declaresWorkspaces := len(patterns) > 0

	var apps []DetectedApp
	candidates := append([]string{root}, scanDirs(root)...)
	for _, dir := range candidates {
		isRoot := dir == root
		if !isRoot && excluded(dir, root, excludes) {
			continue
		}
		if insideApp(dir, apps) {
			continue
		}
		if isRoot && declaresWorkspaces {
			continue
		}
		label, ok := appLabel(dir)
		if !ok {
			continue
		}
		relPath := "."
		if !isRoot {
			rel, err := filepath.Rel(root, dir)
			if err != nil {
				return nil, err
			}
			relPath = filepath.ToSlash(rel)
		}
		app := DetectedApp{
			Dir:            dir,
			RelPath:        relPath,
			Label:          label,
			HasPackageJSON: fileExists(filepath.Join(dir, "package.json")),
			InWorkspace:    slices.Contains(globDirs, dir),
			HasConfig:      fileExists(filepath.Join(dir, config.FileName)),
		}
		if app.HasPackageJSON {
			name, err := manager.Resolve(dir)
			switch {
			case err == nil:
				app.Badge = name
			case errors.Is(err, manager.ErrNoLockfile):
				app.NoManager = true
			default:
				return nil, fmt.Errorf("%s: %w", relPath, err)
			}
		} else if label == "Python" {
			app.Badge = pythonBadge(dir)
		}
		apps = append(apps, app)
	}

	sort.SliceStable(apps, func(i, j int) bool {
		if apps[i].InWorkspace != apps[j].InWorkspace {
			return apps[i].InWorkspace
		}
		return apps[i].RelPath < apps[j].RelPath
	})
	return apps, nil
}

func insideApp(dir string, apps []DetectedApp) bool {
	for _, app := range apps {
		rel, err := filepath.Rel(app.Dir, dir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ok is false for a directory with no marker, and for a group (go.work, or a
// solution with no project of its own) whose children are found by the scan
func appLabel(dir string) (label string, ok bool) {
	has := func(names ...string) bool {
		for _, name := range names {
			if fileExists(filepath.Join(dir, name)) {
				return true
			}
		}
		return false
	}
	hasGlob := func(pattern string) bool {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		return err == nil && len(matches) > 0
	}

	if has("go.work") {
		return "", false
	}
	if hasGlob("*.sln") && !hasGlob("*.csproj") && !hasGlob("*.fsproj") {
		return "", false
	}

	switch {
	case has("composer.json") && has("artisan"):
		return "Laravel", true
	case hasGlob("*.csproj"):
		return "C#", true
	case hasGlob("*.fsproj"):
		return "F#", true
	case has("go.mod"):
		return "Go", true
	case has("Cargo.toml"):
		return "Rust", true
	case has("pom.xml", "build.gradle", "build.gradle.kts"):
		return "Java/Kotlin", true
	case has("pyproject.toml", "requirements.txt"):
		return "Python", true
	case has("package.json") && has("tsconfig.json"):
		return "Typescript", true
	case has("package.json"):
		return "Javascript", true
	}
	return "", false
}

func pythonBadge(dir string) string {
	switch {
	case fileExists(filepath.Join(dir, "uv.lock")):
		return "uv"
	case fileExists(filepath.Join(dir, "poetry.lock")):
		return "poetry"
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
