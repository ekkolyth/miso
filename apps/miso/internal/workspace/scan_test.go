package workspace

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestSkipScanDir(t *testing.T) {
	for _, name := range []string{"node_modules", ".git", ".turbo"} {
		if !skipScanDir(name) {
			t.Errorf("skipScanDir(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"apps", "web"} {
		if skipScanDir(name) {
			t.Errorf("skipScanDir(%q) = true, want false", name)
		}
	}
}

func TestScanDirsDepthAndSkips(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		"apps/web", "api",
		"node_modules/x", ".cache/y", "apps/node_modules", "apps/.next",
		"apps/web/src",
	} {
		mkdirAll(t, filepath.Join(root, dir))
	}

	var got []string
	for _, dir := range scanDirs(root) {
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, filepath.ToSlash(rel))
	}
	want := []string{"api", "apps", "apps/web"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("scanDirs() = %v, want %v", got, want)
	}
}
