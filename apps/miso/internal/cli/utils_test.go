package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/manager/drivers"
	"github.com/ekkolyth/miso/internal/testutil"
)

func init() { drivers.Register() }

func ensureManagerIn(t *testing.T, packageJSON string) (string, error) {
	t.Helper()
	root := t.TempDir()
	if packageJSON != "" {
		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	name, _, err := EnsureManager(root, config.Config{})
	return name, err
}

func TestEnsureManager_NoSourceMessage(t *testing.T) {
	_, err := ensureManagerIn(t, "")
	if err == nil || !strings.Contains(err.Error(), "could not determine package manager: add a packageManager field to package.json, or run 'miso init'") {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureManager_Field(t *testing.T) {
	name, err := ensureManagerIn(t, `{"packageManager":"pnpm@9"}`)
	testutil.NoError(t, err)
	testutil.Equal(t, name, "pnpm")
}

func TestEnsureManager_Unsupported(t *testing.T) {
	_, err := ensureManagerIn(t, `{"packageManager":"deno@2"}`)
	if err == nil || !strings.Contains(err.Error(), `unsupported package manager "deno"`) {
		t.Fatalf("err = %v", err)
	}
}

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
