package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
)

// writeTempEnv creates a temp .env file and returns the project root dir.
func writeTempEnv(t *testing.T, filename, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp env: %v", err)
	}
	return dir
}

func TestRunEntry_ArrayMode_CollectsAllMissing(t *testing.T) {
	dir := writeTempEnv(t, ".env", "EXISTING=value\n")
	entry := &config.EnvEntry{
		Label: "test",
		Path:  ".env",
		Variables: config.EnvVariables{
			Array: []string{"MISSING_A", "MISSING_B", "EXISTING"},
		},
	}
	logger := log.New(os.Stderr)

	errs := runEntry(dir, entry, logger)
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
}

func TestRunEntry_ObjectMode_CollectsAllErrors(t *testing.T) {
	dir := writeTempEnv(t, ".env", "BAD_PORT=abc\n")
	entry := &config.EnvEntry{
		Label: "test",
		Path:  ".env",
		Variables: config.EnvVariables{
			Object: map[string]config.VarConfigOrString{
				"MISSING_VAR": {IsShorthand: true, Type: "string"},
				"BAD_PORT":    {IsShorthand: true, Type: "port"},
			},
		},
		Required: config.EnvRequired{Mode: "all"},
	}
	logger := log.New(os.Stderr)

	errs := runEntry(dir, entry, logger)
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
}

func TestRunEntry_FileNotFound_SingleError(t *testing.T) {
	dir := t.TempDir()
	entry := &config.EnvEntry{
		Label: "test",
		Path:  "nonexistent.env",
	}
	logger := log.New(os.Stderr)

	errs := runEntry(dir, entry, logger)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
}

func TestRunEntry_ArrayMode_VarErrorType(t *testing.T) {
	dir := writeTempEnv(t, ".env", "")
	entry := &config.EnvEntry{
		Label: "test",
		Path:  ".env",
		Variables: config.EnvVariables{
			Array: []string{"MISSING"},
		},
	}
	logger := log.New(os.Stderr)

	errs := runEntry(dir, entry, logger)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	ve, ok := errs[0].(*varError)
	if !ok {
		t.Fatalf("expected *varError, got %T", errs[0])
	}
	if ve.name != "MISSING" {
		t.Errorf("varError.name = %q, want %q", ve.name, "MISSING")
	}
}

func TestRun_GroupedErrors_MultipleEntries(t *testing.T) {
	dir := writeTempEnv(t, "a.env", "")
	if err := os.WriteFile(filepath.Join(dir, "b.env"), []byte("BAD_PORT=abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{
				Label: "alpha",
				Path:  "a.env",
				Variables: config.EnvVariables{
					Array: []string{"MISSING_ONE", "MISSING_TWO"},
				},
			},
			{
				Label: "beta",
				Path:  "b.env",
				Variables: config.EnvVariables{
					Object: map[string]config.VarConfigOrString{
						"BAD_PORT": {IsShorthand: true, Type: "port"},
					},
				},
				Required: config.EnvRequired{Mode: "none"},
			},
		},
	}
	logger := log.New(os.Stderr)

	err := Run(dir, cfg, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Run now returns a simple error; detailed output is printed to stderr
	if !strings.Contains(err.Error(), "env validation failed") {
		t.Errorf("expected 'env validation failed' error, got: %s", err.Error())
	}
}

func TestRun_SuccessfulEntries_StillPass(t *testing.T) {
	dir := writeTempEnv(t, "good.env", "PORT=8080\n")
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{
				Label: "good",
				Path:  "good.env",
				Variables: config.EnvVariables{
					Object: map[string]config.VarConfigOrString{
						"PORT": {IsShorthand: true, Type: "port"},
					},
				},
				Required: config.EnvRequired{Mode: "all"},
			},
		},
	}
	logger := log.New(os.Stderr)

	err := Run(dir, cfg, logger)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestRun_PassingEntryLogsInfo_WhenSiblingFails(t *testing.T) {
	dir := writeTempEnv(t, "good.env", "PORT=8080\n")
	if err := os.WriteFile(filepath.Join(dir, "bad.env"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{
				Label: "good",
				Path:  "good.env",
				Variables: config.EnvVariables{
					Object: map[string]config.VarConfigOrString{
						"PORT": {IsShorthand: true, Type: "port"},
					},
				},
				Required: config.EnvRequired{Mode: "all"},
			},
			{
				Label: "bad",
				Path:  "bad.env",
				Variables: config.EnvVariables{
					Array: []string{"MISSING_VAR"},
				},
			},
		},
	}

	// Capture log output to a buffer
	var buf strings.Builder
	logger := log.NewWithOptions(&buf, log.Options{})

	err := Run(dir, cfg, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The good entry should have logged its INFO line despite bad entry failing
	logOutput := buf.String()
	if !strings.Contains(logOutput, "good") {
		t.Errorf("expected INFO log for passing entry 'good', got: %s", logOutput)
	}
}

func TestBuildProcessEnv_SingleRepo_InjectsEnvFile(t *testing.T) {
	dir := writeTempEnv(t, ".env.local", "DB_URL=postgres://localhost\nAPI_KEY=secret123\n")
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: ".env.local"},
		},
	}
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	if environ == nil {
		t.Fatal("environ is nil, want non-nil")
	}
	envMap := envSliceToMap(environ)
	if envMap["DB_URL"] != "postgres://localhost" {
		t.Errorf("DB_URL = %q, want %q", envMap["DB_URL"], "postgres://localhost")
	}
	if envMap["API_KEY"] != "secret123" {
		t.Errorf("API_KEY = %q, want %q", envMap["API_KEY"], "secret123")
	}
}

func TestBuildProcessEnv_ProcessEnvWins(t *testing.T) {
	dir := writeTempEnv(t, ".env.local", "HOME=/from/file\n")
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: ".env.local"},
		},
	}
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["HOME"] == "/from/file" {
		t.Error("HOME was overwritten by .env file, want process env to win")
	}
}

func TestBuildProcessEnv_NoConfig_Discovery(t *testing.T) {
	dir := writeTempEnv(t, ".env.local", "DISCOVERED=yes\n")
	cfg := config.Config{}
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["DISCOVERED"] != "yes" {
		t.Errorf("DISCOVERED = %q, want %q", envMap["DISCOVERED"], "yes")
	}
}

func TestBuildProcessEnv_NoConfig_NoFile_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{}
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	if environ != nil {
		t.Errorf("environ = %v, want nil", environ)
	}
}

func TestBuildProcessEnv_MissingFile_SoftFail(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: "nonexistent.env"},
		},
	}
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() should soft-fail, got error: %v", err)
	}
	if environ != nil {
		t.Errorf("environ should be nil when all files missing, got %d entries", len(environ))
	}
}

func TestBuildProcessEnv_Monorepo_ScopedByPath(t *testing.T) {
	dir := t.TempDir()
	webDir := filepath.Join(dir, "apps", "web")
	apiDir := filepath.Join(dir, "apps", "api")
	os.MkdirAll(webDir, 0o755)
	os.MkdirAll(apiDir, 0o755)
	os.WriteFile(filepath.Join(webDir, ".env.local"), []byte("WEB_PORT=3000\n"), 0o644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("API_PORT=4000\n"), 0o644)
	cfg := config.Config{
		Repo: "mono",
		Env: []*config.EnvEntry{
			{Label: "web", Path: "apps/web/.env.local"},
			{Label: "api", Path: "apps/api/.env"},
		},
	}
	environ, err := BuildProcessEnv(dir, cfg, webDir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["WEB_PORT"] != "3000" {
		t.Errorf("WEB_PORT = %q, want %q", envMap["WEB_PORT"], "3000")
	}
	if envMap["API_PORT"] != "" {
		t.Errorf("API_PORT = %q, want empty", envMap["API_PORT"])
	}
}

func TestBuildProcessEnv_Monorepo_RootGetsAll(t *testing.T) {
	dir := t.TempDir()
	webDir := filepath.Join(dir, "apps", "web")
	apiDir := filepath.Join(dir, "apps", "api")
	os.MkdirAll(webDir, 0o755)
	os.MkdirAll(apiDir, 0o755)
	os.WriteFile(filepath.Join(webDir, ".env.local"), []byte("WEB_PORT=3000\n"), 0o644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("API_PORT=4000\n"), 0o644)
	cfg := config.Config{
		Repo: "mono",
		Env: []*config.EnvEntry{
			{Label: "web", Path: "apps/web/.env.local"},
			{Label: "api", Path: "apps/api/.env"},
		},
	}
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["WEB_PORT"] != "3000" {
		t.Errorf("WEB_PORT = %q, want %q", envMap["WEB_PORT"], "3000")
	}
	if envMap["API_PORT"] != "4000" {
		t.Errorf("API_PORT = %q, want %q", envMap["API_PORT"], "4000")
	}
}

func TestBuildProcessEnv_Monorepo_NoConfig_PerWorkspaceDiscovery(t *testing.T) {
	dir := t.TempDir()
	webDir := filepath.Join(dir, "apps", "web")
	os.MkdirAll(webDir, 0o755)
	os.WriteFile(filepath.Join(webDir, ".env.local"), []byte("DISCOVERED_WEB=yes\n"), 0o644)
	cfg := config.Config{Repo: "mono"}
	environ, err := BuildProcessEnv(dir, cfg, webDir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["DISCOVERED_WEB"] != "yes" {
		t.Errorf("DISCOVERED_WEB = %q, want %q", envMap["DISCOVERED_WEB"], "yes")
	}
}

func TestBuildProcessEnv_MultipleEntries_LaterWins(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.env"), []byte("SHARED=from_a\nONLY_A=yes\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.env"), []byte("SHARED=from_b\nONLY_B=yes\n"), 0o644)
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: "a.env"},
			{Path: "b.env"},
		},
	}
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["SHARED"] != "from_b" {
		t.Errorf("SHARED = %q, want %q (later entry should win)", envMap["SHARED"], "from_b")
	}
	if envMap["ONLY_A"] != "yes" {
		t.Errorf("ONLY_A = %q, want %q", envMap["ONLY_A"], "yes")
	}
	if envMap["ONLY_B"] != "yes" {
		t.Errorf("ONLY_B = %q, want %q", envMap["ONLY_B"], "yes")
	}
}

// envSliceToMap converts []string{"KEY=VALUE",...} to map[string]string
func envSliceToMap(environ []string) map[string]string {
	m := make(map[string]string)
	for _, e := range environ {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
