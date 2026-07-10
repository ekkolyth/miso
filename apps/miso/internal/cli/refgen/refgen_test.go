package refgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/cli"
)

func sourceBodies(t *testing.T) map[string]string {
	t.Helper()
	root, err := RepoRoot(".")
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	bodies, err := LoadSourceBodies(root)
	if err != nil {
		t.Fatalf("LoadSourceBodies: %v", err)
	}
	return bodies
}

func TestCorrespondence_EveryBuiltinHasContent(t *testing.T) {
	if err := CheckCorrespondence(sourceBodies(t)); err != nil {
		t.Error(err)
	}
}

func TestContentSanity_NonEmptyProseFirst(t *testing.T) {
	for name, body := range sourceBodies(t) {
		trimmed := strings.TrimSpace(body)
		if trimmed == "" {
			t.Errorf("%s: empty content", name)
			continue
		}
		first := strings.SplitN(trimmed, "\n", 2)[0]
		if strings.HasPrefix(first, "#") {
			t.Errorf("%s: must lead with a prose paragraph, not a heading", name)
		}
	}
}

func TestCommandsDoc_HasHeadingsAndMeta(t *testing.T) {
	out := CommandsDoc(sourceBodies(t))
	for _, want := range []string{"title: Commands", "## `miso add`", "**Usage:** `miso add <pkg>`", "## `miso version`"} {
		if !strings.Contains(out, want) {
			t.Errorf("CommandsDoc missing %q", want)
		}
	}
}

func TestReservedKeywordsDoc_ListsEveryBuiltin(t *testing.T) {
	out := ReservedKeywordsDoc()
	for _, c := range cli.Builtins {
		if !strings.Contains(out, "`"+c.Name+"`") {
			t.Errorf("ReservedKeywordsDoc missing %q", c.Name)
		}
	}
	if !strings.Contains(out, "| `v` |") && !strings.Contains(out, "`v`") {
		t.Error("ReservedKeywordsDoc missing version alias")
	}
}

func TestDrift_GeneratedMatchesCommitted(t *testing.T) {
	root, err := RepoRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	bodies, err := LoadSourceBodies(root)
	if err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		path string
		want string
	}{
		{filepath.Join(root, "apps", "docs", "content", "reference", "commands.mdx"), CommandsDoc(bodies)},
		{filepath.Join(root, "apps", "docs", "content", "reference", "reserved-keywords.mdx"), ReservedKeywordsDoc()},
	}
	for _, ch := range checks {
		got, err := os.ReadFile(ch.path)
		if err != nil {
			t.Errorf("read %s: %v", ch.path, err)
			continue
		}
		if string(got) != ch.want {
			t.Errorf("%s is stale — run `go generate ./...` in apps/miso and commit", ch.path)
		}
	}

	embedDir := filepath.Join(root, "apps", "miso", "internal", "cli", "reference", "content")
	for name, body := range bodies {
		got, err := os.ReadFile(filepath.Join(embedDir, name+".mdx"))
		if err != nil {
			t.Errorf("read embed copy %s: %v", name, err)
			continue
		}
		if string(got) != body {
			t.Errorf("embed copy %s.mdx is stale — run `go generate ./...` in apps/miso and commit", name)
		}
	}
}
