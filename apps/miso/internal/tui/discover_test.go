package tui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/workspace"
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

// TestDiscoverTuiScripts_ExactMatchAcrossWorkspaces verifies each workspace
// contributes only its script named exactly "dev" — "dev:worker" stays opt-in.
func TestDiscoverTuiScripts_ExactMatchAcrossWorkspaces(t *testing.T) {
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

	// entries are sorted alphabetically by label
	labels := labelsOf(entries)
	if !slices.Equal(labels, []string{"api", "app"}) {
		t.Fatalf("labels = %v, want [api app]", labels)
	}
	for _, e := range entries {
		if e.ScriptName != "dev" {
			t.Errorf("%s: ScriptName = %q, want dev", e.Label, e.ScriptName)
		}
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
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) == 0 || entries[0].Label != "@org/web" {
		t.Fatalf("label = %v, want @org/web", labelsOf(entries))
	}
}

// TestDiscoverWorkspaceScripts_WorkspaceNameSurvivesLabelDedup verifies that a
// member running its main script plus a member-local companion carries the
// member name in WorkspaceName even though Label is rewritten to
// "member/scriptName".
func TestDiscoverWorkspaceScripts_WorkspaceNameSurvivesLabelDedup(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, webDir, map[string]string{
		"dev":    "next dev",
		"studio": "prisma studio",
	})
	if err := os.WriteFile(filepath.Join(webDir, "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"concurrent":["studio"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), labelsOf(entries))
	}
	for _, e := range entries {
		if e.Label == "web" {
			t.Errorf("Label = %q, want deduped (two entries in this member)", e.Label)
		}
		if e.WorkspaceName != "web" {
			t.Errorf("WorkspaceName = %q, want web (label = %q)", e.WorkspaceName, e.Label)
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
	entries, err := discoverEntries(cfg, "dev", root, nil)
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
	entries, err := discoverEntries(cfg, "dev", root, nil)
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

// TestEntryLabelUsesPackageNameForm verifies a member with two entries produces
// "@scope/pkg/script" labels (not "@scope/pkg:script"), and a member with a
// single entry uses the bare package name.
func TestEntryLabelUsesPackageNameForm(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web","apps/single"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	singleDir := filepath.Join(root, "apps", "single")
	for _, dir := range []string{webDir, singleDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(webDir, "package.json"),
		[]byte(`{"name":"@ekko/web","scripts":{"dev":"next dev","studio":"prisma studio"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"concurrent":["studio"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(singleDir, "package.json"),
		[]byte(`{"name":"@ekko/single","scripts":{"dev":"vite"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}

	labels := labelsOf(entries)
	sort.Strings(labels)
	expected := []string{"@ekko/single", "@ekko/web/dev", "@ekko/web/studio"}
	if !slices.Equal(labels, expected) {
		t.Errorf("labels = %v, want %v", labels, expected)
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

// a member's script wins fan-out even when the root also defines the same
// name — root is never a fan-out member.
func TestDiscoverEntriesMemberFanOutWinsOverRootScript(t *testing.T) {
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
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 fan-out entry, got %d: %v", len(entries), labelsOf(entries))
	}
	if entries[0].WorkspaceDir != webDir {
		t.Errorf("entry dir = %q, want member dir %q", entries[0].WorkspaceDir, webDir)
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

// TestDiscoverEntriesMemberConcurrentRunsWithinMember verifies a member's own
// concurrent task (declared in that member's miso.json) is resolved within
// that member, alongside its main script.
func TestDiscoverEntriesMemberConcurrentRunsWithinMember(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/explorer"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "apps", "explorer")
	scriptsDir := filepath.Join(appDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, appDir, map[string]string{"dev": "vite"})
	writeScript(t, scriptsDir, "convex") // apps/explorer/scripts/convex.sh
	if err := os.WriteFile(filepath.Join(appDir, "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"concurrent":["convex"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	labels := labelsOf(entries)
	if !slices.Contains(labels, "explorer/dev") || !slices.Contains(labels, "explorer/convex") {
		t.Fatalf("want explorer/dev + explorer/convex, got %v", labels)
	}
}

// TestDiscoverEntriesRootConcurrentTargetsMemberScope verifies a root-scope
// concurrent entry written as "@member/script" resolves script inside that
// member, not against the root scope.
func TestDiscoverEntriesRootConcurrentTargetsMemberScope(t *testing.T) {
	root := t.TempDir()
	rootScripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(rootScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, rootScripts, "dev") // root scripts/dev.sh — no member defines "dev", so it resolves at root scope
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	webScripts := filepath.Join(webDir, "scripts")
	if err := os.MkdirAll(webScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, webScripts, "studio") // apps/web/scripts/studio.sh

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"@web/studio"}}},
	}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	labels := labelsOf(entries)
	if !slices.Contains(labels, "web") && !slices.Contains(labels, "web/studio") {
		t.Fatalf("want a studio entry from the web member, got %v", labels)
	}
}

// TestDiscoverEntriesMemberConcurrentCrossReferencesOtherMember verifies a
// member's own concurrent entry written as "@other/script" resolves inside
// that other member, not the declaring member — discoverMemberFanOut calling
// resolveConcurrent with a non-nil local scope and an @-ref.
func TestDiscoverEntriesMemberConcurrentCrossReferencesOtherMember(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/explorer","apps/worker"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	explorerDir := filepath.Join(root, "apps", "explorer")
	if err := os.MkdirAll(explorerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, explorerDir, map[string]string{"dev": "vite"})
	if err := os.WriteFile(filepath.Join(explorerDir, "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"concurrent":["@worker/queue"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	workerScripts := filepath.Join(root, "apps", "worker", "scripts")
	if err := os.MkdirAll(workerScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, workerScripts, "queue") // apps/worker/scripts/queue.sh

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	labels := labelsOf(entries)
	if !slices.Contains(labels, "explorer") {
		t.Fatalf("want explorer's own dev entry, got %v", labels)
	}
	if !slices.Contains(labels, "worker") {
		t.Fatalf("want @worker/queue to resolve inside the worker member, got %v", labels)
	}
}

// TestResolveConcurrentAtRefErrors verifies malformed and unknown @-refs in a
// concurrent entry return errors instead of silently doing nothing.
func TestResolveConcurrentAtRefErrors(t *testing.T) {
	root := t.TempDir()
	members := []workspace.Member{{Name: "web", Dir: filepath.Join(root, "apps", "web")}}
	cfg := config.Config{Scripts: "./scripts"}

	t.Run("missing slash", func(t *testing.T) {
		if _, err := resolveConcurrent(cfg, "@web", root, nil, members); err == nil {
			t.Fatal("expected error for @member with no /script")
		}
	})

	t.Run("unknown member", func(t *testing.T) {
		if _, err := resolveConcurrent(cfg, "@nope/script", root, nil, members); err == nil {
			t.Fatal("expected error for unknown member @nope")
		}
	})
}

// TestDiscoverEntriesConcurrentMemberRefNameBeatsBasename verifies an
// "@member/script" concurrent entry resolves the member NAMED "web", not an
// unrelated member whose directory basename happens to be "web" — the same
// name-first tiering ResolveScopes already applies to explicit CLI @scopes.
// The basename-collision workspace is listed first so an any-match, first-hit
// loop (the pre-fix behavior) would have picked it by mistake.
func TestDiscoverEntriesConcurrentMemberRefNameBeatsBasename(t *testing.T) {
	root := t.TempDir()
	rootScripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(rootScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, rootScripts, "dev") // root scripts/dev.sh — neither member below defines "dev", so it resolves at root scope

	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["projects/legacy/web","apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	legacyDir := filepath.Join(root, "projects", "legacy", "web")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "package.json"),
		[]byte(`{"name":"legacy-web","scripts":{"build":"vite build"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	webDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "package.json"),
		[]byte(`{"name":"web","scripts":{"build":"vite build"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"@web/build"}}},
	}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}
	labels := labelsOf(entries)
	if !slices.Contains(labels, "web") {
		t.Fatalf("want @web/build to resolve the member named web, got %v", labels)
	}
	if slices.Contains(labels, "legacy-web") {
		t.Fatalf("resolved the dir-basename collision instead of the member named web: %v", labels)
	}
}

// verifies "#name" resolves at root scope even when declared in a member's
// own task list.
func TestResolveConcurrentHashPrefixResolvesAtRoot(t *testing.T) {
	root := t.TempDir()
	rootScripts := filepath.Join(root, "scripts", "db")
	if err := os.MkdirAll(rootScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootScripts, "up.sh"), []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	memberDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	local := WorkspaceInfo{Name: "web", Dir: memberDir, ScriptsFolder: "./scripts"}

	entries, err := resolveConcurrent(cfg, "#db/up", root, &local, nil)
	if err != nil {
		t.Fatalf("resolveConcurrent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].WorkspaceDir != root {
		t.Errorf("WorkspaceDir = %q, want root %q", entries[0].WorkspaceDir, root)
	}
	if entries[0].ScriptName != "db/up" {
		t.Errorf("ScriptName = %q, want db/up", entries[0].ScriptName)
	}
}

// an unresolvable concurrent entry fails the run instead of vanishing
func TestResolveConcurrentUnresolvableEntryErrors(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{Scripts: "./scripts"}

	testCases := []struct {
		name       string
		concName   string
		local      *WorkspaceInfo
		wantSearch string
	}{
		{name: "bare name at root", concName: "ghost", local: nil, wantSearch: "scripts folder"},
		{name: "hash name", concName: "#ghost", local: nil, wantSearch: "scripts folder"},
		{name: "bare name in member", concName: "ghost",
			local:      &WorkspaceInfo{Name: "web", Dir: filepath.Join(root, "apps", "web"), ScriptsFolder: "./scripts"},
			wantSearch: `member "web"`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := resolveConcurrent(cfg, testCase.concName, root, testCase.local, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "ghost") {
				t.Errorf("error %q does not name the entry", err.Error())
			}
			if !strings.Contains(err.Error(), testCase.wantSearch) {
				t.Errorf("error %q does not name a searched location (%s)", err.Error(), testCase.wantSearch)
			}
		})
	}
}

// a check script never pulls in its "lint:fix" or "docker/up" sibling — those
// write to the user's tree and must be named to run
func TestDiscoverWorkspaceScriptsMatchesExactNameOnly(t *testing.T) {
	wsDir := t.TempDir()
	writePackageJSON(t, wsDir, map[string]string{
		"lint":     "echo check",
		"lint:fix": "echo fix",
		"lint-fix": "echo dashfix",
		"lintfix":  "echo barefix",
	})
	dockerDir := filepath.Join(wsDir, "scripts", "docker")
	lintDir := filepath.Join(wsDir, "scripts", "lint")
	for _, dir := range []string{dockerDir, lintDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeScript(t, dockerDir, "index")
	writeScript(t, dockerDir, "up")
	writeScript(t, dockerDir, "build")
	writeScript(t, lintDir, "fix")

	ws := WorkspaceInfo{Name: "web", Dir: wsDir}

	lintEntries, err := discoverWorkspaceScripts("lint", ws, "./scripts")
	if err != nil {
		t.Fatalf("discoverWorkspaceScripts(lint): %v", err)
	}
	if len(lintEntries) != 1 || lintEntries[0].ScriptName != "lint" {
		t.Fatalf("lint resolved %v, want only lint", scriptNamesOf(lintEntries))
	}

	dockerEntries, err := discoverWorkspaceScripts("docker", ws, "./scripts")
	if err != nil {
		t.Fatalf("discoverWorkspaceScripts(docker): %v", err)
	}
	if len(dockerEntries) != 1 || dockerEntries[0].ScriptName != "docker" {
		t.Fatalf("docker resolved %v, want only docker", scriptNamesOf(dockerEntries))
	}
	if dockerEntries[0].ScriptSource != "folder" {
		t.Errorf("ScriptSource = %q, want folder", dockerEntries[0].ScriptSource)
	}
}

// MISO-2: "@lumen/ios" must not also start "ios:device", which needs a
// physical device attached and fails the whole run
func TestDiscoverEntriesAtRefConcurrentResolvesExactScript(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/lumen"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lumenDir := filepath.Join(root, "apps", "lumen")
	if err := os.MkdirAll(lumenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, lumenDir, map[string]string{
		"ios":        "expo run:ios",
		"ios:device": "expo run:ios --device",
		"android":    "expo run:android",
	})

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"@lumen/ios", "@lumen/android"}}},
	}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}

	names := scriptNamesOf(entries)
	sort.Strings(names)
	if !slices.Equal(names, []string{"android", "ios"}) {
		t.Fatalf("script names = %v, want [android ios]", names)
	}
	labels := labelsOf(entries)
	sort.Strings(labels)
	if !slices.Equal(labels, []string{"lumen/android", "lumen/ios"}) {
		t.Errorf("labels = %v, want [lumen/android lumen/ios]", labels)
	}
}

// a bare concurrent name in a member's own miso.json resolves only that
// script within the member
func TestDiscoverEntriesMemberConcurrentResolvesExactScript(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, webDir, map[string]string{
		"dev":     "vite",
		"ios":     "expo run:ios",
		"ios:sim": "expo run:ios --simulator",
	})
	if err := os.WriteFile(filepath.Join(webDir, "miso.json"),
		[]byte(`{"repo":{"tasks":{"dev":{"concurrent":["ios"]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	entries, err := discoverEntries(cfg, "dev", root, nil)
	if err != nil {
		t.Fatalf("discoverEntries: %v", err)
	}

	names := scriptNamesOf(entries)
	sort.Strings(names)
	if !slices.Equal(names, []string{"dev", "ios"}) {
		t.Fatalf("script names = %v, want [dev ios]", names)
	}
}

func scriptNamesOf(entries []TuiScriptEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ScriptName
	}
	return out
}
