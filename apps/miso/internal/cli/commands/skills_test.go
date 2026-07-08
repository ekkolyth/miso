package commands_test

import (
	"reflect"
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

func TestBuildSkillsArgs(t *testing.T) {
	add := commands.BuildSkillsArgsForTest("add", "miso", []string{"claude", "codex"})
	wantAdd := []string{"add", "https://github.com/ekkolyth/miso/tree/main/packages/skills", "-a", "claude", "-a", "codex", "-y"}
	if !reflect.DeepEqual(add, wantAdd) {
		t.Errorf("add args = %v, want %v", add, wantAdd)
	}
	rm := commands.BuildSkillsArgsForTest("remove", "miso", []string{"claude"})
	wantRm := []string{"remove", "miso", "-a", "claude", "-y"}
	if !reflect.DeepEqual(rm, wantRm) {
		t.Errorf("remove args = %v, want %v", rm, wantRm)
	}
}

func TestHasNpmOverridesConflict(t *testing.T) {
	cases := []struct {
		name    string
		manager string
		dir     string
		want    bool
	}{
		{"npm with overrides", "npm", "testdata/overrides", true},
		{"npm without overrides", "npm", "testdata/plain", false},
		{"bun with overrides", "bun", "testdata/overrides", false},
		{"npm missing package.json", "npm", "testdata/does-not-exist", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := commands.HasNpmOverridesConflictForTest(tc.manager, tc.dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
