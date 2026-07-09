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
		dele bool
	}{
		{"miso", `{"repo":"miso"}`, "miso", false},
		{"turbo", `{"repo":"turbo"}`, "turbo", true},
		{"nx", `{"repo":"nx"}`, "nx", true},
		{"empty", `{}`, "", false},
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
			if cfg.IsDelegated() != tt.dele {
				t.Errorf("IsDelegated() = %v, want %v", cfg.IsDelegated(), tt.dele)
			}
		})
	}
}

func TestLoadRepoObject(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "turbo",
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
	if cfg.Repo != "turbo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "turbo")
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

func TestLoadRepoObjectMisoWithDependsOn(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "miso",
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
		// direct-construction passthrough; Load rejects these values
		{"single", "single"},
		{"mono", "mono"},
		{"turbo", "turbo"},
		{"nx", "nx"},
		{"", "miso"},
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

func TestLoadRepoObjectMisoWithTasks(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "miso",
			"tasks": {
				"dev": { "concurrent": ["frontend", "backend"] }
			}
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v, want success (tasks valid in all modes)", err)
	}
	if cfg.Repo != "miso" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "miso")
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
			"dev":   {Concurrent: []string{"services", "db:studio"}},
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

// mirrors what init writes for a single project, or a monorepo where the
// user picks miso orchestration: Repo left unset, so the saved config must
// load back as miso mode.
func TestSaveOmitsRepoWhenUnset(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Schema:  SchemaURL,
		Scripts: "./scripts",
		Flags:   make(map[string][]string),
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if strings.Contains(string(data), `"repo"`) {
		t.Error("saved config contains repo, want it omitted when unset")
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if got := loaded.RepoMode(); got != "miso" {
		t.Errorf("RepoMode() after round-trip = %q, want miso", got)
	}
}

// mirrors what init writes for a monorepo where the user picks turbo/nx
// orchestration: Repo set explicitly, so the saved config must load back
// as the delegated mode.
func TestSaveWritesRepoWhenDelegated(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Schema:  SchemaURL,
		Scripts: "./scripts",
		Flags:   make(map[string][]string),
		Repo:    "turbo",
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(data), `"repo": "turbo"`) {
		t.Errorf("saved config = %s, want it to contain repo: turbo", data)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if got := loaded.RepoMode(); got != "turbo" {
		t.Errorf("RepoMode() after round-trip = %q, want turbo", got)
	}
	if !loaded.IsDelegated() {
		t.Error("IsDelegated() after round-trip = false, want true")
	}
}

func TestRepoMode_DefaultsToMiso(t *testing.T) {
	cfg := Config{}
	if got := cfg.RepoMode(); got != "miso" {
		t.Errorf("RepoMode() = %q, want miso", got)
	}
}

func TestLoad_RejectsLegacyRepoValue(t *testing.T) {
	dir := writeTempConfig(t, `{"repo":"single","scripts":"./scripts"}`)
	if _, err := Load(dir); err == nil {
		t.Error("expected load error for legacy repo value \"single\"")
	}
}

func TestLoad_AcceptsMisoMode(t *testing.T) {
	dir := writeTempConfig(t, `{"repo":"turbo","scripts":"./scripts"}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.IsDelegated() || cfg.RepoMode() != "turbo" {
		t.Errorf("got mode %q delegated=%v, want turbo delegated", cfg.RepoMode(), cfg.IsDelegated())
	}
}

func TestLoad_ParsesEnvScope(t *testing.T) {
	dir := t.TempDir()
	body := `{"scripts":"./scripts","env":[{"scope":"web","path":"apps/web/.env.local"}]}`
	if err := os.WriteFile(filepath.Join(dir, "miso.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Env) != 1 || cfg.Env[0].Scope != "web" {
		t.Fatalf("Scope = %q, want web (entries: %+v)", scopeOf(cfg), cfg.Env)
	}
}

func scopeOf(cfg Config) string {
	if len(cfg.Env) == 0 {
		return ""
	}
	return cfg.Env[0].Scope
}

func TestLoadEnv_ArrayForm(t *testing.T) {
	dir := writeTempConfig(t, `{
		"env": [
			{"label": "app", "path": ".env", "variables": {"PORT": "port"}}
		]
	}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Env) != 1 {
		t.Fatalf("got %d env entries, want 1", len(cfg.Env))
	}
	e := cfg.Env[0]
	if e.Label != "app" || e.Path != ".env" {
		t.Errorf("entry = %+v, want label=app path=.env", e)
	}
	v, ok := e.Variables.Object["PORT"]
	if !ok || !v.IsShorthand || v.Type != "port" {
		t.Errorf("PORT var = %+v, want shorthand port", v)
	}
}

func TestLoadEnv_BareStringForm(t *testing.T) {
	dir := writeTempConfig(t, `{ "env": ".env.local" }`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Env) != 1 || cfg.Env[0].Path != ".env.local" {
		t.Fatalf("env = %+v, want single entry path=.env.local", cfg.Env)
	}
}

func TestLoadEnv_RequiredModeString(t *testing.T) {
	dir := writeTempConfig(t, `{
		"env": [{"path": ".env", "required": "none", "variables": {"X": "string"}}]
	}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Env[0].Required.Mode != "none" {
		t.Errorf("required.Mode = %q, want none", cfg.Env[0].Required.Mode)
	}
}

func TestLoadEnv_VariablesArrayForm(t *testing.T) {
	dir := writeTempConfig(t, `{
		"env": [{"path": ".env", "variables": ["A", "B"]}]
	}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	arr := cfg.Env[0].Variables.Array
	if len(arr) != 2 || arr[0] != "A" || arr[1] != "B" {
		t.Errorf("variables array = %v, want [A B]", arr)
	}
}
