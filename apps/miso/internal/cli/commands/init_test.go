package commands_test

import (
	"testing"

	"github.com/ekkolyth/miso/internal/cli/commands"
)

func TestRepoFieldForOrchestration_Turbo(t *testing.T) {
	if got := commands.RepoFieldForOrchestration("turbo"); got != "turbo" {
		t.Errorf("RepoFieldForOrchestration(turbo) = %q, want %q", got, "turbo")
	}
}

func TestRepoFieldForOrchestration_Nx(t *testing.T) {
	if got := commands.RepoFieldForOrchestration("nx"); got != "nx" {
		t.Errorf("RepoFieldForOrchestration(nx) = %q, want %q", got, "nx")
	}
}

func TestRepoFieldForOrchestration_Miso(t *testing.T) {
	if got := commands.RepoFieldForOrchestration("miso"); got != "" {
		t.Errorf("RepoFieldForOrchestration(miso) = %q, want empty", got)
	}
}

func TestRepoFieldForOrchestration_Unset(t *testing.T) {
	if got := commands.RepoFieldForOrchestration(""); got != "" {
		t.Errorf("RepoFieldForOrchestration(\"\") = %q, want empty", got)
	}
}
