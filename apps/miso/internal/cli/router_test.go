package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

// minimal cfg helpers — both non-delegated (miso mode); the repo field no longer
// distinguishes mono vs single, so the scenario lives in each test's name.
func monoCfg() config.Config   { return config.Config{Repo: "miso"} }
func singleCfg() config.Config { return config.Config{Repo: "miso"} }

func TestParseDevConflictErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "dev.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"dev":"turbo run dev"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// turbo mode, not simple — resolution checks both folder and package.json
	cfg := config.Config{Repo: "turbo", Scripts: "./scripts"}
	if _, err := ParseCLI([]string{"dev"}, cfg, root); err == nil {
		t.Fatal("expected error for \"dev\" defined in both scripts folder and package.json")
	}
}

func TestParseAtWorkspaceScript(t *testing.T) {
	parsed, err := ParseCLI([]string{"@api/test"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Action != ActionWorkspaceScript {
		t.Errorf("Action = %v, want ActionWorkspaceScript", parsed.Action)
	}
	if parsed.WorkspaceName != "api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "api")
	}
	if parsed.ScriptName != "test" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "test")
	}
}

func TestParseAtWorkspaceColonScript(t *testing.T) {
	parsed, err := ParseCLI([]string{"@api/test:unit"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.WorkspaceName != "api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "api")
	}
	if parsed.ScriptName != "test:unit" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "test:unit")
	}
}

func TestParseAtScopedWorkspace(t *testing.T) {
	parsed, err := ParseCLI([]string{"@myorg/api/build"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.WorkspaceName != "myorg/api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "myorg/api")
	}
	if parsed.ScriptName != "build" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "build")
	}
}

func TestParseAtWorkspaceMissingSlashErrors(t *testing.T) {
	_, err := ParseCLI([]string{"@api"}, monoCfg(), t.TempDir())
	if err == nil {
		t.Error("expected error for @workspace with no slash, got nil")
	}
}

func TestParseAtPathWorkspace(t *testing.T) {
	parsed, err := ParseCLI([]string{"@packages/api/build"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.WorkspaceName != "packages/api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "packages/api")
	}
	if parsed.ScriptName != "build" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "build")
	}
}

func TestParseAtWorkspaceScriptInSingleRepo(t *testing.T) {
	parsed, err := ParseCLI([]string{"@api/test"}, singleCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Action != ActionWorkspaceScript {
		t.Errorf("Action = %v, want ActionWorkspaceScript (should work outside mono mode)", parsed.Action)
	}
}

func TestParseColonScriptNotWorkspaceRoutedInMono(t *testing.T) {
	// test:unit in monorepo should NOT be ActionWorkspaceScript
	// (it will fall through to ResolveScript which returns ScriptSourceNone for an empty dir,
	// then passthrough — important thing is it's NOT ActionWorkspaceScript)
	parsed, err := ParseCLI([]string{"test:unit"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Action == ActionWorkspaceScript {
		t.Errorf("Action = ActionWorkspaceScript, want anything else (colon-named script should not be workspace-routed)")
	}
}

func TestParseMisoxFallsThroughToPassthrough(t *testing.T) {
	// typed `miso misox foo` is not a miso command — the standalone misox binary
	// dispatches via argv[0], so here it must route to passthrough, not misox.
	parsed, err := ParseCLI([]string{"misox", "foo"}, singleCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Action != ActionPassthrough {
		t.Errorf("Action = %v, want ActionPassthrough", parsed.Action)
	}
	if parsed.Command != "misox" {
		t.Errorf("Command = %q, want %q", parsed.Command, "misox")
	}
}

func TestParseCLI_SimpleModeIgnoresPackageJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"scripts":{"foo":"echo hi"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pm := false
	cfg := config.Config{PackageManager: &pm, Scripts: "./scripts"} // SimpleMode

	parsed, err := ParseCLI([]string{"foo"}, cfg, root)
	if err != nil {
		t.Fatalf("ParseCLI() error: %v", err)
	}
	if parsed.Action != ActionPassthrough {
		t.Errorf("Action = %v, want ActionPassthrough (simple mode must not resolve package.json)", parsed.Action)
	}
}

func TestParseCLI_NonSimpleResolvesPackageJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"scripts":{"foo":"echo hi"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts"} // PackageManager nil → not simple

	parsed, err := ParseCLI([]string{"foo"}, cfg, root)
	if err != nil {
		t.Fatalf("ParseCLI() error: %v", err)
	}
	if parsed.Action != ActionScriptPackageJSON {
		t.Errorf("Action = %v, want ActionScriptPackageJSON", parsed.Action)
	}
}
