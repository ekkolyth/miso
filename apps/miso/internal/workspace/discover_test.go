package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverMembers_PackageJSONWorkspaces(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"workspaces":["apps/*"]}`)
	mkdirAll(t, filepath.Join(root, "apps", "web"))
	writeFile(t, filepath.Join(root, "apps", "web", "package.json"), `{"name":"@org/web"}`)
	mkdirAll(t, filepath.Join(root, "apps", "api"))

	members, err := DiscoverMembers(root, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembers() error: %v", err)
	}
	byName := map[string]Member{}
	for _, m := range members {
		byName[m.Name] = m
	}
	if _, ok := byName["@org/web"]; !ok {
		t.Errorf("missing @org/web; got %v", members)
	}
	if _, ok := byName["api"]; !ok { // basename fallback
		t.Errorf("missing api (basename fallback); got %v", members)
	}
}

func TestDiscoverMembers_PnpmPreferredWithExclusion(t *testing.T) {
	root := t.TempDir()
	// package.json also present — pnpm-workspace.yaml must win.
	writeFile(t, filepath.Join(root, "package.json"), `{"workspaces":["should-not-be-used/*"]}`)
	writeFile(t, filepath.Join(root, "pnpm-workspace.yaml"),
		"packages:\n  - \"packages/*\"\n  - \"!packages/internal\"\n")
	mkdirAll(t, filepath.Join(root, "packages", "ui"))
	mkdirAll(t, filepath.Join(root, "packages", "internal"))

	members, err := DiscoverMembers(root, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembers() error: %v", err)
	}
	names := map[string]bool{}
	for _, m := range members {
		names[m.Name] = true
	}
	if !names["ui"] {
		t.Errorf("expected ui member; got %v", members)
	}
	if names["internal"] {
		t.Errorf("internal should be excluded; got %v", members)
	}
}

func TestDiscoverMembers_ConfigPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"workspaces":["apps/*"]}`)
	mkdirAll(t, filepath.Join(root, "apps", "web"))
	writeFile(t, filepath.Join(root, "apps", "web", "miso.json"), `{"scripts":"./tasks"}`)

	members, err := DiscoverMembers(root, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembers() error: %v", err)
	}
	if len(members) != 1 || members[0].ConfigPath == "" {
		t.Fatalf("expected member with ConfigPath set; got %v", members)
	}
}
