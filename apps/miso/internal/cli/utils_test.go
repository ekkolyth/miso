package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/testutil"
)

func TestFindProjectRoot_FindsMisoJson(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindProjectRoot(sub)
	testutil.NoError(t, err)
	// macOS temp dirs are symlinks (/var -> /private/var); compare resolved paths.
	gotResolved, _ := filepath.EvalSymlinks(got)
	rootResolved, _ := filepath.EvalSymlinks(root)
	testutil.Equal(t, gotResolved, rootResolved)
}

func TestFindProjectRoot_NoProject(t *testing.T) {
	_, err := FindProjectRoot(t.TempDir())
	testutil.ErrorContains(t, err, "no project found")
}
