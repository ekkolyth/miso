package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager/bun"
)

func TestFilterEntriesByWorkspace(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "web", WorkspaceName: "@ekko/web"},
		{Label: "api", WorkspaceName: "@ekko/api"},
	}
	got := filterEntriesByWorkspace(entries, []string{"@ekko/web"})
	if len(got) != 1 || got[0].WorkspaceName != "@ekko/web" {
		t.Errorf("got %v, want only @ekko/web", got)
	}
	if len(filterEntriesByWorkspace(entries, nil)) != 2 {
		t.Error("nil filter keeps all entries")
	}
}

func TestFolderSpawn_ShellScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.sh")
	if err := os.WriteFile(path, []byte("echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, args, err := folderSpawn(path, "", "")
	if err != nil {
		t.Fatalf("folderSpawn() error: %v", err)
	}
	if cmd != "sh" {
		t.Errorf("cmd = %q, want sh (not the old hardcode path only)", cmd)
	}
	// args must end with the script path and carry -e for the shell.
	if len(args) != 2 || args[0] != "-e" || args[1] != path {
		t.Errorf("args = %v, want [-e %s]", args, path)
	}
}

// TestBuildRunRootScriptArgsReachScript is the regression fence for the Task 8
// arg-drop: a root single-script run must thread its invocation args through to
// the spawned script instead of dropping them.
func TestBuildRunRootScriptArgsReachScript(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "echo-args.sh"), []byte("echo \"got:[$@]\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "echo-args", root, nil, nil, []string{"staging"})
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	var buf bytes.Buffer
	if _, err := RunPlain(pm, &buf, levels, concurrentProcs); err != nil {
		t.Fatalf("RunPlain: %v", err)
	}

	if out := buf.String(); !strings.Contains(out, "got:[staging]") {
		t.Errorf("script args did not reach the script; output:\n%s", out)
	}
}

// TestBuildRunPackageJSONScriptArgsReachScript is the regression fence for the
// package.json spawn path: a root single-script run resolved from
// package.json (not a scripts/ folder file) must thread its invocation args
// into mgr.BuildRun instead of dropping them.
func TestBuildRunPackageJSONScriptArgsReachScript(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"scripts":{"deploy":"echo deploying"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	pm, _, _, ran, err := buildRun(cfg, "deploy", root, bun.Bun{}, nil, []string{"staging"})
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	proc := pm.findProc("deploy")
	if proc == nil {
		t.Fatal("no process for label \"deploy\"")
		return
	}
	gotCommand, gotArgs := proc.Command, proc.Args

	want := bun.Bun{}.BuildRun("deploy", []string{"staging"})
	if gotCommand != want.Command {
		t.Errorf("Command = %q, want %q", gotCommand, want.Command)
	}
	if !reflect.DeepEqual(gotArgs, want.Args) {
		t.Errorf("Args = %v, want %v", gotArgs, want.Args)
	}
}

// TestBuildRunConcurrentCompanionDoesNotReceiveArgs verifies that when a root
// script has a concurrent companion, invocation args reach only the main entry
// — the companion always spawns arg-less.
func TestBuildRunConcurrentCompanionDoesNotReceiveArgs(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "dev.sh"), []byte("echo \"main:[$@]\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "worker.sh"), []byte("echo \"conc:[$@]\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"worker"}}},
	}
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, []string{"staging"})
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	var buf bytes.Buffer
	if _, err := RunPlain(pm, &buf, levels, concurrentProcs); err != nil {
		t.Fatalf("RunPlain: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "main:[staging]") {
		t.Errorf("main entry did not receive args; output:\n%s", out)
	}
	if !strings.Contains(out, "conc:[]") {
		t.Errorf("concurrent companion did not run arg-less; output:\n%s", out)
	}
	if strings.Contains(out, "conc:[staging]") {
		t.Errorf("concurrent companion leaked the main entry's args; output:\n%s", out)
	}
}

// TestBuildRunMemberFanOutDropsArgs verifies that when a script has no root
// resolution and fans out to 2+ members, invocation args are dropped rather
// than broadcast to every member — which member they'd apply to is ambiguous.
func TestBuildRunMemberFanOutDropsArgs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web","apps/api"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"web", "api"} {
		scriptsDir := filepath.Join(root, "apps", member, "scripts")
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "echo \"" + member + ":[$@]\"\n"
		if err := os.WriteFile(filepath.Join(scriptsDir, "dev.sh"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Config{Scripts: "./scripts"}
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, []string{"staging"})
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	var buf bytes.Buffer
	if _, err := RunPlain(pm, &buf, levels, concurrentProcs); err != nil {
		t.Fatalf("RunPlain: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web:[]") || !strings.Contains(out, "api:[]") {
		t.Errorf("expected both fan-out members to run arg-less; output:\n%s", out)
	}
	if strings.Contains(out, "staging") {
		t.Errorf("fan-out entries unexpectedly received args; output:\n%s", out)
	}
}

// TestBuildRunMemberFanOutNoConfigMemberDropsRootConcurrent fences the
// EffectiveConfig no-config-early-return bug: a member with no miso.json must
// not inherit root's task list either, else a root concurrent broadcasts into
// every fan-out member even though that member never declared it.
func TestBuildRunMemberFanOutNoConfigMemberDropsRootConcurrent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webScripts := filepath.Join(root, "apps", "web", "scripts")
	if err := os.MkdirAll(webScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webScripts, "dev.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webScripts, "services.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// apps/web has no miso.json of its own — ConfigPath resolves empty.

	// root declares a concurrent for "dev" but has no root dev script, so
	// resolution falls straight through to member fan-out.
	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"services"}}},
	}
	pm, _, _, ran, err := buildRun(cfg, "dev", root, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	// single unambiguous match in the member keeps the bare workspace label —
	// DeduplicateLabels only splits it to "web/dev" when a "web/services"
	// companion also lands, which the fix must prevent.
	if pm.findProc("web") == nil {
		t.Fatal("expected fan-out entry web")
	}
	if proc := pm.findProc("web/services"); proc != nil {
		t.Errorf("no-config member spawned root's \"services\" concurrent it never declared: %+v", proc.Entry)
	}
	if len(pm.Processes) != 1 {
		var got []string
		for _, p := range pm.Processes {
			got = append(got, p.Entry.Label)
		}
		t.Errorf("Processes = %v, want only [web]", got)
	}
}

// TestClassifyEntriesPartitionsByMarker verifies the split keys on IsConcurrent,
// keeping unmarked entries in the main partition and marked ones out of it.
func TestClassifyEntriesPartitionsByMarker(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "web/dev", ScriptName: "dev"},
		{Label: "web/worker", ScriptName: "worker", IsConcurrent: true},
		{Label: "api", ScriptName: "dev"},
	}
	main, concurrent := classifyEntries(entries)
	if got := labelsOf(main); len(got) != 2 || got[0] != "web/dev" || got[1] != "api" {
		t.Fatalf("main = %v, want [web/dev api]", got)
	}
	if got := labelsOf(concurrent); len(got) != 1 || got[0] != "web/worker" {
		t.Fatalf("concurrent = %v, want [web/worker]", got)
	}
}

// TestBuildRunMemberFanOutConcurrentCompanionExemptFromDependsOn fences the
// dependsOn-split misclassification: a member's own concurrent companion
// (injected during fan-out, absent from the root concurrent list) must be
// classified concurrent — never sorted into the dependency levels the topo
// sort would then block on.
func TestBuildRunMemberFanOutConcurrentCompanionExemptFromDependsOn(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webScripts := filepath.Join(root, "apps", "web", "scripts")
	if err := os.MkdirAll(webScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webScripts, "dev.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webScripts, "worker.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "web", "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"concurrent":["worker"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Root declares a ^dev dependsOn (no root concurrent list) — this triggers
	// the buildRun split. The member's "worker" companion is in no root
	// concurrent list, so only the IsConcurrent marker can classify it.
	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {DependsOn: []string{"^dev"}}},
	}
	_, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	if len(concurrentProcs) != 1 || concurrentProcs[0].Entry.Label != "web/worker" {
		var got []string
		for _, p := range concurrentProcs {
			got = append(got, p.Entry.Label)
		}
		t.Fatalf("concurrentProcs = %v, want [web/worker]", got)
	}
	for _, level := range levels {
		for _, e := range level {
			if e.Label == "web/worker" {
				t.Fatalf("companion web/worker leaked into dependency levels: %v", levels)
			}
		}
	}
}
