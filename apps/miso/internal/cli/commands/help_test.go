package commands

import (
	"strings"
	"testing"
)

func TestRenderHelp_ListsEveryBuiltin(t *testing.T) {
	out := RenderHelp()
	for _, name := range []string{
		"install", "add", "remove", "run", "dev", "scripts", "env",
		"init", "upgrade", "skills", "completion", "version", "help",
	} {
		if !strings.Contains(out, name) {
			t.Errorf("help output missing command %q", name)
		}
	}
}

func TestRenderHelp_ShowsAliasesUsageAndDocs(t *testing.T) {
	out := RenderHelp()
	if !strings.Contains(out, "remove, rm") {
		t.Error("help output should show the remove/rm alias pair")
	}
	if !strings.Contains(out, "version, v") {
		t.Error("help output should show the version/v alias pair")
	}
	if !strings.Contains(out, "add <pkg>") {
		t.Error("help output should show the add usage hint")
	}
	if !strings.Contains(out, "https://misojs.dev") {
		t.Error("help output missing docs link")
	}
}

func TestRenderHelp_OmitsMisox(t *testing.T) {
	if strings.Contains(RenderHelp(), "misox") {
		t.Error("help output must not mention misox")
	}
}

func TestRenderMisoLogo_NotEmpty(t *testing.T) {
	if strings.TrimSpace(RenderMisoLogo()) == "" {
		t.Error("RenderMisoLogo returned empty output")
	}
}
