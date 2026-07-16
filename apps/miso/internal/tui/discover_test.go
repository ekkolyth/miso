package tui

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/config"
)

// writePackageJSON writes a minimal package.json with the given scripts map to dir.
func writePackageJSON(t *testing.T, dir string, scripts map[string]string) {
	t.Helper()
	content := `{"scripts":{`
	first := true
	for k, v := range scripts {
		if !first {
			content += ","
		}
		content += `"` + k + `":"` + v + `"`
		first = false
	}
	content += `}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}

// writeScript creates a shell script file at scriptsDir/name.sh with executable permission.
func writeScript(t *testing.T, scriptsDir, name string) string {
	t.Helper()
	path := filepath.Join(scriptsDir, name+".sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho hello\n"), 0o755); err != nil {
		t.Fatalf("write script %s: %v", path, err)
	}
	return path
}

// TestDiscoverTuiScripts_PrefixMatching verifies that prefix matching works
// across two workspaces, producing correct labels for single and multiple matches.
func TestDiscoverTuiScripts_PrefixMatching(t *testing.T) {
	// workspace A: has only "dev" in package.json
	wsADir := t.TempDir()
	writePackageJSON(t, wsADir, map[string]string{
		"dev":   "vite",
		"build": "vite build",
	})

	// workspace B: has "dev" and "dev:worker" in package.json
	wsBDir := t.TempDir()
	writePackageJSON(t, wsBDir, map[string]string{
		"dev":        "next dev",
		"dev:worker": "node worker.js",
		"build":      "next build",
	})

	workspaces := []WorkspaceInfo{
		{Name: "app", Dir: wsADir},
		{Name: "api", Dir: wsBDir},
	}

	entries, err := DiscoverTuiScripts("dev", workspaces, "./scripts")
	if err != nil {
		t.Fatalf("DiscoverTuiScripts: %v", err)
	}

	// expect 3 entries: app (single match) + api/dev + api/dev:worker
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), labelsOf(entries))
	}

	// entries are sorted alphabetically by label
	labels := labelsOf(entries)
	expected := []string{"api/dev", "api/dev:worker", "app"}
	sort.Strings(expected)
	for i, want := range expected {
		if labels[i] != want {
			t.Errorf("entry[%d]: label = %q, want %q", i, labels[i], want)
		}
	}

	// verify script names
	labelToName := make(map[string]string)
	for _, e := range entries {
		labelToName[e.Label] = e.ScriptName
	}
	if labelToName["app"] != "dev" {
		t.Errorf("app label should have script name 'dev', got %q", labelToName["app"])
	}
	if labelToName["api/dev"] != "dev" {
		t.Errorf("api/dev label should have script name 'dev', got %q", labelToName["api/dev"])
	}
	if labelToName["api/dev:worker"] != "dev:worker" {
		t.Errorf("api/dev:worker label should have script name 'dev:worker', got %q", labelToName["api/dev:worker"])
	}
}

// TestDiscoverEntries_CarriesScopedMemberName verifies a member whose package.json
// name is scoped (@org/web) surfaces as label "@org/web", not the dir basename.
func TestDiscoverEntries_CarriesScopedMemberName(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "package.json"),
		[]byte(`{"name":"@org/web","scripts":{"dev":"vite"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) == 0 || entries[0].Label != "@org/web" {
		t.Fatalf("label = %v, want @org/web", labelsOf(entries))
	}
}

// TestDiscoverWorkspaceScripts_WorkspaceNameSurvivesLabelDedup verifies that a
// member with two matching scripts carries the member name in WorkspaceName
// even though Label is rewritten to "member/scriptName".
func TestDiscoverWorkspaceScripts_WorkspaceNameSurvivesLabelDedup(t *testing.T) {
	wsDir := t.TempDir()
	writePackageJSON(t, wsDir, map[string]string{
		"dev":      "next dev",
		"dev:next": "next dev --turbo",
	})

	ws := WorkspaceInfo{Name: "web", Dir: wsDir}
	entries, err := discoverWorkspaceScripts("dev", ws, "./scripts")
	if err != nil {
		t.Fatalf("discoverWorkspaceScripts: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), labelsOf(entries))
	}
	for _, e := range entries {
		if e.Label == ws.Name {
			t.Errorf("Label = %q, want deduped (multiple matches in this member)", e.Label)
		}
		if e.WorkspaceName != ws.Name {
			t.Errorf("WorkspaceName = %q, want %q (label = %q)", e.WorkspaceName, ws.Name, e.Label)
		}
	}
}

func TestResolveSingleRepoScripts(t *testing.T) {
	root := t.TempDir()
	writePackageJSON(t, root, map[string]string{
		"dev":   "vite",
		"build": "tsc",
	})

	cfg := config.Config{
		Scripts: "./scripts",
	}

	entries, err := ResolveSingleRepoScripts([]string{"dev", "build"}, root, cfg)
	if err != nil {
		t.Fatalf("ResolveSingleRepoScripts: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	for _, e := range entries {
		if e.Label != e.ScriptName {
			t.Errorf("label %q != script name %q", e.Label, e.ScriptName)
		}
		if e.ScriptSource != "packagejson" {
			t.Errorf("%q: expected source 'packagejson', got %q", e.Label, e.ScriptSource)
		}
	}
}

func TestResolveSingleRepoScripts_SkipsMissing(t *testing.T) {
	root := t.TempDir()
	writePackageJSON(t, root, map[string]string{
		"dev": "vite",
	})

	cfg := config.Config{
		Scripts: "./scripts",
	}

	entries, err := ResolveSingleRepoScripts([]string{"dev", "nonexistent"}, root, cfg)
	if err != nil {
		t.Fatalf("ResolveSingleRepoScripts: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (missing script skipped), got %d", len(entries))
	}
	if entries[0].ScriptName != "dev" {
		t.Errorf("expected 'dev', got %q", entries[0].ScriptName)
	}
}

// TestDiscoverTuiScripts_ScriptsFolder verifies that a script discovered in the
// scripts folder is reported with source "folder".
func TestDiscoverTuiScripts_ScriptsFolder(t *testing.T) {
	wsDir := t.TempDir()

	// create the scripts subdirectory and write a script
	scriptsDir := filepath.Join(wsDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	scriptPath := writeScript(t, scriptsDir, "dev")

	workspaces := []WorkspaceInfo{
		{Name: "myapp", Dir: wsDir},
	}

	entries, err := DiscoverTuiScripts("dev", workspaces, "./scripts")
	if err != nil {
		t.Fatalf("DiscoverTuiScripts: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), labelsOf(entries))
	}

	e := entries[0]
	if e.Label != "myapp" {
		t.Errorf("Label = %q, want 'myapp'", e.Label)
	}
	if e.ScriptSource != "folder" {
		t.Errorf("ScriptSource = %q, want 'folder'", e.ScriptSource)
	}
	if e.ScriptPath != scriptPath {
		t.Errorf("ScriptPath = %q, want %q", e.ScriptPath, scriptPath)
	}
	if e.ScriptName != "dev" {
		t.Errorf("ScriptName = %q, want 'dev'", e.ScriptName)
	}
}

// TestDiscoverEntries_HonorsMemberScriptsFolder verifies a member whose miso.json
// sets scripts:"./tasks" has its scripts discovered from <member>/tasks, not the
// root default of ./scripts.
func TestDiscoverEntries_HonorsMemberScriptsFolder(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	tasksDir := filepath.Join(webDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "miso.json"),
		[]byte(`{"scripts":"./tasks"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := writeScript(t, tasksDir, "dev")

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), labelsOf(entries))
	}
	e := entries[0]
	if e.ScriptSource != "folder" {
		t.Errorf("ScriptSource = %q, want folder", e.ScriptSource)
	}
	if e.ScriptPath != scriptPath {
		t.Errorf("ScriptPath = %q, want %q (member tasks folder)", e.ScriptPath, scriptPath)
	}
}

// TestDiscoverEntries_HonorsMemberShell verifies a member whose miso.json sets
// shell:"zsh" produces an entry stamped with that shell, overriding the root shell.
func TestDiscoverEntries_HonorsMemberShell(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	scriptsDir := filepath.Join(webDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "miso.json"),
		[]byte(`{"shell":"zsh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeScript(t, scriptsDir, "dev")

	cfg := config.Config{Scripts: "./scripts", Shell: "bash"}
	entries, err := discoverEntries(cfg, "dev", root)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), labelsOf(entries))
	}
	if entries[0].Shell != "zsh" {
		t.Errorf("Shell = %q, want zsh (member override)", entries[0].Shell)
	}
}

// TestEntryLabelUsesPackageNameForm verifies multi-script members produce
// "@scope/pkg/script" labels (not "@scope/pkg:script"), and a member with a
// single matching script uses the bare package name.
func TestEntryLabelUsesPackageNameForm(t *testing.T) {
	multiDir := t.TempDir()
	writePackageJSON(t, multiDir, map[string]string{
		"dev":      "next dev",
		"dev:next": "next dev --turbo",
	})

	singleDir := t.TempDir()
	writePackageJSON(t, singleDir, map[string]string{
		"dev": "vite",
	})

	workspaces := []WorkspaceInfo{
		{Name: "@ekko/web", Dir: multiDir},
		{Name: "@ekko/single", Dir: singleDir},
	}

	entries, err := DiscoverTuiScripts("dev", workspaces, "./scripts")
	if err != nil {
		t.Fatalf("DiscoverTuiScripts: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), labelsOf(entries))
	}

	labels := labelsOf(entries)
	sort.Strings(labels)
	expected := []string{"@ekko/single", "@ekko/web/dev", "@ekko/web/dev:next"}
	sort.Strings(expected)
	for i, want := range expected {
		if labels[i] != want {
			t.Errorf("entry[%d]: label = %q, want %q", i, labels[i], want)
		}
	}
}

func TestDeduplicateLabels(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "app", ScriptName: "dev", WorkspaceDir: "/ws/app"},
		{Label: "app", ScriptName: "services", WorkspaceDir: "/ws/app"},
		{Label: "docker", ScriptName: "services", WorkspaceDir: "/ws/docker"},
	}

	result := DeduplicateLabels(entries)

	labels := labelsOf(result)
	expected := map[string]bool{
		"app/dev":      true,
		"app/services": true,
		"docker":       true,
	}
	if len(labels) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(labels), labels)
	}
	for _, l := range labels {
		if !expected[l] {
			t.Errorf("unexpected label %q, expected one of %v", l, expected)
		}
	}
}

func TestDeduplicateLabels_NoDuplicates(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "app", ScriptName: "dev", WorkspaceDir: "/ws/app"},
		{Label: "api", ScriptName: "dev", WorkspaceDir: "/ws/api"},
	}

	result := DeduplicateLabels(entries)
	labels := labelsOf(result)
	if labels[0] != "app" || labels[1] != "api" {
		t.Errorf("labels changed unexpectedly: %v", labels)
	}
}

// TestDiscoverTuiScriptsErrorsWhenScriptInBothSources verifies a member
// defining the same script name in both scripts/ and package.json errors
// instead of silently picking the folder script.
func TestDiscoverTuiScriptsErrorsWhenScriptInBothSources(t *testing.T) {
	dir := t.TempDir()
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, scriptsDir, "dev")
	writePackageJSON(t, dir, map[string]string{"dev": "vite"})

	_, err := DiscoverTuiScripts("dev", []WorkspaceInfo{{Name: "web", Dir: dir}}, "./scripts")
	if err == nil {
		t.Fatal("expected ambiguous-script error when dev is defined in both sources")
	}
	if !errors.Is(err, scripting.ErrAmbiguousScript) {
		t.Errorf("error = %v, want ErrAmbiguousScript", err)
	}
}

// TestDiscoverEntriesRootScriptWinsOverFanOut verifies a script resolving at
// the root wins over fan-out, even when a member also defines it.
func TestDiscoverEntriesRootScriptWinsOverFanOut(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"],"scripts":{"dev":"echo root"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, webDir, map[string]string{"dev": "vite"})

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 root entry (no fan-out), got %d: %v", len(entries), labelsOf(entries))
	}
	if entries[0].WorkspaceDir != root {
		t.Errorf("entry dir = %q, want root %q", entries[0].WorkspaceDir, root)
	}
}

// labelsOf returns the Label field from each entry in order.
func labelsOf(entries []TuiScriptEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Label
	}
	return out
}
