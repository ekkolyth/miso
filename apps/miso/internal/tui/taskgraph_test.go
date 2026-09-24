package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func writeFolderScripts(t *testing.T, dir string, names ...string) {
	t.Helper()
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		writeScript(t, scriptsDir, name)
	}
}

// writes a root package.json declaring members, and each member's package.json
func writeWorkspace(t *testing.T, root string, members map[string]string) {
	t.Helper()
	var patterns []string
	for name, pkg := range members {
		patterns = append(patterns, `"apps/`+name+`"`)
		writePackageJSONRaw(t, filepath.Join(root, "apps", name), pkg)
	}
	sort.Strings(patterns)
	writePackageJSONRaw(t, root, `{"workspaces":[`+strings.Join(patterns, ",")+`]}`)
}

// levels as sorted label sets, since entries within a level run in parallel
func levelLabels(levels [][]TuiScriptEntry) [][]string {
	var out [][]string
	for _, level := range levels {
		labels := labelsOf(level)
		sort.Strings(labels)
		out = append(out, labels)
	}
	return out
}

func assertLevels(t *testing.T, cfg config.Config, cmd, root string, want [][]string) {
	t.Helper()
	_, levels, _, ran, err := buildRun(cfg, cmd, root, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}
	if got := levelLabels(levels); !reflect.DeepEqual(got, want) {
		t.Fatalf("levels = %v, want %v", got, want)
	}
}

func issueTasks() map[string]config.TaskConfig {
	return map[string]config.TaskConfig{
		"migrate": {DependsOn: []string{"services"}},
		"dev":     {DependsOn: []string{"migrate", "build"}},
	}
}

func TestBuildRunBareDependsOnRunsNamedRootTasksInOrder(t *testing.T) {
	root := t.TempDir()
	writeFolderScripts(t, root, "services", "migrate", "build", "dev")
	cfg := config.Config{Scripts: "./scripts", Tasks: issueTasks()}

	assertLevels(t, cfg, "dev", root, [][]string{{"build", "services"}, {"migrate"}, {"dev"}})
}

func TestBuildRunBareDependsOnSimpleMode(t *testing.T) {
	root := t.TempDir()
	writeFolderScripts(t, root, "services", "migrate", "build", "dev")
	simple := false
	cfg := config.Config{Scripts: "./scripts", PackageManager: &simple, Tasks: issueTasks()}

	assertLevels(t, cfg, "dev", root, [][]string{{"build", "services"}, {"migrate"}, {"dev"}})
}

func TestBuildRunDependsOnSharedDependencyRunsOnce(t *testing.T) {
	root := t.TempDir()
	writeFolderScripts(t, root, "base", "a", "b", "dev")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"dev": {DependsOn: []string{"a", "b"}},
		"a":   {DependsOn: []string{"base"}},
		"b":   {DependsOn: []string{"base"}},
	}}

	assertLevels(t, cfg, "dev", root, [][]string{{"base"}, {"a", "b"}, {"dev"}})
}

func TestBuildRunDependsOnHashAndAtRefs(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root, map[string]string{"web": `{"name":"web"}`})
	writeFolderScripts(t, root, "gen", "dev")
	writeFolderScripts(t, filepath.Join(root, "apps", "web"), "codegen")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"dev": {DependsOn: []string{"#gen", "@web/codegen"}},
	}}

	assertLevels(t, cfg, "dev", root, [][]string{{"gen", "web"}, {"dev"}})
}

func TestBuildRunMemberDeclaredDependsOnResolvesInMember(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root, map[string]string{"web": `{"name":"web"}`, "api": `{"name":"api"}`})
	webDir := filepath.Join(root, "apps", "web")
	writeFolderScripts(t, webDir, "dev", "build")
	writeFolderScripts(t, filepath.Join(root, "apps", "api"), "dev")
	if err := os.WriteFile(filepath.Join(webDir, "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"dependsOn":["build"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts"}

	assertLevels(t, cfg, "dev", root, [][]string{{"api", "web/build"}, {"web/dev"}})
}

func TestBuildRunCaretNameRunsNamedTaskUpstream(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root, map[string]string{
		"ui":  `{"name":"ui"}`,
		"web": `{"name":"web","dependencies":{"ui":"*"}}`,
	})
	writeFolderScripts(t, filepath.Join(root, "apps", "ui"), "build", "dev")
	writeFolderScripts(t, filepath.Join(root, "apps", "web"), "build", "dev")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"dev": {DependsOn: []string{"^build"}},
	}}

	assertLevels(t, cfg, "dev", root, [][]string{{"ui/build", "ui/dev"}, {"web"}})
}

func TestBuildRunCaretSameNameKeepsPackageOrder(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root, map[string]string{
		"ui":  `{"name":"ui"}`,
		"web": `{"name":"web","dependencies":{"ui":"*"}}`,
	})
	writeFolderScripts(t, filepath.Join(root, "apps", "ui"), "build")
	writeFolderScripts(t, filepath.Join(root, "apps", "web"), "build")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"build": {DependsOn: []string{"^build"}},
	}}

	assertLevels(t, cfg, "build", root, [][]string{{"ui"}, {"web"}})
}

func TestBuildRunDependsOnUnresolvableErrors(t *testing.T) {
	root := t.TempDir()
	writeFolderScripts(t, root, "dev")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"dev": {DependsOn: []string{"ghost"}},
	}}

	_, _, _, _, err := buildRun(cfg, "dev", root, nil, nil, nil, false)
	if err == nil {
		t.Fatal("expected error for unresolvable dependsOn, got nil")
	}
	if !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "root scope") {
		t.Errorf("error %q does not name the entry and the searched location", err.Error())
	}
}

// an upstream member without the named script is skipped, not an error
func TestBuildRunCaretMissingUpstreamScriptIsSkipped(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root, map[string]string{
		"ui":  `{"name":"ui"}`,
		"web": `{"name":"web","dependencies":{"ui":"*"}}`,
	})
	writeFolderScripts(t, filepath.Join(root, "apps", "ui"), "dev")
	writeFolderScripts(t, filepath.Join(root, "apps", "web"), "dev")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"dev": {DependsOn: []string{"^ghost"}},
	}}

	assertLevels(t, cfg, "dev", root, [][]string{{"ui", "web"}})
}

func TestBuildRunDependsOnCycleErrors(t *testing.T) {
	root := t.TempDir()
	writeFolderScripts(t, root, "a", "b")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"a": {DependsOn: []string{"b"}},
		"b": {DependsOn: []string{"a"}},
	}}

	_, _, _, _, err := buildRun(cfg, "a", root, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("err = %v, want circular dependency", err)
	}
}

func TestBuildRunDependsOnCompanionClashErrors(t *testing.T) {
	root := t.TempDir()
	writeFolderScripts(t, root, "dev", "logs")
	cfg := config.Config{Scripts: "./scripts", Tasks: map[string]config.TaskConfig{
		"dev": {Concurrent: []string{"logs"}, DependsOn: []string{"logs"}},
	}}

	_, _, _, _, err := buildRun(cfg, "dev", root, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "both concurrent and dependsOn") {
		t.Fatalf("err = %v, want a concurrent/dependsOn clash", err)
	}
}
