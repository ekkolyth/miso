package scriptlog

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderContract(t *testing.T) {
	want := `ERROR Building the Debian VM image requires Lima 2.x, but limactl was not found. install="brew install lima" then="miso dev"`
	if got := ansi.Strip(Render(contractMessage)); got != want {
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

func TestRenderValueQuoting(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"v", `INFO t k=v`},
		{"a b", `INFO t k="a b"`},
		{`say "hi"`, `INFO t k="say \"hi\""`},
		{"a=b", `INFO t k=a=b`},
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
	rendered := Render(contractMessage)
	if rendered == ansi.Strip(rendered) {
		t.Error("Render() output has no styling")
	}
}
