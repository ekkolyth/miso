package config

import (
	"os"
	"path/filepath"
	"strings"
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
		"tui": "tabbed"
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.TuiMode != "tabbed" {
		t.Errorf("Tui = %q, want %q", cfg.TuiMode, "tabbed")
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

func TestLoadRepoObjectSingleWithDependsOn(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "single",
			"tasks": { "build": { "dependsOn": ["^build"] } }
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v, want success (tasks valid in all modes)", err)
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil")
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

func TestLoadRepoObjectWithConcurrent(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "turbo",
			"tasks": {
				"dev": { "concurrent": ["services", "db:studio"] }
			}
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil, want populated map")
	}
	dev, ok := cfg.Tasks["dev"]
	if !ok {
		t.Fatal("Tasks[\"dev\"] missing")
	}
	if len(dev.Concurrent) != 2 || dev.Concurrent[0] != "services" || dev.Concurrent[1] != "db:studio" {
		t.Errorf("Tasks[\"dev\"].Concurrent = %v, want [services db:studio]", dev.Concurrent)
	}
}

func TestLoadRepoObjectSingleWithTasks(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "single",
			"tasks": {
				"dev": { "concurrent": ["frontend", "backend"] }
			}
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v, want success (tasks valid in all modes)", err)
	}
	if cfg.Repo != "single" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "single")
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil, want populated map")
	}
}

func TestLoadRepoObjectNxWithTasks(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "nx",
			"tasks": { "dev": { "concurrent": ["api:serve"] } }
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v, want success (tasks valid in nx mode)", err)
	}
	if cfg.Repo != "nx" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "nx")
	}
}

func TestTaskConcurrent(t *testing.T) {
	cfg := Config{
		Tasks: map[string]TaskConfig{
			"dev": {Concurrent: []string{"services", "db:studio"}},
			"build": {},
		},
	}

	got := cfg.TaskConcurrent("dev")
	if len(got) != 2 || got[0] != "services" || got[1] != "db:studio" {
		t.Errorf("TaskConcurrent(\"dev\") = %v, want [services db:studio]", got)
	}

	if got := cfg.TaskConcurrent("build"); got != nil {
		t.Errorf("TaskConcurrent(\"build\") = %v, want nil", got)
	}

	if got := cfg.TaskConcurrent("unknown"); got != nil {
		t.Errorf("TaskConcurrent(\"unknown\") = %v, want nil", got)
	}

	nilCfg := Config{}
	if got := nilCfg.TaskConcurrent("dev"); got != nil {
		t.Errorf("TaskConcurrent on nil Tasks = %v, want nil", got)
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

func TestLoadPackageManagerFalse(t *testing.T) {
	dir := writeTempConfig(t, `{
		"packageManager": false
	}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PackageManager == nil {
		t.Fatal("PackageManager is nil, want *false")
	}
	if *cfg.PackageManager != false {
		t.Errorf("PackageManager = %v, want false", *cfg.PackageManager)
	}
}

func TestLoadPackageManagerTrue(t *testing.T) {
	dir := writeTempConfig(t, `{
		"packageManager": true
	}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PackageManager == nil {
		t.Fatal("PackageManager is nil, want *true")
	}
	if *cfg.PackageManager != true {
		t.Errorf("PackageManager = %v, want true", *cfg.PackageManager)
	}
}

func TestLoadPackageManagerAbsent(t *testing.T) {
	dir := writeTempConfig(t, `{}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PackageManager != nil {
		t.Errorf("PackageManager = %v, want nil (absent)", *cfg.PackageManager)
	}
}

func TestSimpleMode(t *testing.T) {
	f := false
	cfg := Config{PackageManager: &f}
	if !cfg.SimpleMode() {
		t.Error("SimpleMode() = false, want true")
	}

	tr := true
	cfg2 := Config{PackageManager: &tr}
	if cfg2.SimpleMode() {
		t.Error("SimpleMode() = true, want false (explicit true)")
	}

	cfg3 := Config{}
	if cfg3.SimpleMode() {
		t.Error("SimpleMode() = true, want false (nil/absent)")
	}
}

func TestSaveRoundTripsPackageManagerFalse(t *testing.T) {
	dir := t.TempDir()
	f := false
	cfg := Config{
		Schema:         SchemaURL,
		PackageManager: &f,
		Scripts:        "./scripts",
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if loaded.PackageManager == nil {
		t.Fatal("PackageManager is nil after round-trip, want *false")
	}
	if *loaded.PackageManager != false {
		t.Errorf("PackageManager = %v after round-trip, want false", *loaded.PackageManager)
	}
}

func TestSaveOmitsPackageManagerWhenNil(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Schema:  SchemaURL,
		Scripts: "./scripts",
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if strings.Contains(string(data), "packageManager") {
		t.Error("saved config contains packageManager, want it omitted when nil")
	}
}
