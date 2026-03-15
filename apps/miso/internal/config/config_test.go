package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return dir
}

func TestLoadTuiConfig(t *testing.T) {
	dir := writeTempConfig(t, `{
		"tui": "tabbed",
		"multi": {
			"web": ["dev", "test"],
			"api": ["start"]
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.TuiMode != "tabbed" {
		t.Errorf("Tui = %q, want %q", cfg.TuiMode, "tabbed")
	}

	if cfg.Multi == nil {
		t.Fatal("Multi is nil, want populated map")
	}

	web, ok := cfg.Multi["web"]
	if !ok {
		t.Fatal("Multi[\"web\"] missing")
	}
	if len(web) != 2 || web[0] != "dev" || web[1] != "test" {
		t.Errorf("Multi[\"web\"] = %v, want [dev test]", web)
	}

	api, ok := cfg.Multi["api"]
	if !ok {
		t.Fatal("Multi[\"api\"] missing")
	}
	if len(api) != 1 || api[0] != "start" {
		t.Errorf("Multi[\"api\"] = %v, want [start]", api)
	}
}

func TestLoadTuiConfigDefaults(t *testing.T) {
	dir := writeTempConfig(t, `{
		}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.TuiMode != "off" {
		t.Errorf("Tui = %q, want %q (default)", cfg.TuiMode, "off")
	}

	if cfg.Multi != nil {
		t.Errorf("Multi = %v, want nil", cfg.Multi)
	}
}

func TestLoadTuiConfigObject(t *testing.T) {
	dir := writeTempConfig(t, `{
		"tui": { "mode": "tabbed", "cleanExit": true }
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.TuiMode != "tabbed" {
		t.Errorf("TuiMode = %q, want %q", cfg.TuiMode, "tabbed")
	}
	if !cfg.TuiCleanExit {
		t.Error("TuiCleanExit = false, want true")
	}
}

func TestLoadTuiConfigObjectNoCleanExit(t *testing.T) {
	dir := writeTempConfig(t, `{
		"tui": { "mode": "merged" }
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.TuiMode != "merged" {
		t.Errorf("TuiMode = %q, want %q", cfg.TuiMode, "merged")
	}
	if cfg.TuiCleanExit {
		t.Error("TuiCleanExit = true, want false")
	}
}

func TestLoadRepoStringValues(t *testing.T) {
	tests := []struct {
		name string
		json string
		repo string
		mono bool
		dele bool
	}{
		{"single", `{"repo":"single"}`, "single", false, false},
		{"mono", `{"repo":"mono"}`, "mono", true, false},
		{"turbo", `{"repo":"turbo"}`, "turbo", true, true},
		{"nx", `{"repo":"nx"}`, "nx", true, true},
		{"empty", `{}`, "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTempConfig(t, tt.json)
			cfg, err := Load(dir)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.Repo != tt.repo {
				t.Errorf("Repo = %q, want %q", cfg.Repo, tt.repo)
			}
			if cfg.IsMonorepo() != tt.mono {
				t.Errorf("IsMonorepo() = %v, want %v", cfg.IsMonorepo(), tt.mono)
			}
			if cfg.IsDelegated() != tt.dele {
				t.Errorf("IsDelegated() = %v, want %v", cfg.IsDelegated(), tt.dele)
			}
		})
	}
}

func TestLoadRepoObject(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "mono",
			"tasks": {
				"build": { "dependsOn": ["^build"] },
				"dev": {}
			}
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Repo != "mono" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "mono")
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil, want populated map")
	}
	build, ok := cfg.Tasks["build"]
	if !ok {
		t.Fatal("Tasks[\"build\"] missing")
	}
	if len(build.DependsOn) != 1 || build.DependsOn[0] != "^build" {
		t.Errorf("Tasks[\"build\"].DependsOn = %v, want [^build]", build.DependsOn)
	}
	dev, ok := cfg.Tasks["dev"]
	if !ok {
		t.Fatal("Tasks[\"dev\"] missing")
	}
	if len(dev.DependsOn) != 0 {
		t.Errorf("Tasks[\"dev\"].DependsOn = %v, want []", dev.DependsOn)
	}
}

func TestLoadRepoObjectTurboWithTasks(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "turbo",
			"tasks": { "dev": {}, "dev:db": { "dependsOn": ["^dev"] } }
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Repo != "turbo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "turbo")
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil, want populated map")
	}
	if len(cfg.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(cfg.Tasks))
	}
	if _, ok := cfg.Tasks["dev"]; !ok {
		t.Error("Tasks[\"dev\"] missing")
	}
}

func TestLoadRepoObjectInvalidMode(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "single",
			"tasks": { "build": { "dependsOn": ["^build"] } }
		}
	}`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load() expected error for tasks with single mode, got nil")
	}
}

func TestRepoMode(t *testing.T) {
	tests := []struct {
		repo string
		want string
	}{
		{"single", "single"},
		{"mono", "mono"},
		{"turbo", "turbo"},
		{"nx", "nx"},
		{"", "single"},
	}
	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			cfg := Config{Repo: tt.repo}
			if got := cfg.RepoMode(); got != tt.want {
				t.Errorf("RepoMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTuiEnabled(t *testing.T) {
	tests := []struct {
		tui  string
		want bool
	}{
		{"off", false},
		{"", false},
		{"tabbed", true},
		{"merged", true},
	}

	for _, tt := range tests {
		t.Run(tt.tui, func(t *testing.T) {
			cfg := Config{TuiMode: tt.tui}
			got := cfg.TuiEnabled()
			if got != tt.want {
				t.Errorf("TuiEnabled() with TuiMode=%q = %v, want %v", tt.tui, got, tt.want)
			}
		})
	}
}
