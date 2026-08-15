package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// miso's own pane lines carry a badge so they read as miso's rather than the
// child's, and the badge is styled, not bare text
func TestMisoLine(t *testing.T) {
	styles := Default()
	got := styles.MisoLine("env validated — 3 scopes, 105 variables")

	if plain := ansi.Strip(got); plain != "[miso] env validated — 3 scopes, 105 variables" {
		t.Errorf("plain text = %q, want the badge followed by the message", plain)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("line carries no styling: %q", got)
	}
	if strings.HasPrefix(got, "[miso]") {
		t.Errorf("badge is unstyled: %q", got)
	}
}
