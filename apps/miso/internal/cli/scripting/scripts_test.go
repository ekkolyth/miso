package scripting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestResolveScriptFolderOnlyIgnoresPackageJSON(t *testing.T) {
	dir := t.TempDir()
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkgJSON := `{"scripts": {"build": "echo building"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts"}
	cfg.EnsureDefaults()
	resolved, err := ResolveScriptFolderOnly("build", dir, cfg)
	if err != nil {
		t.Fatalf("ResolveScriptFolderOnly() error: %v", err)
	}
	if resolved.Source != ScriptSourceNone {
		t.Errorf("Source = %v, want ScriptSourceNone (package.json should be ignored)", resolved.Source)
	}
}

func TestResolveScriptFolderOnlyFindsFolderScript(t *testing.T) {
	dir := t.TempDir()
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte("#!/bin/sh\necho build"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts"}
	cfg.EnsureDefaults()
	resolved, err := ResolveScriptFolderOnly("build", dir, cfg)
	if err != nil {
		t.Fatalf("ResolveScriptFolderOnly() error: %v", err)
	}
	if resolved.Source != ScriptSourceFolder {
		t.Errorf("Source = %v, want ScriptSourceFolder", resolved.Source)
	}
}

func TestListNamesIncludesTurboTasks(t *testing.T) {
	dir := t.TempDir()

	turboJSON := `{"tasks": {"build": {}, "lint": {}, "test": {}}}`
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(turboJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgJSON := `{"scripts": {"dev": "turbo dev", "build": "turbo run build"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Repo: "turbo"}

	names, err := ListNames(dir, cfg)
	if err != nil {
		t.Fatalf("ListNames() error: %v", err)
	}

	want := map[string]bool{"build": true, "dev": true, "lint": true, "test": true}
	got := make(map[string]bool)
	for _, n := range names {
		got[n] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing expected name %q in ListNames result: %v", name, names)
		}
	}
	if len(names) != len(got) {
		t.Errorf("ListNames has duplicates: %v", names)
	}
}
