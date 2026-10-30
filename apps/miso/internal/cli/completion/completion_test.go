package completion

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGetCandidates_IncludesEveryBuiltin(t *testing.T) {
	got := getCandidates("miso", "", t.TempDir())
	set := map[string]bool{}
	for _, c := range got {
		set[c] = true
	}
	for _, want := range []string{
		"install", "i", "add", "remove", "rm", "run", "dev", "scripts", "env",
		"init", "upgrade", "skills", "completion", "version", "v", "help",
	} {
		if !set[want] {
			t.Errorf("completion missing built-in %q", want)
		}
	}
	if set["misox"] {
		t.Error("completion must not offer misox")
	}
	if set["__complete"] {
		t.Error("completion must not offer __complete")
	}
}

func TestGetCandidates_MemberDirIncludesMemberScripts(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"package.json":                 `{"name":"fx","private":true,"workspaces":["apps/*"],"scripts":{"root-pkg":"x"}}`,
		"miso.json":                    `{}`,
		"apps/web/package.json":        `{"name":"web","private":true,"scripts":{"web-pkg":"x"}}`,
		"apps/web/miso.json":           `{}`,
		"apps/web/scripts/web-show.sh": "#!/bin/sh\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	member := filepath.Join(root, "apps", "web")
	for _, prev := range []string{"miso", "run"} {
		set := map[string]bool{}
		for _, c := range getCandidates(prev, "", member) {
			set[c] = true
		}
		for _, want := range []string{"root-pkg", "web-pkg", "web-show"} {
			if !set[want] {
				t.Errorf("after %q from apps/web: completion missing %q", prev, want)
			}
		}
	}
}

func TestGetCandidates_StandaloneRootOffersMemberScripts(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"package.json":                 `{"name":"fx","private":true,"workspaces":["apps/*"]}`,
		"miso.json":                    `{"packageManager":false}`,
		"apps/web/package.json":        `{"name":"web","private":true}`,
		"apps/web/scripts/web-show.sh": "#!/bin/sh\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Contains(getCandidates("miso", "", filepath.Join(root, "apps", "web")), "web-show") {
		t.Error("a standalone root still finds members, so completion must offer member scripts")
	}
}
