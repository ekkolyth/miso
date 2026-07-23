package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPnpmWorkspace_PackagesAndExclusion(t *testing.T) {
	dir := t.TempDir()
	yaml := "packages:\n  - \"apps/*\"\n  - \"packages/*\"\n  - \"!packages/internal\"\ncatalog:\n  react: ^18\n"
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPnpmWorkspace(dir)
	if err != nil {
		t.Fatalf("ReadPnpmWorkspace() error: %v", err)
	}
	want := []string{"apps/*", "packages/*", "!packages/internal"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pattern %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadPnpmWorkspace_AbsentReturnsNil(t *testing.T) {
	got, err := ReadPnpmWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("ReadPnpmWorkspace() error: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
