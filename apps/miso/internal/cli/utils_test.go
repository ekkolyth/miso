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

// root package.json lists apps/* when listed; apps/web carries its own miso.json
func writeMemberConfigLayout(t *testing.T, listed, rootConfig bool) string {
	t.Helper()
	root := t.TempDir()
	rootPackage := `{"name":"fx","private":true}`
	if listed {
		rootPackage = `{"name":"fx","private":true,"workspaces":["apps/*"]}`
	}
	writeFile(t, filepath.Join(root, "package.json"), rootPackage)
	if rootConfig {
		writeFile(t, filepath.Join(root, config.FileName), "{}")
	}
	writeFile(t, filepath.Join(root, "apps", "web", "package.json"), `{"name":"web","private":true}`)
	writeFile(t, filepath.Join(root, "apps", "web", config.FileName), "{}")
	if err := os.MkdirAll(filepath.Join(root, "apps", "web", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSameDir(t *testing.T, got, want string) {
	t.Helper()
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(want)
	testutil.Equal(t, gotResolved, wantResolved)
}

func TestFindProjectRoot_MemberConfigResolvesToWorkspaceRoot(t *testing.T) {
	root := writeMemberConfigLayout(t, true, true)
	got, err := FindProjectRoot(filepath.Join(root, "apps", "web", "src"))
	testutil.NoError(t, err)
	assertSameDir(t, got, root)
}

func TestFindProjectRoot_NestedConfigOutsideWorkspaceStaysRoot(t *testing.T) {
	root := writeMemberConfigLayout(t, false, true)
	member := filepath.Join(root, "apps", "web")
	got, err := FindProjectRoot(member)
	testutil.NoError(t, err)
	assertSameDir(t, got, member)
}

func TestFindProjectRoot_MemberConfigWithoutRootConfigStaysRoot(t *testing.T) {
	root := writeMemberConfigLayout(t, true, false)
	member := filepath.Join(root, "apps", "web")
	got, err := FindProjectRoot(member)
	testutil.NoError(t, err)
	assertSameDir(t, got, member)
}

func TestFindProjectRoot_ProjectNestedInsideMemberStaysRoot(t *testing.T) {
	root := writeMemberConfigLayout(t, true, true)
	nested := filepath.Join(root, "apps", "web", "testdata", "fixture")
	writeFile(t, filepath.Join(nested, config.FileName), "{}")
	got, err := FindProjectRoot(nested)
	testutil.NoError(t, err)
	assertSameDir(t, got, nested)
}

// standalone mode turns workspace discovery off, so the root doesn't claim the member
func TestFindProjectRoot_StandaloneModeRootDoesNotClaimMember(t *testing.T) {
	root := writeMemberConfigLayout(t, true, true)
	writeFile(t, filepath.Join(root, config.FileName), `{"packageManager":false}`)
	member := filepath.Join(root, "apps", "web")
	got, err := FindProjectRoot(member)
	testutil.NoError(t, err)
	assertSameDir(t, got, member)
}
