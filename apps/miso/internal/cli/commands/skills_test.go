package commands_test

import (
	"testing"

	"github.com/ekkolyth/miso/internal/cli/commands"
)

func TestParseSkillsFlags_AddOnly(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--add"})
	if !add {
		t.Error("expected add=true")
	}
	if rm {
		t.Error("expected rm=false")
	}
}

func TestParseSkillsFlags_RmOnly(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--rm"})
	if add {
		t.Error("expected add=false")
	}
	if !rm {
		t.Error("expected rm=true")
	}
}

func TestParseSkillsFlags_Both(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--add", "--rm"})
	if !add {
		t.Error("expected add=true")
	}
	if !rm {
		t.Error("expected rm=true")
	}
}

func TestParseSkillsFlags_Neither(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"add", "lodash"})
	if add {
		t.Error("expected add=false")
	}
	if rm {
		t.Error("expected rm=false")
	}
}

func TestParseSkillsFlags_Empty(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{})
	if add {
		t.Error("expected add=false")
	}
	if rm {
		t.Error("expected rm=false")
	}
}

func TestParseSkillsFlags_ExtraArgs(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--add", "--verbose"})
	if !add {
		t.Error("expected add=true")
	}
	if rm {
		t.Error("expected rm=false")
	}
}
