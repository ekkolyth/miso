package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

// a delegated pipeline task only clashes with a folder script. A root
// package.json entry point ("build": "turbo run build") is the turbo
// convention and must still delegate.
func TestDelegatedFolderClash(t *testing.T) {
	testCases := []struct {
		name          string
		folderScript  string
		packageScript string
		expectClash   bool
	}{
		{name: "package.json entry point does not clash", packageScript: "build", expectClash: false},
		{name: "folder script clashes", folderScript: "build", expectClash: true},
		{name: "no script at all does not clash", expectClash: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
				t.Fatal(err)
			}

			pkg := `{"name":"root"}`
			if testCase.packageScript != "" {
				pkg = `{"name":"root","scripts":{"` + testCase.packageScript + `":"turbo run build"}}`
			}
			if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkg), 0o644); err != nil {
				t.Fatal(err)
			}

			if testCase.folderScript != "" {
				path := filepath.Join(root, "scripts", testCase.folderScript+".sh")
				if err := os.WriteFile(path, []byte("exit 0\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			cfg := config.Config{Scripts: "./scripts"}
			if got := delegatedFolderClash("build", root, cfg); got != testCase.expectClash {
				t.Errorf("delegatedFolderClash = %v, want %v", got, testCase.expectClash)
			}
		})
	}
}
