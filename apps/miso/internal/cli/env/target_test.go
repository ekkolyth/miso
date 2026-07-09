package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
