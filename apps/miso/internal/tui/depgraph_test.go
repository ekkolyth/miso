package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func writePackageJSONRaw(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}

func TestReadPackageInfo(t *testing.T) {
	dir := t.TempDir()
	writePackageJSONRaw(t, dir, `{
		"name": "@myapp/shared",
		"dependencies": { "lodash": "^4.0.0", "@myapp/utils": "workspace:*" },
		"devDependencies": { "typescript": "^5.0.0" }
	}`)

	info, err := ReadPackageInfo(dir)
	if err != nil {
		t.Fatalf("ReadPackageInfo() error: %v", err)
	}
	if info.Name != "@myapp/shared" {
		t.Errorf("Name = %q, want %q", info.Name, "@myapp/shared")
	}
	if len(info.Deps) != 3 {
		t.Errorf("Deps count = %d, want 3", len(info.Deps))
	}
}

func TestReadPackageInfoNoFile(t *testing.T) {
	dir := t.TempDir()
	info, err := ReadPackageInfo(dir)
	if err != nil {
		t.Fatalf("ReadPackageInfo() error: %v", err)
	}
	if info.Name != "" {
		t.Errorf("Name = %q, want empty", info.Name)
	}
	if len(info.Deps) != 0 {
		t.Errorf("Deps count = %d, want 0", len(info.Deps))
	}
}

func TestBuildDependencyGraph(t *testing.T) {
	root := t.TempDir()

	shared := filepath.Join(root, "packages", "shared")
	ui := filepath.Join(root, "packages", "ui")
	web := filepath.Join(root, "apps", "web")

	writePackageJSONRaw(t, shared, `{"name": "@myapp/shared"}`)
	writePackageJSONRaw(t, ui, `{"name": "@myapp/ui", "dependencies": {"@myapp/shared": "workspace:*"}}`)
	writePackageJSONRaw(t, web, `{"name": "@myapp/web", "dependencies": {"@myapp/ui": "workspace:*", "@myapp/shared": "workspace:*", "react": "^18.0.0"}}`)

	workspaces := []WorkspaceInfo{
		{Name: "shared", Dir: shared},
		{Name: "ui", Dir: ui},
		{Name: "web", Dir: web},
	}

	graph, err := BuildDependencyGraph(workspaces)
	if err != nil {
		t.Fatalf("BuildDependencyGraph() error: %v", err)
	}

	if len(graph["shared"]) != 0 {
		t.Errorf("graph[shared] = %v, want empty", graph["shared"])
	}
	if len(graph["ui"]) != 1 || graph["ui"][0] != "shared" {
		t.Errorf("graph[ui] = %v, want [shared]", graph["ui"])
	}
	if len(graph["web"]) != 2 {
		t.Errorf("graph[web] = %v, want 2 entries", graph["web"])
	}
}

func TestTopoSort(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "shared", ScriptName: "build", WorkspaceDir: "/p/shared"},
		{Label: "ui", ScriptName: "build", WorkspaceDir: "/p/ui"},
		{Label: "web", ScriptName: "build", WorkspaceDir: "/p/web"},
	}

	graph := map[string][]string{
		"shared": {},
		"ui":     {"shared"},
		"web":    {"ui", "shared"},
	}

	levels, err := TopoSort(entries, graph)
	if err != nil {
		t.Fatalf("TopoSort() error: %v", err)
	}

	if len(levels) != 3 {
		t.Fatalf("levels count = %d, want 3", len(levels))
	}
	if len(levels[0]) != 1 || levels[0][0].Label != "shared" {
		t.Errorf("level 0 = %v, want [shared]", levels[0])
	}
	if len(levels[1]) != 1 || levels[1][0].Label != "ui" {
		t.Errorf("level 1 = %v, want [ui]", levels[1])
	}
	if len(levels[2]) != 1 || levels[2][0].Label != "web" {
		t.Errorf("level 2 = %v, want [web]", levels[2])
	}
}

func TestTopoSortParallel(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "a", ScriptName: "build"},
		{Label: "b", ScriptName: "build"},
		{Label: "c", ScriptName: "build"},
	}

	graph := map[string][]string{
		"a": {},
		"b": {},
		"c": {"a", "b"},
	}

	levels, err := TopoSort(entries, graph)
	if err != nil {
		t.Fatalf("TopoSort() error: %v", err)
	}
	if len(levels) != 2 {
		t.Fatalf("levels count = %d, want 2", len(levels))
	}
	if len(levels[0]) != 2 {
		t.Errorf("level 0 count = %d, want 2 (a and b in parallel)", len(levels[0]))
	}
	if len(levels[1]) != 1 || levels[1][0].Label != "c" {
		t.Errorf("level 1 = %v, want [c]", levels[1])
	}
}

func TestTopoSortCycle(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "a", ScriptName: "build"},
		{Label: "b", ScriptName: "build"},
	}

	graph := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}

	_, err := TopoSort(entries, graph)
	if err == nil {
		t.Fatal("TopoSort() expected error for circular dependency, got nil")
	}
}

func TestTopoSortNoDeps(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "a", ScriptName: "build"},
		{Label: "b", ScriptName: "build"},
	}

	graph := map[string][]string{
		"a": {},
		"b": {},
	}

	levels, err := TopoSort(entries, graph)
	if err != nil {
		t.Fatalf("TopoSort() error: %v", err)
	}
	if len(levels) != 1 {
		t.Fatalf("levels count = %d, want 1 (all parallel)", len(levels))
	}
	if len(levels[0]) != 2 {
		t.Errorf("level 0 count = %d, want 2", len(levels[0]))
	}
}
