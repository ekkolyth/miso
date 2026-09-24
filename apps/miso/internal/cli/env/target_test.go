package env

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/workspace"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rootCfg() config.Config {
	return config.Config{
		Env: []*config.EnvEntry{
			{Scope: "global", Path: ".env"},
			{Scope: "web", Path: "apps/web/.env.local"},
			{Scope: "api", Path: "apps/api/.env.local"},
		},
	}
}

func TestBuildTargetEnv_RootScriptGetsGlobalOnly(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "LOG_COLORS=true\n")
	write(t, filepath.Join(root, "apps", "web", ".env.local"), "CONVEX_DEPLOYMENT=web-only\n")

	target := workspace.Target{Kind: workspace.TargetScript, Name: "ekklipse"}
	environ, err := BuildTargetEnv(root, rootCfg(), target)
	if err != nil {
		t.Fatalf("BuildTargetEnv() error: %v", err)
	}
	got := envSliceToMap(environ)
	if got["LOG_COLORS"] != "true" {
		t.Errorf("LOG_COLORS = %q, want true (global)", got["LOG_COLORS"])
	}
	if got["CONVEX_DEPLOYMENT"] != "" {
		t.Errorf("CONVEX_DEPLOYMENT = %q, want empty (leak)", got["CONVEX_DEPLOYMENT"])
	}
}

func TestBuildTargetEnv_MemberScopedIsolation(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "LOG_COLORS=true\n")
	write(t, filepath.Join(root, "apps", "web", ".env.local"), "WEB_PORT=3000\n")
	write(t, filepath.Join(root, "apps", "api", ".env.local"), "API_PORT=4000\n")

	target := workspace.Target{Kind: workspace.TargetMember, Name: "web", Dir: filepath.Join(root, "apps", "web")}
	got := envSliceToMap(mustEnv(t, root, rootCfg(), target))
	if got["WEB_PORT"] != "3000" {
		t.Errorf("WEB_PORT = %q, want 3000", got["WEB_PORT"])
	}
	if got["API_PORT"] != "" {
		t.Errorf("API_PORT = %q, want empty (isolation)", got["API_PORT"])
	}
}

func TestBuildTargetEnv_Precedence_LocalOverTargetOverGlobal(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "X=global\n")
	write(t, filepath.Join(root, "apps", "web", ".env.local"), "X=target\n")
	// member-local config overrides again
	write(t, filepath.Join(root, "apps", "web", "miso.json"),
		`{"scripts":"./scripts","env":[{"path":".env.member"}]}`)
	write(t, filepath.Join(root, "apps", "web", ".env.member"), "X=local\n")

	target := workspace.Target{Kind: workspace.TargetMember, Name: "web", Dir: filepath.Join(root, "apps", "web")}
	got := envSliceToMap(mustEnv(t, root, rootCfg(), target))
	if got["X"] != "local" {
		t.Errorf("X = %q, want local (local > target > global)", got["X"])
	}
}

func TestBuildTargetEnv_AmbientWins(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "HOME=/from/file\n")
	target := workspace.Target{Kind: workspace.TargetScript, Name: "dev"}
	got := envSliceToMap(mustEnv(t, root, rootCfg(), target))
	if got["HOME"] == "/from/file" {
		t.Error("HOME overwritten by .env; ambient must win")
	}
}

func TestBuildTargetEnv_MemberMatchesByBasename(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "LOG_COLORS=true\n")
	write(t, filepath.Join(root, "apps", "web", ".env.local"), "CONVEX_DEPLOYMENT=web-only\n")

	target := workspace.Target{Kind: workspace.TargetMember, Name: "@org/web", Dir: filepath.Join(root, "apps", "web")}
	got := envSliceToMap(mustEnv(t, root, rootCfg(), target))
	if got["CONVEX_DEPLOYMENT"] != "web-only" {
		t.Errorf("CONVEX_DEPLOYMENT = %q, want web-only (matched by basename)", got["CONVEX_DEPLOYMENT"])
	}
	if got["LOG_COLORS"] != "true" {
		t.Errorf("LOG_COLORS = %q, want true (global)", got["LOG_COLORS"])
	}
}

func TestBuildTargetEnv_ZeroConfig_DiscoversRootDotEnv(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "ROOT_VAR=root-value\n")

	target := workspace.Target{Kind: workspace.TargetScript, Name: "dev"}
	got := envSliceToMap(mustEnv(t, root, config.Config{}, target))
	if got["ROOT_VAR"] != "root-value" {
		t.Errorf("ROOT_VAR = %q, want root-value (zero-config discovery)", got["ROOT_VAR"])
	}
}

func TestBuildTargetEnv_ZeroConfig_MemberDiscoversInMemberDir(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "ROOT_ONLY=root-value\n")
	write(t, filepath.Join(root, "apps", "web", ".env"), "WEB_VAR=web-value\n")

	target := workspace.Target{Kind: workspace.TargetMember, Name: "web", Dir: filepath.Join(root, "apps", "web")}
	got := envSliceToMap(mustEnv(t, root, config.Config{}, target))
	if got["WEB_VAR"] != "web-value" {
		t.Errorf("WEB_VAR = %q, want web-value (member-dir discovery)", got["WEB_VAR"])
	}
	if got["ROOT_ONLY"] != "" {
		t.Errorf("ROOT_ONLY = %q, want empty (member discovery scoped to member dir)", got["ROOT_ONLY"])
	}
}

func mustEnv(t *testing.T, root string, cfg config.Config, target workspace.Target) []string {
	t.Helper()
	environ, err := BuildTargetEnv(root, cfg, target)
	if err != nil {
		t.Fatalf("BuildTargetEnv() error: %v", err)
	}
	return environ
}

func pathEntries(environ []string) []string {
	m := envSliceToMap(environ)
	if m["PATH"] == "" {
		return nil
	}
	return strings.Split(m["PATH"], string(os.PathListSeparator))
}

func TestBuildTargetEnv_PrependsMemberBinNearestFirst(t *testing.T) {
	root := t.TempDir()
	memberBin := filepath.Join(root, "apps", "web", "node_modules", ".bin")
	rootBin := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(memberBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rootBin, 0o755); err != nil {
		t.Fatal(err)
	}

	target := workspace.Target{Kind: workspace.TargetMember, Name: "web", Dir: filepath.Join(root, "apps", "web")}
	environ := mustEnv(t, root, config.Config{}, target)
	entries := pathEntries(environ)
	if len(entries) < 2 || entries[0] != memberBin || entries[1] != rootBin {
		t.Errorf("PATH prefix = %v, want [%s %s ...]", entries, memberBin, rootBin)
	}
}

func TestBuildTargetEnv_NoEnvFilesButBinsStillReturns(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules", ".bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := workspace.Target{Kind: workspace.TargetScript, Name: "dev"}
	environ, err := BuildTargetEnv(root, config.Config{}, target)
	if err != nil {
		t.Fatalf("BuildTargetEnv() error: %v", err)
	}
	if environ == nil {
		t.Fatal("expected augmented environ (bins present), got nil")
	}
	if pathEntries(environ)[0] != filepath.Join(root, "node_modules", ".bin") {
		t.Errorf("PATH[0] = %q, want root .bin", pathEntries(environ)[0])
	}
}

func TestBuildTargetEnv_NoFilesNoBinsReturnsNil(t *testing.T) {
	target := workspace.Target{Kind: workspace.TargetScript, Name: "dev"}
	environ, err := BuildTargetEnv(t.TempDir(), config.Config{}, target)
	if err != nil {
		t.Fatalf("BuildTargetEnv() error: %v", err)
	}
	if environ != nil {
		t.Errorf("got %v, want nil (no env, no bins)", environ)
	}
}

// A delegated repo only reaches BuildTargetEnv when miso is orchestrating the
// run itself (a repo.tasks override). Turbo-owned runs go through DelegateLaunch,
// which builds its own environ — so injection follows orchestration, not mode.
func TestBuildTargetEnv_DelegatedInjectsWhenMisoOrchestrates(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "X=delegated\n")
	cfg := config.Config{
		Repo:  "turbo",
		Tasks: map[string]config.TaskConfig{"dev": {}},
		Env:   []*config.EnvEntry{{Scope: "global", Path: ".env"}},
	}
	got, err := BuildTargetEnv(root, cfg, workspace.Target{Kind: workspace.TargetScript, Name: "dev"})
	if err != nil {
		t.Fatalf("BuildTargetEnv() error: %v", err)
	}
	if envSliceToMap(got)["X"] != "delegated" {
		t.Error("miso-orchestrated task in a delegated repo got no injected env")
	}
}

func TestValidatedLine_CountsGlobalAndTargetScopes(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "LOG_COLORS=true\n")
	write(t, filepath.Join(root, "apps", "web", ".env.local"), "PORT=3000\n")

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Scope: "global", Path: ".env", Variables: config.EnvVariables{Array: []string{"LOG_COLORS"}}},
			{Scope: "web", Path: "apps/web/.env.local", Variables: config.EnvVariables{Array: []string{"PORT", "HOST"}}},
			{Scope: "api", Path: "apps/api/.env.local", Variables: config.EnvVariables{Array: []string{"NOPE"}}},
		},
	}
	target := workspace.Target{Kind: workspace.TargetMember, Name: "web", Dir: filepath.Join(root, "apps", "web")}

	got := ValidatedLine(root, cfg, target)
	want := "env validated — 2 scopes, 3 variables"
	if got != want {
		t.Errorf("ValidatedLine() = %q, want %q", got, want)
	}
}

func TestValidatedLine_SingularAndEmpty(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "LOG_COLORS=true\n")

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Scope: "global", Path: ".env", Variables: config.EnvVariables{Array: []string{"LOG_COLORS"}}},
		},
	}
	got := ValidatedLine(root, cfg, workspace.Target{Kind: workspace.TargetScript, Name: "dev"})
	if got != "env validated — 1 scope, 1 variable" {
		t.Errorf("ValidatedLine() = %q, want singular form", got)
	}

	// nothing declared anywhere — no result to report
	if line := ValidatedLine(root, config.Config{}, workspace.Target{Kind: workspace.TargetScript, Name: "dev"}); line != "" {
		t.Errorf("ValidatedLine() = %q, want empty", line)
	}
}

func TestBuildTargetEnv_ShellDefaultResolves(t *testing.T) {
	unsetEnv(t, "TEST_DATABASE_URL", "TEST_REDIS_URL")
	root := t.TempDir()
	write(t, filepath.Join(root, "test.env"), shellDefaultEnv)
	cfg := config.Config{Env: []*config.EnvEntry{{Scope: "global", Path: "test.env"}}}

	target := workspace.Target{Kind: workspace.TargetScript, Name: "show"}
	got := envSliceToMap(mustEnv(t, root, cfg, target))
	expected := "postgres://test:test@localhost:55432/ekkolore_test?sslmode=disable"
	if got["TEST_DATABASE_URL"] != expected {
		t.Errorf("TEST_DATABASE_URL = %q, want %q", got["TEST_DATABASE_URL"], expected)
	}
}

// BuildTargetEnv's failure must render exactly as `miso env` renders it
func assertSameFailure(t *testing.T, root string, cfg config.Config, target workspace.Target) string {
	t.Helper()
	environ, buildErr := BuildTargetEnv(root, cfg, target)
	if buildErr == nil {
		t.Fatalf("BuildTargetEnv() = %v, nil; want an error", environ)
	}
	runErr := Run(root, cfg, log.New(io.Discard))
	if runErr == nil {
		t.Fatal("Run() = nil, want an error")
	}
	var built, ran *ValidationError
	if !errors.As(buildErr, &built) || !errors.As(runErr, &ran) {
		t.Fatalf("want *ValidationError from both; got %T and %T", buildErr, runErr)
	}
	var builtOut, ranOut bytes.Buffer
	built.Render(&builtOut)
	ran.Render(&ranOut)
	if builtOut.String() != ranOut.String() {
		t.Errorf("BuildTargetEnv renders\n%s\nmiso env renders\n%s", builtOut.String(), ranOut.String())
	}
	return builtOut.String()
}

func TestBuildTargetEnv_UnsupportedExpansionFailsRun(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "test.env"), "X=$(date)\n")
	cfg := config.Config{Env: []*config.EnvEntry{{Scope: "global", Path: "test.env"}}}

	out := assertSameFailure(t, root, cfg, workspace.Target{Kind: workspace.TargetScript, Name: "show"})
	if !strings.Contains(out, "$(…)") {
		t.Errorf("%q should name the unsupported form", out)
	}
}

func TestBuildTargetEnv_DiscoveredFileParseErrorFails(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "X=${X:?boom}\n")

	target := workspace.Target{Kind: workspace.TargetScript, Name: "show"}
	if _, err := BuildTargetEnv(root, config.Config{}, target); err == nil {
		t.Fatal("BuildTargetEnv() = nil error, want the parse failure")
	}
	if err := Run(root, config.Config{}, log.New(io.Discard)); err == nil {
		t.Fatal("Run() = nil error in discovery mode, want the parse failure")
	}
}

func TestBuildTargetEnv_MissingDeclaredFileFails(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{Env: []*config.EnvEntry{{Scope: "global", Path: ".env.local"}}}

	out := assertSameFailure(t, root, cfg, workspace.Target{Kind: workspace.TargetScript, Name: "show"})
	if !strings.Contains(out, "env file not found") {
		t.Errorf("%q should say the file is missing", out)
	}
}

func TestBuildTargetEnv_ReportsEveryLoadFailure(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.env"), "X=${X:?boom}\n")
	cfg := config.Config{Env: []*config.EnvEntry{
		{Scope: "global", Path: "a.env"},
		{Scope: "global", Path: "missing.env"},
	}}

	out := assertSameFailure(t, root, cfg, workspace.Target{Kind: workspace.TargetScript, Name: "show"})
	if !strings.Contains(out, "a.env") || !strings.Contains(out, "missing.env") {
		t.Errorf("%q should report both entries", out)
	}
}

func TestBuildTargetEnv_BrokenMemberConfigFails(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "package.json"), `{"workspaces":["apps/*"]}`)
	write(t, filepath.Join(root, ".env"), "X=1\n")
	memberDir := filepath.Join(root, "apps", "web")
	write(t, filepath.Join(memberDir, "miso.json"), "{not valid json")
	cfg := config.Config{Env: []*config.EnvEntry{{Scope: "global", Path: ".env"}}}

	target := workspace.Target{Kind: workspace.TargetMember, Name: "web", Dir: memberDir}
	_, buildErr := BuildTargetEnv(root, cfg, target)
	runErr := Run(root, cfg, log.New(io.Discard))
	if buildErr == nil || runErr == nil {
		t.Fatalf("both must fail; BuildTargetEnv: %v, Run: %v", buildErr, runErr)
	}
	if buildErr.Error() != runErr.Error() {
		t.Errorf("BuildTargetEnv: %q\nmiso env: %q", buildErr.Error(), runErr.Error())
	}
}

func TestBuildTargetEnv_MemberWithoutConfigStillRuns(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".env"), "X=1\n")
	memberDir := filepath.Join(root, "apps", "web")
	write(t, filepath.Join(memberDir, "index.js"), "")
	cfg := config.Config{Env: []*config.EnvEntry{{Scope: "global", Path: ".env"}}}

	target := workspace.Target{Kind: workspace.TargetMember, Name: "web", Dir: memberDir}
	if got := envSliceToMap(mustEnv(t, root, cfg, target)); got["X"] != "1" {
		t.Errorf("X = %q, want 1", got["X"])
	}
}

func TestBuildTargetEnv_MemberDeclaredDiscoveredFileReportedOnce(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "package.json"), `{"workspaces":["apps/*"]}`)
	memberDir := filepath.Join(root, "apps", "api")
	write(t, filepath.Join(memberDir, "miso.json"), `{"env":[{"path":".env"}]}`)
	write(t, filepath.Join(memberDir, ".env"), "X=${X:?boom}\n")

	target := workspace.Target{Kind: workspace.TargetMember, Name: "api", Dir: memberDir}
	out := assertSameFailure(t, root, config.Config{}, target)
	if strings.Count(out, "unsupported expansion") != 1 {
		t.Errorf("want the failure once, got\n%s", out)
	}
}

func TestBuildTargetEnv_FailuresInMisoEnvOrder(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "package.json"), `{"workspaces":["apps/*"]}`)
	memberDir := filepath.Join(root, "apps", "api")
	write(t, filepath.Join(memberDir, "miso.json"), `{"env":[{"path":"member.env"}]}`)
	cfg := config.Config{Env: []*config.EnvEntry{
		{Scope: "global", Path: "global.env"},
	}}

	target := workspace.Target{Kind: workspace.TargetMember, Name: "api", Dir: memberDir}
	out := assertSameFailure(t, root, cfg, target)
	if strings.Index(out, "member.env") > strings.Index(out, "global.env") {
		t.Errorf("want the member entry first, as miso env prints it; got\n%s", out)
	}
}

func TestRun_AbsoluteEntryPath(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(t.TempDir(), "shared.env")
	write(t, abs, "X=1\n")
	cfg := config.Config{Env: []*config.EnvEntry{{Scope: "global", Path: abs, Variables: config.EnvVariables{Array: []string{"X"}}}}}

	if err := Run(root, cfg, log.New(io.Discard)); err != nil {
		t.Fatalf("Run() = %v, want nil for an absolute path", err)
	}
}
