package scripting

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/testutil"
)

func writePackageJSON(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeExecutableScript creates an executable script file under root/scripts.
func writeExecutableScript(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, "scripts", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveScript_FolderScript(t *testing.T) {
	root := t.TempDir()
	writeExecutableScript(t, root, "build.sh")

	got, err := ResolveScript("build", root, config.Config{Scripts: "./scripts"})
	testutil.NoError(t, err)
	testutil.Equal(t, got.Source, ScriptSourceFolder)
}

func TestResolveScript_NotFound(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveScript("missing", root, config.Config{Scripts: "./scripts"})
	testutil.NoError(t, err)
	testutil.Equal(t, got.Source, ScriptSourceNone)
}

func TestResolveScript_PackageJSONScript(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, root, `{"scripts":{"dev":"turbo run dev"}}`)

	got, err := ResolveScript("dev", root, config.Config{Scripts: "./scripts"})
	testutil.NoError(t, err)
	testutil.Equal(t, got.Source, ScriptSourcePackageJSON)
}

func TestResolveScript_FolderAndPackageJSONConflictErrors(t *testing.T) {
	root := t.TempDir()
	writeExecutableScript(t, root, "dev.sh")
	writePackageJSON(t, root, `{"scripts":{"dev":"turbo run dev"}}`)

	_, err := ResolveScript("dev", root, config.Config{Scripts: "./scripts"})
	if !errors.Is(err, ErrAmbiguousScript) {
		t.Fatalf("expected ErrAmbiguousScript for a name in both sources, got %v", err)
	}
}

func TestHasScript(t *testing.T) {
	root := t.TempDir()
	writeExecutableScript(t, root, "deploy.sh")

	has, err := HasScript("deploy", root, config.Config{Scripts: "./scripts"})
	testutil.NoError(t, err)
	testutil.Equal(t, has, true)

	has, err = HasScript("nope", root, config.Config{Scripts: "./scripts"})
	testutil.NoError(t, err)
	testutil.Equal(t, has, false)
}
