package scripting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestListNamesIncludesTurboTasks(t *testing.T) {
	dir := t.TempDir()

	turboJSON := `{"tasks": {"build": {}, "lint": {}, "test": {}}}`
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(turboJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgJSON := `{"scripts": {"dev": "turbo dev", "build": "turbo run build"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Repo: "turbo"}

	names, err := ListNames(dir, cfg)
	if err != nil {
		t.Fatalf("ListNames() error: %v", err)
	}

	want := map[string]bool{"build": true, "dev": true, "lint": true, "test": true}
	got := make(map[string]bool)
	for _, n := range names {
		got[n] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing expected name %q in ListNames result: %v", name, names)
		}
	}
	if len(names) != len(got) {
		t.Errorf("ListNames has duplicates: %v", names)
	}
}
