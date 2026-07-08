package env

import (
	"os"
	"path/filepath"
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

func mustEnv(t *testing.T, root string, cfg config.Config, target workspace.Target) []string {
	t.Helper()
	environ, err := BuildTargetEnv(root, cfg, target)
	if err != nil {
		t.Fatalf("BuildTargetEnv() error: %v", err)
	}
	return environ
}
