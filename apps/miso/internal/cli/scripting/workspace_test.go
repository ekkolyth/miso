package scripting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func setupWorkspace(t *testing.T) (root string, wsDir string) {
	t.Helper()
	root = t.TempDir()
	wsDir = filepath.Join(root, "packages", "api")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// write root package.json declaring the workspace
	rootPkg := `{"workspaces": ["packages/api"]}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(rootPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, wsDir
}

func TestResolveWorkspaceScriptFromFolder(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	scriptsDir := filepath.Join(wsDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte("#!/bin/sh\necho build"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts", Repo: "miso"}

	resolved, workDir, _, err := ResolveWorkspaceScript("api", "build", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourceFolder {
		t.Errorf("Source = %v, want ScriptSourceFolder", resolved.Source)
	}
	if resolved.Path == "" {
		t.Error("Path should be non-empty (absolute path to build script)")
	}
	if workDir != wsDir {
		t.Errorf("workDir = %q, want %q", workDir, wsDir)
	}
}

func TestResolveWorkspaceScriptFromPackageJSON(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	// no scripts/ folder — only package.json
	wsPkg := `{"scripts": {"test:unit": "vitest run"}}`
	if err := os.WriteFile(filepath.Join(wsDir, "package.json"), []byte(wsPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts", Repo: "miso"}

	resolved, workDir, _, err := ResolveWorkspaceScript("api", "test:unit", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourcePackageJSON {
		t.Errorf("Source = %v, want ScriptSourcePackageJSON", resolved.Source)
	}
	if resolved.Path != "vitest run" {
		t.Errorf("Path = %q, want %q", resolved.Path, "vitest run")
	}
	if workDir != wsDir {
		t.Errorf("workDir = %q, want %q", workDir, wsDir)
	}
}

func TestResolveWorkspaceScriptNotFound(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	cfg := config.Config{Scripts: "./scripts", Repo: "miso"}

	resolved, workDir, _, err := ResolveWorkspaceScript("api", "nonexistent", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourceNone {
		t.Errorf("Source = %v, want ScriptSourceNone", resolved.Source)
	}
	if workDir != wsDir {
		t.Errorf("workDir = %q, want %q", workDir, wsDir)
	}
}

func TestResolveWorkspaceScript_HonorsMemberScriptsAndShell(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	// member overrides scripts folder to ./tasks and shell to zsh
	if err := os.WriteFile(filepath.Join(wsDir, "miso.json"),
		[]byte(`{"scripts":"./tasks","shell":"zsh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tasksDir := filepath.Join(wsDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "build.sh"), []byte("#!/bin/sh\necho build"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts", Shell: "bash", Repo: "miso"}

	resolved, workDir, shell, err := ResolveWorkspaceScript("api", "build", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourceFolder {
		t.Errorf("Source = %v, want ScriptSourceFolder (found in member tasks folder)", resolved.Source)
	}
	if workDir != wsDir {
		t.Errorf("workDir = %q, want %q", workDir, wsDir)
	}
	if shell != "zsh" {
		t.Errorf("shell = %q, want zsh (member override)", shell)
	}
}

func TestResolveWorkspaceScriptByPackageName(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	// workspace has a scoped package name
	wsPkg := `{"name": "@myorg/api", "scripts": {"build": "tsc"}}`
	if err := os.WriteFile(filepath.Join(wsDir, "package.json"), []byte(wsPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts", Repo: "miso"}

	resolved, _, _, err := ResolveWorkspaceScript("@myorg/api", "build", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourcePackageJSON {
		t.Errorf("Source = %v, want ScriptSourcePackageJSON", resolved.Source)
	}
}
