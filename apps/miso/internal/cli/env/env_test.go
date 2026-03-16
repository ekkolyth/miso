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
