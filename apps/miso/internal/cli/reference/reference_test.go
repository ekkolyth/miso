package reference

import (
	"strings"
	"testing"
)

func TestRender_HeaderFields(t *testing.T) {
	out, ok := Render("add")
	if !ok {
		t.Fatal("Render(add) returned ok=false")
	}
	for _, want := range []string{"miso add", "add dependencies", "Usage: miso add <pkg>"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing header field %q", want)
		}
	}
}

func TestRender_ResolvesAlias(t *testing.T) {
	out, ok := Render("v")
	if !ok || !strings.Contains(out, "miso version") {
		t.Errorf("alias v did not resolve to version (ok=%v)", ok)
	}
}

func TestRender_UnknownIsFalse(t *testing.T) {
	if _, ok := Render("nope"); ok {
		t.Error("Render(nope) should be ok=false")
	}
}

func TestBody_EmbeddedContentPresent(t *testing.T) {
	body, ok := Body("add")
	if !ok {
		t.Fatal("Body(add) not found in embedded content")
	}
	if !strings.Contains(body, "### When to use") {
		t.Error("add body missing its sections")
	}
}

func TestRunCommandHelp_UnknownErrors(t *testing.T) {
	if err := RunCommandHelp("definitely-not-a-command"); err == nil {
		t.Error("expected an error for an unknown command")
	}
}
