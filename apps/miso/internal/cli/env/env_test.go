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

// Suppress unused import warning — strings used in later tests added to this file.
var _ = strings.Contains
