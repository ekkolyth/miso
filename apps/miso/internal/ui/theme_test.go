package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// the label block is the app's one label shape — bare text, filled, padded.
// Brackets belong to the plain sink's "[label] line" framing, not the label.
func TestLabel(t *testing.T) {
	got := Label(lipgloss.Color("#7c3aed"), "web")

	plain := ansi.Strip(got)
	if plain != " web " {
		t.Errorf("plain text = %q, want padded bare text", plain)
	}
	if strings.ContainsAny(plain, "[]") {
		t.Errorf("label brackets itself: %q", plain)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("label carries no styling: %q", got)
	}
}

// miso's own pane lines carry the same label block as a tab, in miso's tint
func TestMisoLine(t *testing.T) {
	styles := Default()
	got := styles.MisoLine("env validated — 3 scopes, 105 variables")

	if plain := ansi.Strip(got); plain != " miso  env validated — 3 scopes, 105 variables" {
		t.Errorf("plain text = %q, want the label block followed by the message", plain)
	}
	if !strings.HasPrefix(got, Label(MisoColor, "miso")) {
		t.Errorf("badge is not the shared label block: %q", got)
	}
}

// a workspace must never render in miso's tint
func TestMisoColorIsNotALabelColor(t *testing.T) {
	for _, c := range LabelColors {
		if c == MisoColor {
			t.Errorf("MisoColor %v is also a workspace label color", c)
		}
	}
}
