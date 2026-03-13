package tui

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

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

	// expect 3 entries: app (single match) + api:dev + api:dev:worker
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), labelsOf(entries))
	}

	// entries are sorted alphabetically by label
	labels := labelsOf(entries)
	expected := []string{"api:dev", "api:dev:worker", "app"}
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
	if labelToName["api:dev"] != "dev" {
		t.Errorf("api:dev label should have script name 'dev', got %q", labelToName["api:dev"])
	}
	if labelToName["api:dev:worker"] != "dev:worker" {
		t.Errorf("api:dev:worker label should have script name 'dev:worker', got %q", labelToName["api:dev:worker"])
	}
}

// TestDiscoverTuiScripts_MultiConfig verifies DiscoverMultiScripts returns
// entries labelled with the script names from the multi array.
func TestDiscoverTuiScripts_MultiConfig(t *testing.T) {
	root := t.TempDir()
	writePackageJSON(t, root, map[string]string{
		"dev":   "vite",
		"build": "tsc",
	})

	cfg := config.Config{
		Scripts: "./scripts",
	}

	entries, err := DiscoverMultiScripts([]string{"dev", "build"}, root, cfg)
	if err != nil {
		t.Fatalf("DiscoverMultiScripts: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// labels should match script names exactly
	for _, e := range entries {
		if e.Label != e.ScriptName {
			t.Errorf("label %q != script name %q", e.Label, e.ScriptName)
		}
		if e.ScriptSource != "packagejson" {
			t.Errorf("%q: expected source 'packagejson', got %q", e.Label, e.ScriptSource)
		}
		if e.WorkspaceDir != root {
			t.Errorf("%q: expected WorkspaceDir %q, got %q", e.Label, root, e.WorkspaceDir)
		}
	}

	labels := labelsOf(entries)
	if labels[0] != "dev" {
		t.Errorf("entries[0].Label = %q, want 'dev'", labels[0])
	}
	if labels[1] != "build" {
		t.Errorf("entries[1].Label = %q, want 'build'", labels[1])
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

// labelsOf returns the Label field from each entry in order.
func labelsOf(entries []TuiScriptEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Label
	}
	return out
}
