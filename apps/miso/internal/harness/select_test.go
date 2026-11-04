package harness

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSelectFieldShowsEveryHarness(t *testing.T) {
	harnesses := []Harness{
		{Label: "Claude Code", Agent: "claude-code"},
		{Label: "Codex", Agent: "codex"},
	}
	var chosen []string
	field := selectField(harnesses, &chosen)
	field.Focus()
	field.Update(nil)
	view := ansi.Strip(field.View())
	for _, entry := range harnesses {
		if !strings.Contains(view, entry.Label) {
			t.Errorf("%q missing from view:\n%s", entry.Label, view)
		}
	}
}
