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
		"package-manager": "npm",
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

	if cfg.Tui != "tabbed" {
		t.Errorf("Tui = %q, want %q", cfg.Tui, "tabbed")
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
		"package-manager": "npm"
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Tui != "off" {
		t.Errorf("Tui = %q, want %q (default)", cfg.Tui, "off")
	}

	if cfg.Multi != nil {
		t.Errorf("Multi = %v, want nil", cfg.Multi)
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
			cfg := Config{Tui: tt.tui}
			got := cfg.TuiEnabled()
			if got != tt.want {
				t.Errorf("TuiEnabled() with Tui=%q = %v, want %v", tt.tui, got, tt.want)
			}
		})
	}
}
