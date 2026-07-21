package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestDiscoverMembersCached_MatchesUncached(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps", "web"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverMembersCached(root, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembersCached: %v", err)
	}
	if len(got) != 1 || got[0].Name != "web" {
		t.Fatalf("members = %v, want [web]", got)
	}
}

// TestDiscoverMembersCached_MemoizesPerRoot proves the memo by construction:
// a second call for the same root must not re-glob, so it keeps returning the
// first result even after the workspace layout underneath it changes.
func TestDiscoverMembersCached_MemoizesPerRoot(t *testing.T) {
	root := t.TempDir()
	memberDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := DiscoverMembersCached(root, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembersCached (first): %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first call: len(members) = %d, want 1: %v", len(first), first)
	}

	// A fresh DiscoverMembers call would now see zero members — if the cached
	// call re-globbed, it would too.
	if err := os.RemoveAll(memberDir); err != nil {
		t.Fatal(err)
	}

	second, err := DiscoverMembersCached(root, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembersCached (second): %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("second call re-globbed instead of returning the cached result: %v", second)
	}
}

// TestDiscoverMembersCached_IndependentPerRoot verifies the memo is keyed by
// root, not a single shared slot — two different roots must not see each
// other's cached members.
func TestDiscoverMembersCached_IndependentPerRoot(t *testing.T) {
	rootA := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "package.json"),
		[]byte(`{"workspaces":["apps/a"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(rootA, "apps", "a"), 0o755); err != nil {
		t.Fatal(err)
	}

	rootB := t.TempDir()

	gotA, err := DiscoverMembersCached(rootA, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembersCached(rootA): %v", err)
	}
	if len(gotA) != 1 {
		t.Fatalf("rootA members = %v, want 1 entry", gotA)
	}

	gotB, err := DiscoverMembersCached(rootB, config.Config{})
	if err != nil {
		t.Fatalf("DiscoverMembersCached(rootB): %v", err)
	}
	if len(gotB) != 0 {
		t.Fatalf("rootB members = %v, want none (must not see rootA's cache entry)", gotB)
	}
}
