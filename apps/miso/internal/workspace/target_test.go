package workspace

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func members(root string) []Member {
	return []Member{
		{Name: "@org/web", Dir: filepath.Join(root, "apps", "web")},
		{Name: "api", Dir: filepath.Join(root, "apps", "api")},
	}
}

func TestResolveTarget_Member(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveTarget("@org/web", members(root), root, config.Config{})
	if err != nil {
		t.Fatalf("ResolveTarget() error: %v", err)
	}
	if got.Kind != TargetMember || got.Name != "@org/web" {
		t.Errorf("got %+v, want member @org/web", got)
	}
}

func TestResolveTarget_MemberByBasename(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveTarget("web", members(root), root, config.Config{})
	if err != nil {
		t.Fatalf("ResolveTarget() error: %v", err)
	}
	if got.Kind != TargetMember {
		t.Errorf("got %+v, want member", got)
	}
}

func TestResolveTarget_Task(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{Tasks: map[string]config.TaskConfig{"build": {}}}
	got, err := ResolveTarget("build", members(root), root, cfg)
	if err != nil {
		t.Fatalf("ResolveTarget() error: %v", err)
	}
	if got.Kind != TargetTask {
		t.Errorf("got %+v, want task", got)
	}
}

func TestResolveTarget_DefaultsToScript(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveTarget("dev", members(root), root, config.Config{})
	if err != nil {
		t.Fatalf("ResolveTarget() error: %v", err)
	}
	if got.Kind != TargetScript || got.Name != "dev" {
		t.Errorf("got %+v, want script dev", got)
	}
}

func TestResolveTarget_GlobalReserved(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveTarget("global", members(root), root, config.Config{}); err == nil {
		t.Error("expected error for reserved 'global'")
	}
}

func TestResolveTarget_AmbiguousMember(t *testing.T) {
	root := t.TempDir()
	dupes := []Member{
		{Name: "one", Dir: filepath.Join(root, "apps", "api")},
		{Name: "two", Dir: filepath.Join(root, "packages", "api")},
	}
	_, err := ResolveTarget("api", dupes, root, config.Config{})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got: %v", err)
	}
}

func TestFind_ByRelativePath(t *testing.T) {
	root := t.TempDir()
	got, err := Find("apps/web", members(root), root)
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}
	if got.Name != "@org/web" {
		t.Errorf("got %q, want @org/web", got.Name)
	}
}

func TestFind_Ambiguous(t *testing.T) {
	root := t.TempDir()
	dupes := []Member{
		{Name: "one", Dir: filepath.Join(root, "apps", "api")},
		{Name: "two", Dir: filepath.Join(root, "packages", "api")},
	}
	_, err := Find("api", dupes, root)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got: %v", err)
	}
}

func TestFind_NotFound(t *testing.T) {
	root := t.TempDir()
	_, err := Find("nope", members(root), root)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

func TestFromCWD_Found(t *testing.T) {
	root := t.TempDir()
	got, ok := FromCWD(filepath.Join(root, "apps", "web", "src"), members(root))
	if !ok {
		t.Fatal("expected FromCWD to find the containing member")
	}
	if got.Name != "@org/web" {
		t.Errorf("got %q, want @org/web", got.Name)
	}
}

func TestFromCWD_NotFound(t *testing.T) {
	root := t.TempDir()
	_, ok := FromCWD(filepath.Join(root, "other"), members(root))
	if ok {
		t.Error("expected FromCWD to report no containing member")
	}
}

// apps/web is named "web"; projects/ekkolore/web shares the basename "web" but
// is named "ekkolore-web". An exact name match must win over the basename clash.
func precedenceMembers(root string) []Member {
	return []Member{
		{Name: "web", Dir: filepath.Join(root, "apps", "web")},
		{Name: "ekkolore-web", Dir: filepath.Join(root, "projects", "ekkolore", "web")},
	}
}

func TestFind_ExactNameBeatsBasename(t *testing.T) {
	root := t.TempDir()
	ms := precedenceMembers(root)

	got, err := Find("web", ms, root)
	if err != nil {
		t.Fatalf("Find(web) error: %v", err)
	}
	if got.Name != "web" {
		t.Errorf("Find(web) = %+v, want apps/web by exact name", got)
	}

	got2, err := Find("ekkolore-web", ms, root)
	if err != nil {
		t.Fatalf("Find(ekkolore-web) error: %v", err)
	}
	if got2.Name != "ekkolore-web" {
		t.Errorf("Find(ekkolore-web) = %q, want ekkolore-web", got2.Name)
	}
}

func TestResolveTarget_ExactNameBeatsBasename(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveTarget("web", precedenceMembers(root), root, config.Config{})
	if err != nil {
		t.Fatalf("ResolveTarget(web) error: %v", err)
	}
	if got.Kind != TargetMember || got.Name != "web" {
		t.Errorf("got %+v, want member web (exact name wins over basename)", got)
	}
}

func TestFind_AmbiguousByName(t *testing.T) {
	root := t.TempDir()
	dupes := []Member{
		{Name: "dup", Dir: filepath.Join(root, "apps", "one")},
		{Name: "dup", Dir: filepath.Join(root, "apps", "two")},
	}
	if _, err := Find("dup", dupes, root); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error for duplicate names, got: %v", err)
	}
}
