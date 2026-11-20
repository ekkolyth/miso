package scriptlog

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/ekkolyth/miso/internal/ui"
)

func TestRenderExample(t *testing.T) {
	want := `ERROR Building the Debian VM image requires Lima 2.x, but limactl was not found. install="brew install lima" then="miso dev"`
	if got := ansi.Strip(Render(exampleMessage)); got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestRenderLevels(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG hi"},
		{LevelInfo, "INFO hi"},
		{LevelWarn, "WARN hi"},
		{LevelError, "ERROR hi"},
	}
	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			got := ansi.Strip(Render(Message{Level: tt.level, Text: "hi"}))
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderBadgeStyles(t *testing.T) {
	styles := ui.Default()
	tests := []struct {
		level Level
		style lipgloss.Style
	}{
		{LevelDebug, styles.Muted},
		{LevelInfo, styles.Accent},
		{LevelWarn, lipgloss.NewStyle().Bold(true).Foreground(ui.WarningColor)},
		{LevelError, styles.Error},
	}
	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			prefix := tt.style.Render(strings.ToUpper(string(tt.level)))
			if got := Render(Message{Level: tt.level, Text: "hi"}); !strings.HasPrefix(got, prefix) {
				t.Errorf("Render() = %q, want prefix %q", got, prefix)
			}
		})
	}
}

func TestRenderValueQuoting(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"v", `INFO t k=v`},
		{"a b", `INFO t k="a b"`},
		{`say "hi"`, `INFO t k="say \"hi\""`},
		{"a=b", `INFO t k=a=b`},
		{"a.go\nb.go", `INFO t k="a.go\nb.go"`},
		{"a\tb", `INFO t k="a\tb"`},
		{"a\x07b", `INFO t k="a\ab"`},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			msg := Message{Level: LevelInfo, Text: "t", Fields: []Field{{Key: "k", Value: tt.value}}}
			if got := ansi.Strip(Render(msg)); got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderStyled(t *testing.T) {
	rendered := Render(exampleMessage)
	if rendered == ansi.Strip(rendered) {
		t.Error("Render() output has no styling")
	}
}
