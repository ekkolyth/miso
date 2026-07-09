package cli

import (
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

// minimal cfg helpers
func monoCfg() config.Config   { return config.Config{Repo: "mono"} }
func singleCfg() config.Config { return config.Config{Repo: "single"} }

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
