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

func TestFindWorkspace_ByRelativePath(t *testing.T) {
	root := t.TempDir()
	got, err := FindWorkspace("apps/web", members(root), root)
	if err != nil {
		t.Fatalf("FindWorkspace() error: %v", err)
	}
	if got.Name != "@org/web" {
		t.Errorf("got %q, want @org/web", got.Name)
	}
}

func TestFindWorkspace_Ambiguous(t *testing.T) {
	root := t.TempDir()
	dupes := []Member{
		{Name: "one", Dir: filepath.Join(root, "apps", "api")},
		{Name: "two", Dir: filepath.Join(root, "packages", "api")},
	}
	_, err := FindWorkspace("api", dupes, root)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got: %v", err)
	}
}

func TestFindWorkspace_NotFound(t *testing.T) {
	root := t.TempDir()
	_, err := FindWorkspace("nope", members(root), root)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}
