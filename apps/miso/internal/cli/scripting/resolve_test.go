package scripting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/testutil"
)

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
