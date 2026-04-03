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

func TestDeduplicateLabels(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "app", ScriptName: "dev", WorkspaceDir: "/ws/app"},
		{Label: "app", ScriptName: "services", WorkspaceDir: "/ws/app"},
		{Label: "docker", ScriptName: "services", WorkspaceDir: "/ws/docker"},
	}

	result := DeduplicateLabels(entries)

	labels := labelsOf(result)
	expected := map[string]bool{
		"app:dev":      true,
		"app:services": true,
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

// labelsOf returns the Label field from each entry in order.
func labelsOf(entries []TuiScriptEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Label
	}
	return out
}
