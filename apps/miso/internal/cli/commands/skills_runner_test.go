package commands_test

import (
	"testing"

	"github.com/ekkolyth/miso/internal/cli/commands"
	"github.com/ekkolyth/miso/internal/manager"
	"github.com/ekkolyth/miso/internal/manager/bun"
	"github.com/ekkolyth/miso/internal/manager/npm"
	"github.com/ekkolyth/miso/internal/manager/pnpm"
	"github.com/ekkolyth/miso/internal/manager/yarn"
)

func init() {
	manager.RegisterManager("bun", bun.Bun{})
	manager.RegisterManager("npm", npm.Npm{})
	manager.RegisterManager("pnpm", pnpm.Pnpm{})
	manager.RegisterManager("yarn", yarn.Yarn{})
}

func TestResolveSkillsRunner(t *testing.T) {
	none := func(string) bool { return false }
	onlyOnPath := func(want string) func(string) bool {
		return func(cmd string) bool { return cmd == want }
	}

	t.Run("npm without overrides uses npm", func(t *testing.T) {
		driver, err := commands.ResolveSkillsRunnerForTest("npm", "testdata/plain", none)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if driver.Name() != "npm" {
			t.Errorf("got %q, want npm", driver.Name())
		}
	})

	t.Run("npm with overrides falls back to bun", func(t *testing.T) {
		driver, err := commands.ResolveSkillsRunnerForTest("npm", "testdata/overrides", onlyOnPath("bunx"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if driver.Name() != "bun" {
			t.Errorf("got %q, want bun", driver.Name())
		}
	})

	t.Run("npm with overrides falls back to pnpm when bun absent", func(t *testing.T) {
		driver, err := commands.ResolveSkillsRunnerForTest("npm", "testdata/overrides", onlyOnPath("pnpm"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if driver.Name() != "pnpm" {
			t.Errorf("got %q, want pnpm", driver.Name())
		}
	})

	t.Run("npm with overrides and no alternative errors", func(t *testing.T) {
		if _, err := commands.ResolveSkillsRunnerForTest("npm", "testdata/overrides", none); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("bun ignores overrides", func(t *testing.T) {
		driver, err := commands.ResolveSkillsRunnerForTest("bun", "testdata/overrides", none)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if driver.Name() != "bun" {
			t.Errorf("got %q, want bun", driver.Name())
		}
	})
}
