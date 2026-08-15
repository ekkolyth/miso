package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "echo-args", root, nil, nil, []string{"staging"}, false)
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
	pm, _, _, ran, err := buildRun(cfg, "deploy", root, bun.Bun{}, nil, []string{"staging"}, false)
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
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, []string{"staging"}, false)
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
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, []string{"staging"}, false)
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

// fences the root-exclusion rule: a root concurrent list resolves at ROOT
// scope only, never broadcasting into fan-out members. A companion that
// exists only inside a member is unresolvable at root and must hard-error,
// naming the concurrent entry.
func TestBuildRunRootConcurrentUnresolvableAtRootErrors(t *testing.T) {
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

	// root declares a concurrent for "dev" but has no root dev script, and
	// "services" exists only inside the web member — unresolvable at root.
	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"services"}}},
	}
	_, _, _, _, err := buildRun(cfg, "dev", root, nil, nil, nil, false)
	if err == nil {
		t.Fatal("expected error: root concurrent \"services\" is unresolvable at root scope")
	}
	if !strings.Contains(err.Error(), `"services"`) {
		t.Errorf("error = %v, want it to name \"services\"", err)
	}
}

// root package.json "dev": "miso dev" must not preempt member fan-out —
// the recursion trap the root-exclusion rule deletes
func TestDiscoverEntriesRootScriptDoesNotPreemptFanOut(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"],"scripts":{"dev":"miso dev"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webScripts := filepath.Join(root, "apps", "web", "scripts")
	if err := os.MkdirAll(webScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webScripts, "dev.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "web", "package.json"),
		[]byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %v, want only the web fan-out", labelsOf(entries))
	}
	if entries[0].WorkspaceName != "web" {
		t.Errorf("entry = %+v, want web member entry", entries[0])
	}
}

// the #44 compose case: root task concurrent spawns once at root scope
// alongside member fan-out
func TestDiscoverEntriesRootCompanionRunsOnceWithFanOut(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web","apps/api"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, memberName := range []string{"web", "api"} {
		memberScripts := filepath.Join(root, "apps", memberName, "scripts")
		if err := os.MkdirAll(memberScripts, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(memberScripts, "dev.sh"), []byte("exit 0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "apps", memberName, "package.json"),
			[]byte(`{"name":"`+memberName+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rootDB := filepath.Join(root, "scripts", "db")
	if err := os.MkdirAll(rootDB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDB, "up.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"#db/up"}}},
	}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}

	companionCount := 0
	for _, entry := range entries {
		if entry.IsConcurrent {
			companionCount++
			if entry.WorkspaceDir != root {
				t.Errorf("companion resolved at %q, want root", entry.WorkspaceDir)
			}
		}
	}
	if companionCount != 1 {
		t.Errorf("companions = %d, want exactly 1 (labels: %v)", companionCount, labelsOf(entries))
	}
	if len(entries) != 3 {
		t.Errorf("entries = %v, want [api web db/up-companion]", labelsOf(entries))
	}
}

// step 4 of the ladder: no member defines the name → root script is the body
func TestDiscoverEntriesRootBodyWhenNoMemberDefinesName(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps", "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "web", "package.json"),
		[]byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rootDB := filepath.Join(root, "scripts", "db")
	if err := os.MkdirAll(rootDB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDB, "up.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "db/up", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].WorkspaceDir != root {
		t.Fatalf("entries = %v, want single root entry", labelsOf(entries))
	}
}

// no member defines the name, no root script exists either — only a root
// task concurrent. The companion must still launch alone rather than the
// run silently resolving to nothing.
func TestDiscoverEntriesConcurrentOnlyLaunchesAlone(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps", "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "web", "package.json"),
		[]byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rootDB := filepath.Join(root, "scripts", "db")
	if err := os.MkdirAll(rootDB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDB, "up.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"db/up"}}},
	}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %v, want a single companion entry", labelsOf(entries))
	}
	if !entries[0].IsConcurrent {
		t.Errorf("entry = %+v, want IsConcurrent", entries[0])
	}
	if entries[0].WorkspaceDir != root {
		t.Errorf("WorkspaceDir = %q, want root %q", entries[0].WorkspaceDir, root)
	}
}

// TestMarkPlain verifies LaunchPlain's pre-run marking: every process flips to
// pipe-mode and gains FORCE_COLOR (unless NO_COLOR), and a nil Environ is seeded
// from os.Environ rather than clobbered down to FORCE_COLOR alone.
func TestMarkPlain(t *testing.T) {
	pm := NewProcessManager()
	withEnv := pm.Add(TuiScriptEntry{Label: "a"}, "sh", nil, "", []string{"PATH=/bin"})
	noColor := pm.Add(TuiScriptEntry{Label: "b"}, "sh", nil, "", []string{"PATH=/bin", "NO_COLOR=1"})
	inherit := pm.Add(TuiScriptEntry{Label: "c"}, "sh", nil, "", nil)

	markPlain(pm.Processes)

	for _, p := range pm.Processes {
		if !p.NoPTY {
			t.Errorf("%s: NoPTY not set", p.Entry.Label)
		}
	}
	if !slices.Contains(withEnv.Environ, "FORCE_COLOR=1") {
		t.Errorf("explicit env: FORCE_COLOR not added: %v", withEnv.Environ)
	}
	if slices.Contains(noColor.Environ, "FORCE_COLOR=1") {
		t.Errorf("NO_COLOR env: FORCE_COLOR must not be added: %v", noColor.Environ)
	}
	// A nil Environ must be seeded from os.Environ (kept as a superset), not
	// replaced by a lone FORCE_COLOR that would strip PATH/HOME from the child.
	for _, kv := range os.Environ() {
		if !slices.Contains(inherit.Environ, kv) {
			t.Fatalf("nil Environ was clobbered, not seeded from os.Environ (missing %q)", kv)
		}
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
	_, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, nil, false)
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

// a task entry that resolves no body and declares no companions is dead
// config — error, not silence
func TestDiscoverEntriesEmptyTaskEntryErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps", "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "web", "package.json"),
		[]byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"ghost": {}},
	}
	_, err := discoverEntries(cfg, "ghost", root, nil)
	if err == nil {
		t.Fatal("expected error for empty task entry, got nil")
	}
	if !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "declares nothing") {
		t.Errorf("error %q does not describe the empty entry", err.Error())
	}
}

// TestDiscoverEntriesResolutionLadder is the spec's exhaustiveness artifact
// for the routes discoverEntries can take. cmd/main.go's own delegation into
// discoverEntries is out of scope — each case builds cfg + fixture directly.
func TestDiscoverEntriesResolutionLadder(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func(t *testing.T) (cfg config.Config, root, scriptName string)
		wantErr    string
		checkEntry func(t *testing.T, root string, entries []TuiScriptEntry)
	}{
		{
			name: "single repo, root scripts folder file resolves to root entry",
			setup: func(t *testing.T) (config.Config, string, string) {
				root := t.TempDir()
				scriptsDir := filepath.Join(root, "scripts")
				if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writeScript(t, scriptsDir, "dev")
				return config.Config{Scripts: "./scripts"}, root, "dev"
			},
			checkEntry: func(t *testing.T, root string, entries []TuiScriptEntry) {
				if len(entries) != 1 || entries[0].WorkspaceDir != root || entries[0].ScriptSource != "folder" {
					t.Fatalf("entries = %+v, want single root folder entry", entries)
				}
			},
		},
		{
			name: "single repo, root package.json script resolves to root entry",
			setup: func(t *testing.T) (config.Config, string, string) {
				root := t.TempDir()
				writePackageJSON(t, root, map[string]string{"build": "echo build"})
				return config.Config{Scripts: "./scripts"}, root, "build"
			},
			checkEntry: func(t *testing.T, root string, entries []TuiScriptEntry) {
				if len(entries) != 1 || entries[0].WorkspaceDir != root || entries[0].ScriptSource != "packagejson" {
					t.Fatalf("entries = %+v, want single root packagejson entry", entries)
				}
			},
		},
		{
			name: "monorepo, member defines name, root package.json defines same name: member wins, root excluded",
			setup: func(t *testing.T) (config.Config, string, string) {
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
				return config.Config{Scripts: "./scripts"}, root, "dev"
			},
			checkEntry: func(t *testing.T, root string, entries []TuiScriptEntry) {
				webDir := filepath.Join(root, "apps", "web")
				if len(entries) != 1 || entries[0].WorkspaceDir != webDir {
					t.Fatalf("entries = %v, want single member entry at %q", labelsOf(entries), webDir)
				}
			},
		},
		{
			name: "monorepo, no member defines name, root scripts folder file resolves to root entry",
			setup: func(t *testing.T) (config.Config, string, string) {
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "package.json"),
					[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(root, "apps", "web"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "apps", "web", "package.json"),
					[]byte(`{"name":"web"}`), 0o644); err != nil {
					t.Fatal(err)
				}
				rootDB := filepath.Join(root, "scripts", "db")
				if err := os.MkdirAll(rootDB, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(rootDB, "up.sh"), []byte("exit 0\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return config.Config{Scripts: "./scripts"}, root, "db/up"
			},
			checkEntry: func(t *testing.T, root string, entries []TuiScriptEntry) {
				if len(entries) != 1 || entries[0].WorkspaceDir != root {
					t.Fatalf("entries = %v, want single root entry", labelsOf(entries))
				}
			},
		},
		{
			name: "monorepo, member defines name, root task concurrent resolvable at root: member entry + one root companion",
			setup: func(t *testing.T) (config.Config, string, string) {
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "package.json"),
					[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
					t.Fatal(err)
				}
				webDir := filepath.Join(root, "apps", "web")
				if err := os.MkdirAll(webDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writePackageJSON(t, webDir, map[string]string{"dev": "vite"})
				rootDB := filepath.Join(root, "scripts", "db")
				if err := os.MkdirAll(rootDB, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(rootDB, "up.sh"), []byte("exit 0\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				cfg := config.Config{
					Scripts: "./scripts",
					Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"#db/up"}}},
				}
				return cfg, root, "dev"
			},
			checkEntry: func(t *testing.T, root string, entries []TuiScriptEntry) {
				webDir := filepath.Join(root, "apps", "web")
				companions := 0
				members := 0
				for _, entry := range entries {
					if entry.IsConcurrent {
						companions++
						if entry.WorkspaceDir != root {
							t.Errorf("companion resolved at %q, want root", entry.WorkspaceDir)
						}
						continue
					}
					members++
					if entry.WorkspaceDir != webDir {
						t.Errorf("member entry resolved at %q, want %q", entry.WorkspaceDir, webDir)
					}
				}
				if members != 1 || companions != 1 {
					t.Fatalf("entries = %v, want 1 member + 1 companion", labelsOf(entries))
				}
			},
		},
		{
			name: "monorepo, task entry resolves nothing and declares no orchestration: declares-nothing error",
			setup: func(t *testing.T) (config.Config, string, string) {
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "package.json"),
					[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(root, "apps", "web"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "apps", "web", "package.json"),
					[]byte(`{"name":"web"}`), 0o644); err != nil {
					t.Fatal(err)
				}
				cfg := config.Config{
					Scripts: "./scripts",
					Tasks:   map[string]config.TaskConfig{"ghost": {}},
				}
				return cfg, root, "ghost"
			},
			wantErr: "declares nothing",
		},
		{
			name: "monorepo, root task concurrent unresolvable at root: concurrent error",
			setup: func(t *testing.T) (config.Config, string, string) {
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "package.json"),
					[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
					t.Fatal(err)
				}
				webDir := filepath.Join(root, "apps", "web")
				if err := os.MkdirAll(webDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writePackageJSON(t, webDir, map[string]string{"dev": "vite"})
				cfg := config.Config{
					Scripts: "./scripts",
					Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"#ghost"}}},
				}
				return cfg, root, "dev"
			},
			wantErr: "concurrent",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg, root, scriptName := testCase.setup(t)
			entries, err := discoverEntries(cfg, scriptName, root, nil)
			if testCase.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", testCase.wantErr)
				}
				if !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("discoverEntries: %v", err)
			}
			testCase.checkEntry(t, root, entries)
		})
	}
}
