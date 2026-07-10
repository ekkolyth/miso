package env

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/testutil"
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
				Scope: "global",
				Variables: config.EnvVariables{
					Array: []string{"MISSING_ONE", "MISSING_TWO"},
				},
			},
			{
				Label: "beta",
				Path:  "b.env",
				Scope: "global",
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
				Scope: "global",
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
				Scope: "global",
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
				Scope: "global",
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

func TestRun_EmptyScopeIsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatalf("write temp env: %v", err)
	}
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: ".env"}, // no scope
		},
	}
	logger := log.New(io.Discard)
	if err := Run(dir, cfg, logger); err == nil {
		t.Error("expected error for entry with empty scope")
	}
}

func TestRun_DelegatedSkipsScopeRequirement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatalf("write temp env: %v", err)
	}
	cfg := config.Config{
		Repo: "turbo",
		Env: []*config.EnvEntry{
			{Path: ".env"}, // no scope — allowed in delegated mode
		},
	}
	logger := log.New(io.Discard)
	if err := Run(dir, cfg, logger); err != nil {
		t.Errorf("delegated mode should not require scope: %v", err)
	}
}

func TestRun_MemberLocalEnvEntry_MissingFileReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"workspaces":["apps/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	memberDir := filepath.Join(dir, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	memberConfig := `{"scripts":"./scripts","env":[{"label":"member","path":".env.missing"}]}`
	if err := os.WriteFile(filepath.Join(memberDir, "miso.json"), []byte(memberConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Label: "root", Path: ".env", Scope: "global"},
		},
	}
	logger := log.New(io.Discard)

	if err := Run(dir, cfg, logger); err == nil {
		t.Fatal("expected error for member env entry pointing at a missing file")
	}
}

func TestRun_ReservedGlobalMemberName_IsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"workspaces":["apps/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	memberDir := filepath.Join(dir, "apps", "global")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "package.json"), []byte(`{"name":"global"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Label: "root", Path: ".env", Scope: "global"},
		},
	}
	logger := log.New(io.Discard)

	err := Run(dir, cfg, logger)
	if err == nil {
		t.Fatal(`expected error for member named "global"`)
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("expected reserved-name error, got: %s", err.Error())
	}
}

func TestRun_UnknownScope_NotAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"workspaces":["apps/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	memberDir := filepath.Join(dir, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Label: "root", Path: ".env", Scope: "nonexistent-member"},
		},
	}
	var buf strings.Builder
	logger := log.NewWithOptions(&buf, log.Options{})

	// a scope may name a root script/task, so a non-member scope is neither an
	// error nor a warning — the entry just validates as a root entry.
	if err := Run(dir, cfg, logger); err != nil {
		t.Fatalf("unknown scope must not error, got: %v", err)
	}
	if strings.Contains(buf.String(), "scope matches no known member") {
		t.Errorf("unknown scope must not warn, got: %s", buf.String())
	}
}

func TestRun_DiscoverMembersError_ReturnsWrappedError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Label: "root", Path: ".env", Scope: "global"},
		},
	}
	logger := log.New(io.Discard)

	err := Run(dir, cfg, logger)
	if err == nil {
		t.Fatal("expected error for malformed package.json")
	}
	if !strings.Contains(err.Error(), "discover members") {
		t.Errorf("expected wrapped discover members error, got: %s", err.Error())
	}
}

func TestHasEnvFlag(t *testing.T) {
	testutil.Equal(t, HasEnvFlag([]string{"run", "--env", "build"}), true)
	testutil.Equal(t, HasEnvFlag([]string{"run", "build"}), false)
}

func TestStripEnvFlag(t *testing.T) {
	got := StripEnvFlag([]string{"run", "--env", "build"})
	if len(got) != 2 || got[0] != "run" || got[1] != "build" {
		t.Fatalf("StripEnvFlag = %v, want [run build]", got)
	}
}

func TestStripEnvFromFlags(t *testing.T) {
	cfg := config.Config{Flags: map[string][]string{
		"install": {"--frozen-lockfile", "--env"},
		"add":     {"--env"},
	}}
	out := StripEnvFromFlags(cfg)
	for k, v := range out.Flags {
		for _, f := range v {
			if f == EnvFlag {
				t.Errorf("flag group %q still contains %s", k, EnvFlag)
			}
		}
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
