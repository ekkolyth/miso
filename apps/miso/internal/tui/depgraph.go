package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PackageInfo holds the name and dependency names from a workspace's package.json.
type PackageInfo struct {
	Name string
	Deps []string // combined dependencies + devDependencies package names
}

// ReadPackageInfo reads a workspace's package.json and extracts name and dependency names.
// Returns empty PackageInfo (not an error) if package.json doesn't exist.
func ReadPackageInfo(dir string) (PackageInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return PackageInfo{}, nil
		}
		return PackageInfo{}, fmt.Errorf("read package.json in %s: %w", dir, err)
	}

	var pkg struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return PackageInfo{}, fmt.Errorf("parse package.json in %s: %w", dir, err)
	}

	var deps []string
	for name := range pkg.Dependencies {
		deps = append(deps, name)
	}
	for name := range pkg.DevDependencies {
		deps = append(deps, name)
	}

	return PackageInfo{Name: pkg.Name, Deps: deps}, nil
}

// BuildDependencyGraph reads package.json from each workspace and builds an adjacency
// list mapping workspace names to the workspace names they depend on.
// Only dependencies that match another workspace's package name are included.
func BuildDependencyGraph(workspaces []WorkspaceInfo) (map[string][]string, error) {
	type wsInfo struct {
		ws      WorkspaceInfo
		pkgInfo PackageInfo
	}
	var infos []wsInfo
	pkgNameToWs := make(map[string]string)

	for _, ws := range workspaces {
		info, err := ReadPackageInfo(ws.Dir)
		if err != nil {
			return nil, err
		}
		infos = append(infos, wsInfo{ws: ws, pkgInfo: info})
		pkgName := info.Name
		if pkgName == "" {
			pkgName = ws.Name
		}
		pkgNameToWs[pkgName] = ws.Name
	}

	graph := make(map[string][]string)
	for _, info := range infos {
		var localDeps []string
		for _, dep := range info.pkgInfo.Deps {
			if wsName, ok := pkgNameToWs[dep]; ok {
				localDeps = append(localDeps, wsName)
			}
		}
		graph[info.ws.Name] = localDeps
	}

	return graph, nil
}

// TopoSort sorts entries into execution levels based on the dependency graph.
// Each level is a slice of entries that can run in parallel.
// Returns error if a circular dependency is detected.
func TopoSort(entries []TuiScriptEntry, graph map[string][]string) ([][]TuiScriptEntry, error) {
	entryMap := make(map[string]TuiScriptEntry)
	for _, e := range entries {
		entryMap[e.Label] = e
	}

	inDegree := make(map[string]int)
	for _, e := range entries {
		inDegree[e.Label] = 0
	}
	for node, deps := range graph {
		if _, ok := inDegree[node]; !ok {
			continue
		}
		for _, dep := range deps {
			if _, ok := inDegree[dep]; ok {
				inDegree[node]++
			}
		}
	}

	var levels [][]TuiScriptEntry
	remaining := len(entries)

	for remaining > 0 {
		var level []TuiScriptEntry
		for _, e := range entries {
			if deg, ok := inDegree[e.Label]; ok && deg == 0 {
				level = append(level, e)
			}
		}

		if len(level) == 0 {
			return nil, fmt.Errorf("circular dependency detected")
		}

		for _, e := range level {
			delete(inDegree, e.Label)
			remaining--
			for node, deps := range graph {
				if _, ok := inDegree[node]; !ok {
					continue
				}
				for _, dep := range deps {
					if dep == e.Label {
						inDegree[node]--
					}
				}
			}
		}

		levels = append(levels, level)
	}

	return levels, nil
}
